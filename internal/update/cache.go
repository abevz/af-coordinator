package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const refreshInterval = 24 * time.Hour

type Cache struct{ Path string }
type cacheEntry struct {
	LastAttempt time.Time `json:"last_attempt"`
	Result      *Result   `json:"result,omitempty"`
}
type cacheState struct {
	Entries map[string]cacheEntry `json:"entries"`
	Shown   map[string]time.Time  `json:"shown"`
}

func DefaultCache() Cache {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".local", "state")
	}
	return Cache{filepath.Join(root, "dibs", "update.json")}
}
func cacheKey(current string, pre bool) string { return current + "/" + channel(current, pre) }
func (c Cache) load() cacheState {
	s := cacheState{Entries: map[string]cacheEntry{}, Shown: map[string]time.Time{}}
	f, e := os.Open(c.Path)
	if e != nil {
		return s
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 2<<20))
	if e == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Entries == nil {
		s.Entries = map[string]cacheEntry{}
	}
	if s.Shown == nil {
		s.Shown = map[string]time.Time{}
	}
	return s
}

func withLock(path string, work func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("update state is busy: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return work()
}

func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func atomicWrite(path string, b []byte, mode os.FileMode) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".update-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	return syncDir(filepath.Dir(path))
}
func (c Cache) save(s cacheState) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	return atomicWrite(c.Path, b, 0600)
}

func (c Cache) Cached(current string, pre bool) *Result {
	return c.load().Entries[cacheKey(current, pre)].Result
}

// Refresh reserves the attempt before network I/O. A failed background check is
// silent and cannot turn a daemon restart into a second same-day request.
func (c Cache) Refresh(ctx context.Context, source Source, current string, pre, force bool, now time.Time) (Result, error) {
	if _, e := parseVersion(current); e != nil {
		return Result{}, e
	}
	var prior *Result
	due := force
	err := withLock(c.Path+".lock", func() error {
		s := c.load()
		entry := s.Entries[cacheKey(current, pre)]
		prior = entry.Result
		if !force && !entry.LastAttempt.IsZero() && now.Sub(entry.LastAttempt) < refreshInterval {
			return nil
		}
		due = true
		entry.LastAttempt = now
		s.Entries[cacheKey(current, pre)] = entry
		return c.save(s)
	})
	if !force && err != nil {
		return Result{}, err
	}
	if !due {
		if prior != nil {
			return *prior, nil
		}
		return Result{}, fmt.Errorf("release refresh already attempted today")
	}
	r, err := source.Check(ctx, current, pre)
	if err != nil {
		return r, err
	}
	_ = withLock(c.Path+".lock", func() error {
		s := c.load()
		entry := s.Entries[cacheKey(current, pre)]
		entry.Result = &r
		s.Entries[cacheKey(current, pre)] = entry
		return c.save(s)
	})
	return r, nil
}

func RefreshLoop(ctx context.Context, current string) {
	if _, err := parseVersion(current); err != nil || os.Getenv("DIBS_NO_UPDATE_NOTIFIER") == "1" {
		return
	}
	c := DefaultCache()
	source := DefaultSource()
	for {
		check, cancel := context.WithTimeout(ctx, 8*time.Second)
		_, _ = c.Refresh(check, source, current, false, false, time.Now())
		cancel()
		timer := time.NewTimer(refreshInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type NoticeOptions struct {
	Current           string
	Args              []string
	TTY, JSON, Footer bool
	NoNotifier, Child bool
	Now               time.Time
}

func (o NoticeOptions) allowed() bool {
	if !o.TTY || o.JSON || o.NoNotifier || o.Child {
		return false
	}
	args := o.Args
	command := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if args[i] == "--json" {
			return false
		}
		if args[i] == "--actor" {
			i++
			continue
		}
		if command == "" && !strings.HasPrefix(args[i], "-") {
			command = args[i]
		}
	}
	if command == "hooks" || command == "protocol" || command == "update" || command == "mcp" {
		return false
	}
	return command != "watch" || o.Footer
}

func NoticeLine(r Result) string {
	extra := ""
	if r.Breaking {
		extra = ", breaking changes"
	}
	return fmt.Sprintf("dibs %s is available (you have %s%s) — run: dibs update --check", r.Latest, r.Installed, extra)
}

// Notice performs local cache I/O only and uses a nonblocking lock. Concurrent
// commands cannot both claim the same version's daily notice.
func (c Cache) Notice(o NoticeOptions) string {
	if !o.allowed() {
		return ""
	}
	s := c.load()
	r := s.Entries[cacheKey(o.Current, false)].Result
	if r == nil || !r.Available {
		return ""
	}
	cmp, e := Compare(r.Latest, o.Current)
	if e != nil || cmp <= 0 {
		return ""
	}
	line := ""
	_ = withLock(c.Path+".lock", func() error {
		s = c.load()
		r = s.Entries[cacheKey(o.Current, false)].Result
		if r == nil || !r.Available {
			return nil
		}
		if cmp, err := Compare(r.Latest, o.Current); err != nil || cmp <= 0 {
			return nil
		}
		shown := s.Shown[r.Latest]
		if !shown.IsZero() && o.Now.Sub(shown) < refreshInterval {
			return nil
		}
		s.Shown[r.Latest] = o.Now
		if err := c.save(s); err != nil {
			return err
		}
		line = NoticeLine(*r)
		return nil
	})
	return line
}
