package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"v0.1.0-rc.4", "v0.1.0-rc.5", -1}, {"v1.0.0-rc.9", "v1.0.0-rc.10", -1}, {"v1.0.0", "v1.0.0-rc.99", 1}, {"v2.0.0", "v1.9.9", 1}, {"v1.0.0+abc", "1.0.0+other", 0}, {"v1.0.0-alpha.1", "v1.0.0-alpha.a", -1}} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			got, e := Compare(tc.a, tc.b)
			if e != nil || got != tc.want {
				t.Fatalf("Compare=%d,%v", got, e)
			}
		})
	}
	for _, v := range []string{"dev", "v01.0.0", "v1.0.0-rc.01", "v1.0.0-rc..1"} {
		if _, e := parseVersion(v); e == nil {
			t.Fatalf("accepted %s", v)
		}
	}
}

type fixture struct {
	source                 Source
	server                 *httptest.Server
	requests               atomic.Int32
	badChecksum, interrupt bool
	releases               []Release
	changelog              string
}

func newFixture(t *testing.T, tags ...string) *fixture {
	t.Helper()
	f := &fixture{}
	for _, tag := range tags {
		f.releases = append(f.releases, Release{Tag: tag})
		f.changelog += "## " + tag + " — 2026-10-02\n\n### Added\n\n- Added " + tag + ".\n\n### Breaking changes\n\n- Migration for " + tag + ".\n\n"
	}
	archive := testArchive(t)
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		switch {
		case r.URL.Path == "/api/releases":
			rels := append([]Release{}, f.releases...)
			for i := range rels {
				rels[i].Assets = []Asset{{Name: "dibs_linux_amd64.tar.gz", URL: f.server.URL + "/artifact"}, {Name: "checksums.txt", URL: f.server.URL + "/checksums"}}
			}
			_ = json.NewEncoder(w).Encode(rels)
		case strings.HasSuffix(r.URL.Path, "/CHANGELOG.md"):
			fmt.Fprint(w, f.changelog)
		case r.URL.Path == "/checksums":
			hash := sha256.Sum256(archive)
			if f.badChecksum {
				hash = sha256.Sum256([]byte("bad"))
			}
			fmt.Fprintf(w, "%s  dibs_linux_amd64.tar.gz\n", hex.EncodeToString(hash[:]))
		case r.URL.Path == "/artifact":
			if f.interrupt {
				w.Header().Set("Content-Length", fmt.Sprint(len(archive)+20))
				_, _ = w.Write(archive[:len(archive)/2])
				return
			}
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	f.source = Source{APIURL: f.server.URL + "/api", RawURL: f.server.URL + "/raw", Client: f.server.Client()}
	return f
}

func testArchive(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, n := range append(append([]string{}, binaries...), "LICENSE") {
		content := []byte("new-" + n)
		if e := tw.WriteHeader(&tar.Header{Name: n, Mode: 0755, Size: int64(len(content))}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write(content); e != nil {
			t.Fatal(e)
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := gz.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}

func testInstall(t *testing.T, s Source) Installer {
	t.Helper()
	dir := t.TempDir()
	for _, n := range binaries {
		if e := os.WriteFile(filepath.Join(dir, n), []byte("old-"+n), 0755); e != nil {
			t.Fatal(e)
		}
	}
	return Installer{Bindir: dir, Source: s, GOOS: "linux", GOARCH: "amd64"}
}
func assertSet(t *testing.T, i Installer, prefix string) {
	t.Helper()
	for _, n := range binaries {
		b, e := os.ReadFile(filepath.Join(i.Bindir, n))
		if e != nil || string(b) != prefix+n {
			t.Fatalf("%s=%q,%v", n, b, e)
		}
	}
}

func TestReleaseChecks(t *testing.T) {
	for _, tc := range []struct {
		name, current string
		tags          []string
		pre           bool
		latest        string
		available     bool
	}{{"newer", "v1.0.0", []string{"v1.1.0"}, false, "v1.1.0", true}, {"same", "v1.1.0", []string{"v1.1.0"}, false, "v1.1.0", false}, {"older", "v1.2.0", []string{"v1.1.0"}, false, "v1.1.0", false}, {"stable excludes pre", "v1.0.0", []string{"v1.2.0-rc.1", "v1.1.0"}, false, "v1.1.0", true}, {"explicit pre", "v1.0.0", []string{"v1.2.0-rc.1", "v1.1.0"}, true, "v1.2.0-rc.1", true}, {"automatic pre", "v1.0.0-rc.1", []string{"v1.2.0-rc.1", "v1.1.0"}, false, "v1.2.0-rc.1", true}, {"semantic order", "v1.0.0", []string{"v1.1.0", "v2.0.0", "v1.5.0"}, false, "v2.0.0", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.tags...)
			r, e := f.source.Check(context.Background(), tc.current, tc.pre)
			if e != nil || r.Latest != tc.latest || r.Available != tc.available {
				t.Fatalf("check %+v %v", r, e)
			}
			if tc.available && !r.Breaking {
				t.Fatal("breaking changes not detected")
			}
			if !tc.available && f.requests.Load() != 1 {
				t.Fatal("same/older fetched changelog")
			}
		})
	}
	f := newFixture(t, "v9.0.0", "v1.1.0")
	f.releases[0].Draft = true
	r, e := f.source.Check(context.Background(), "v1.0.0", false)
	if e != nil || r.Latest != "v1.1.0" {
		t.Fatalf("draft selection: %+v %v", r, e)
	}
}

func TestChangelogSlice(t *testing.T) {
	doc := "## Unreleased\n- Future\n\n## v1.2.0 — today\n### Added\n- New\n### Breaking changes\n- Break2\n\n## v1.1.0 — yesterday\n### Breaking changes\n- Break1\n### Fixed\n- Fix\n\n## v1.0.0\n### Breaking changes\n- Old\n"
	s, b := ChangelogSlice(doc, "v1.0.0", "v1.2.0")
	if !b || strings.Contains(s, "Old") || strings.Contains(s, "Future") || strings.Index(s, "Break1") > strings.Index(s, "Added") {
		t.Fatalf("wrong slice: %s", s)
	}
	if strings.Count(s, "Break") != 4 {
		t.Fatalf("missing sections: %s", s)
	}
	if s, b = ChangelogSlice("## \n", "v1.0.0", "v1.2.0"); s != "" || b {
		t.Fatal("empty heading")
	}
}

func TestUpgradeAndRollback(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	r, e := f.source.Check(context.Background(), "v1.0.0", false)
	if e != nil {
		t.Fatal(e)
	}
	i := testInstall(t, f.source)
	i.BeforeCommit = func() error { assertSet(t, i, "old-"); return nil }
	if e = i.Upgrade(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	assertSet(t, i, "new-")
	for _, n := range binaries {
		link, e := os.Readlink(filepath.Join(i.Bindir, n))
		if e != nil || link != filepath.Join(".dibs-update", "current", n) {
			t.Fatalf("public path %s", n)
		}
	}
	f.server.Close()
	if e = i.Rollback(); e != nil {
		t.Fatal(e)
	}
	assertSet(t, i, "old-")
	v, e := i.CurrentVersion()
	if e != nil || v != "v1.0.0" {
		t.Fatalf("rollback version %s,%v", v, e)
	}
}

func TestFailedUpgradeKeepsAllBinaries(t *testing.T) {
	for _, kind := range []string{"bad checksum", "interrupted download", "before commit"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, "v1.1.0")
			r, e := f.source.Check(context.Background(), "v1.0.0", false)
			if e != nil {
				t.Fatal(e)
			}
			i := testInstall(t, f.source)
			switch kind {
			case "bad checksum":
				f.badChecksum = true
			case "interrupted download":
				f.interrupt = true
			case "before commit":
				i.BeforeCommit = func() error { return errors.New("interrupted before atomic commit") }
			}
			if e = i.Upgrade(context.Background(), r); e == nil {
				t.Fatal("upgrade succeeded")
			}
			assertSet(t, i, "old-")
			if kind == "before commit" {
				if e = i.Rollback(); e != nil {
					t.Fatal(e)
				}
				assertSet(t, i, "old-")
			}
		})
	}
}

func TestUpgradeRefusesIndependentSymlinks(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	r, e := f.source.Check(context.Background(), "v1.0.0", false)
	if e != nil {
		t.Fatal(e)
	}
	i := testInstall(t, f.source)
	external := filepath.Join(t.TempDir(), "dibs")
	if e = os.WriteFile(external, []byte("external"), 0755); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(i.Bindir, "dibs")
	_ = os.Remove(p)
	if e = os.Symlink(external, p); e != nil {
		t.Fatal(e)
	}
	if e = i.Upgrade(context.Background(), r); e == nil {
		t.Fatal("independent symlink changed")
	}
	b, _ := os.ReadFile(external)
	if string(b) != "external" {
		t.Fatal("external file changed")
	}
}

func seedCache(t *testing.T, c Cache, r Result, now time.Time) {
	t.Helper()
	s := cacheState{Entries: map[string]cacheEntry{cacheKey(r.Installed, false): {LastAttempt: now, Result: &r}}, Shown: map[string]time.Time{}}
	if e := c.save(s); e != nil {
		t.Fatal(e)
	}
}

func TestNoticeCases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*NoticeOptions, *cacheState)
		want   bool
	}{{"human TTY", func(*NoticeOptions, *cacheState) {}, true}, {"non TTY", func(o *NoticeOptions, _ *cacheState) { o.TTY = false }, false}, {"json option", func(o *NoticeOptions, _ *cacheState) { o.JSON = true }, false}, {"json flag", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"--json", "issue", "list"} }, false}, {"opt out", func(o *NoticeOptions, _ *cacheState) { o.NoNotifier = true }, false}, {"child", func(o *NoticeOptions, _ *cacheState) { o.Child = true }, false}, {"hooks", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"hooks", "complete"} }, false}, {"MCP", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"mcp"} }, false}, {"protocol", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"protocol"} }, false}, {"watch stderr", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"watch"} }, false}, {"watch footer", func(o *NoticeOptions, _ *cacheState) { o.Args = []string{"watch"}; o.Footer = true }, true}, {"shown today", func(o *NoticeOptions, s *cacheState) { s.Shown["v1.1.0"] = o.Now.Add(-time.Hour) }, false}, {"shown yesterday", func(o *NoticeOptions, s *cacheState) { s.Shown["v1.1.0"] = o.Now.Add(-25 * time.Hour) }, true}, {"another version", func(o *NoticeOptions, s *cacheState) { s.Shown["v1.0.1"] = o.Now }, true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := Cache{filepath.Join(t.TempDir(), "cache.json")}
			now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			seedCache(t, c, Result{Installed: "v1.0.0", Latest: "v1.1.0", Available: true, Breaking: true}, now)
			o := NoticeOptions{Current: "v1.0.0", Args: []string{"issue", "list"}, TTY: true, Now: now}
			s := c.load()
			tc.mutate(&o, &s)
			if e := c.save(s); e != nil {
				t.Fatal(e)
			}
			line := c.Notice(o)
			if (line != "") != tc.want {
				t.Fatalf("notice %q want %t", line, tc.want)
			}
			if line != "" && !strings.Contains(line, "breaking changes") {
				t.Fatal("missing breaking qualifier")
			}
			if c.Notice(o) != "" {
				t.Fatal("duplicate notice")
			}
		})
	}
}

func TestConcurrentNoticeAndOfflineLatency(t *testing.T) {
	c := Cache{filepath.Join(t.TempDir(), "cache.json")}
	now := time.Now()
	seedCache(t, c, Result{Installed: "v1.0.0", Latest: "v1.1.0", Available: true}, now)
	var shown atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if c.Notice(NoticeOptions{Current: "v1.0.0", TTY: true, Now: now}) != "" {
				shown.Add(1)
			}
		})
	}
	wg.Wait()
	if shown.Load() != 1 {
		t.Fatalf("shown %d times", shown.Load())
	}
	start := time.Now()
	for range 100 {
		_ = c.Notice(NoticeOptions{Current: "v1.0.0", TTY: true, Now: now})
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("cached notice added offline latency")
	}
}

func TestCacheThrottlesFailedBackgroundRefresh(t *testing.T) {
	var requests atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Error(w, "offline", 503) }))
	defer s.Close()
	source := Source{APIURL: s.URL, Client: s.Client()}
	c := Cache{filepath.Join(t.TempDir(), "cache.json")}
	now := time.Now()
	for range 2 {
		if _, e := c.Refresh(context.Background(), source, "v1.0.0", false, false, now); e == nil {
			t.Fatal("offline succeeded")
		}
	}
	if requests.Load() != 1 {
		t.Fatal("background refresh not throttled")
	}
	if _, e := c.Refresh(context.Background(), source, "v1.0.0", false, false, now.Add(25*time.Hour)); e == nil {
		t.Fatal("offline succeeded")
	}
	if requests.Load() != 2 {
		t.Fatal("next day refresh missing")
	}
}

func TestCommandExitCodesAndRestartPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		args          []string
		want          int
		wantRestart   bool
		wantErr       bool
	}{{"current check", "v1.1.0", []string{"--check"}, 0, false, false}, {"older check", "v1.2.0", []string{"--check"}, 0, false, false}, {"new check", "v1.0.0", []string{"--check"}, 10, false, false}, {"breaking refused", "v1.0.0", nil, 1, false, true}, {"yes does not restart", "v1.0.0", []string{"--yes"}, 0, false, false}, {"restart explicit", "v1.0.0", []string{"--yes", "--restart"}, 0, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "v1.1.0")
			i := testInstall(t, f.source)
			var out, errout bytes.Buffer
			restarted := false
			c := Command{Version: tc.version, Source: f.source, Cache: Cache{filepath.Join(t.TempDir(), "cache.json")}, Installer: i, Out: &out, Err: &errout, In: strings.NewReader(""), Restart: func(_ context.Context, v string) error {
				restarted = true
				if v != "v1.1.0" {
					t.Fatal(v)
				}
				return nil
			}}
			code, e := c.Run(context.Background(), tc.args)
			if code != tc.want || (e != nil) != tc.wantErr || restarted != tc.wantRestart {
				t.Fatalf("code=%d err=%v restarted=%t", code, e, restarted)
			}
			if tc.wantErr || len(tc.args) > 0 && tc.args[0] == "--check" {
				assertSet(t, i, "old-")
			} else {
				assertSet(t, i, "new-")
			}
		})
	}
}

func TestRestartFailureRollbackAndInteractiveConfirmation(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	i := testInstall(t, f.source)
	var out, errout bytes.Buffer
	c := Command{Version: "v1.0.0", Source: f.source, Cache: Cache{filepath.Join(t.TempDir(), "cache.json")}, Installer: i, Out: &out, Err: &errout, In: strings.NewReader("yes\nyes\n"), Interactive: true, Restart: func(context.Context, string) error { return errors.New("restart failed") }}
	if code, e := c.Run(context.Background(), nil); code != 1 || e == nil {
		t.Fatal("restart failure missing")
	}
	assertSet(t, i, "new-")
	f.server.Close()
	c.Interactive = false
	if code, e := c.Run(context.Background(), []string{"--rollback"}); code != 0 || e != nil {
		t.Fatalf("rollback failed %d %v", code, e)
	}
	assertSet(t, i, "old-")
}

func TestCommandCheckFailureAndJSON(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	var out bytes.Buffer
	c := Command{Version: "v1.0.0", Source: f.source, Cache: Cache{filepath.Join(t.TempDir(), "cache.json")}, Out: &out, Err: &out, In: strings.NewReader(""), JSON: true}
	code, e := c.Run(context.Background(), []string{"--check"})
	var r Result
	if e != nil || code != 10 || json.Unmarshal(out.Bytes(), &r) != nil {
		t.Fatalf("json check %d %v %s", code, e, &out)
	}
	out.Reset()
	f.server.Close()
	if code, e = c.Run(context.Background(), []string{"--check"}); code != 1 || e == nil || out.Len() != 0 {
		t.Fatalf("failed check contaminated output %d %v %s", code, e, &out)
	}
}

func TestFailedSecondUpgradePreservesRollback(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	r, err := f.source.Check(context.Background(), "v1.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
	i := testInstall(t, f.source)
	if err = i.Upgrade(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	r.Installed, r.Latest = "v1.1.0", "v1.2.0"
	i.BeforeCommit = func() error { return errors.New("interrupted second upgrade") }
	if err = i.Upgrade(context.Background(), r); err == nil {
		t.Fatal("upgrade succeeded")
	}
	assertSet(t, i, "new-")
	if err = i.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertSet(t, i, "old-")
}

func TestStaleConcurrentUpgradeRefused(t *testing.T) {
	f := newFixture(t, "v1.1.0")
	r, err := f.source.Check(context.Background(), "v1.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
	i := testInstall(t, f.source)
	if err = i.Upgrade(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err = i.Upgrade(context.Background(), r); err == nil {
		t.Fatal("stale updater succeeded")
	}
	assertSet(t, i, "new-")
	if err = i.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertSet(t, i, "old-")
}

func TestAdoptionMetadataPrecedesPublication(t *testing.T) {
	i := testInstall(t, Source{})
	// A directory in the pointer destination forces the real rename to fail.
	// The staged original generation must already be complete at that phase.
	if err := os.MkdirAll(filepath.Join(i.root(), "current"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := i.adopt("v1.0.0"); err == nil {
		t.Fatal("pointer publication succeeded")
	}
	assertSet(t, i, "old-")
	if i.managed() {
		t.Fatal("public paths converted before pointer publication")
	}
	generations, err := filepath.Glob(filepath.Join(i.root(), "generation-*"))
	if err != nil || len(generations) != 1 {
		t.Fatalf("generations: %v %v", generations, err)
	}
	version, err := os.ReadFile(filepath.Join(generations[0], "version"))
	if err != nil || string(version) != "v1.0.0" {
		t.Fatalf("staged version metadata: %q %v", version, err)
	}
	// Clearing the failed destination lets a retry safely adopt the original files.
	if err = os.Remove(filepath.Join(i.root(), "current")); err != nil {
		t.Fatal(err)
	}
	if _, err = i.adopt("v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err = i.Rollback(); err != nil {
		t.Fatal(err)
	}
	if version, err := i.CurrentVersion(); err != nil || version != "v1.0.0" {
		t.Fatalf("original rollback version: %s %v", version, err)
	}
	assertSet(t, i, "old-")
}

func TestHomebrewCellarRefusedBeforeMutation(t *testing.T) {
	for _, throughLink := range []bool{false, true} {
		t.Run(fmt.Sprint(throughLink), func(t *testing.T) {
			root := t.TempDir()
			cellar := filepath.Join(root, "Cellar", "dibs", "1.0.0", "bin")
			if err := os.MkdirAll(cellar, 0755); err != nil {
				t.Fatal(err)
			}
			for _, n := range binaries {
				if err := os.WriteFile(filepath.Join(cellar, n), []byte("old-"+n), 0755); err != nil {
					t.Fatal(err)
				}
			}
			bindir := cellar
			if throughLink {
				bindir = filepath.Join(root, "bin-link")
				if err := os.Symlink(cellar, bindir); err != nil {
					t.Fatal(err)
				}
			}
			i := Installer{Bindir: bindir}
			r := Result{Installed: "v1.0.0", Latest: "v1.1.0", Available: true}
			if err := i.Upgrade(context.Background(), r); err == nil || !strings.Contains(err.Error(), "Homebrew") {
				t.Fatalf("upgrade: %v", err)
			}
			if err := i.Rollback(); err == nil || !strings.Contains(err.Error(), "Homebrew") {
				t.Fatalf("rollback: %v", err)
			}
			assertSet(t, i, "old-")
			if _, err := os.Stat(filepath.Join(cellar, ".dibs-update")); !os.IsNotExist(err) {
				t.Fatalf("created Homebrew updater state: %v", err)
			}
		})
	}
}

func TestHomebrewReceiptRefusedWithCustomCellar(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kegs", "dibs", "1.0.0", "bin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "INSTALL_RECEIPT.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	i := Installer{Bindir: dir}
	if err := i.Rollback(); err == nil || !strings.Contains(err.Error(), "Homebrew") {
		t.Fatalf("receipt guard: %v", err)
	}
	if _, err := os.Stat(i.root()); !os.IsNotExist(err) {
		t.Fatalf("created updater state: %v", err)
	}
}
