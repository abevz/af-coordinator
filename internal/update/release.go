package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type Result struct {
	Installed string  `json:"installed"`
	Latest    string  `json:"latest"`
	Available bool    `json:"available"`
	Breaking  bool    `json:"breaking_changes"`
	Changelog string  `json:"changelog,omitempty"`
	Release   Release `json:"release"`
}

type Source struct {
	APIURL, RawURL string
	Client         *http.Client
}

func DefaultSource() Source {
	return Source{APIURL: "https://api.github.com/repos/abevz/dibs", RawURL: "https://raw.githubusercontent.com/abevz/dibs", Client: &http.Client{Timeout: 2 * time.Minute}}
}

func (s Source) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dibs-update")
	req.Header.Set("Accept", "application/vnd.github+json")
	c := s.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release request returned HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("release response exceeds size limit")
	}
	return b, nil
}

func (s Source) Check(ctx context.Context, installed string, includePre bool) (Result, error) {
	r := Result{Installed: installed}
	if _, err := parseVersion(installed); err != nil {
		return r, err
	}
	var releases []Release
	for page := 1; page <= 20; page++ {
		b, err := s.get(ctx, fmt.Sprintf("%s/releases?per_page=100&page=%d", strings.TrimRight(s.APIURL, "/"), page), 8<<20)
		if err != nil {
			return r, err
		}
		var batch []Release
		if err = json.Unmarshal(b, &batch); err != nil {
			return r, err
		}
		releases = append(releases, batch...)
		if len(batch) < 100 {
			break
		}
		if page == 20 {
			return r, fmt.Errorf("release listing exceeds pagination limit")
		}
	}
	pre := channel(installed, includePre) == "prerelease"
	for _, rel := range releases {
		v, err := parseVersion(rel.Tag)
		if err != nil || rel.Draft || (!pre && (rel.Prerelease || len(v.pre) > 0)) {
			continue
		}
		if r.Latest == "" {
			r.Release = rel
			r.Latest = rel.Tag
			continue
		}
		cmp, _ := Compare(rel.Tag, r.Latest)
		if cmp > 0 {
			r.Release = rel
			r.Latest = rel.Tag
		}
	}
	if r.Latest == "" {
		return r, fmt.Errorf("no release found for the %s channel", channel(installed, includePre))
	}
	cmp, _ := Compare(r.Latest, installed)
	r.Available = cmp > 0
	if r.Available {
		b, err := s.get(ctx, strings.TrimRight(s.RawURL, "/")+"/"+url.PathEscape(r.Latest)+"/CHANGELOG.md", 2<<20)
		if err != nil {
			return r, fmt.Errorf("read release CHANGELOG: %w", err)
		}
		r.Changelog, r.Breaking = ChangelogSlice(string(b), installed, r.Latest)
		if r.Changelog == "" {
			return r, fmt.Errorf("CHANGELOG has no section between %s and %s", installed, r.Latest)
		}
	}
	return r, nil
}

// ChangelogSlice includes only released versions in (installed, latest], with
// all Breaking changes sections first, preserving their release headings.
func ChangelogSlice(doc, installed, latest string) (string, bool) {
	var breaking, normal []string
	parts := strings.Split("\n"+doc, "\n## ")
	for _, part := range parts[1:] {
		lines := strings.Split(part, "\n")
		heading := strings.TrimSpace(lines[0])
		fields := strings.Fields(heading)
		if len(fields) == 0 {
			continue
		}
		tag := strings.Trim(fields[0], "[]")
		if _, err := parseVersion(tag); err != nil {
			continue
		}
		lo, _ := Compare(tag, installed)
		hi, _ := Compare(tag, latest)
		if lo <= 0 || hi > 0 {
			continue
		}
		head := "## " + heading + "\n"
		var other []string
		for i := 1; i < len(lines); {
			if strings.HasPrefix(lines[i], "### ") && strings.HasPrefix(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(lines[i], "### "))), "breaking changes") {
				j := i + 1
				for j < len(lines) && !strings.HasPrefix(lines[j], "### ") {
					j++
				}
				breaking = append(breaking, head+strings.Join(lines[i:j], "\n"))
				i = j
			} else {
				other = append(other, lines[i])
				i++
			}
		}
		if strings.TrimSpace(strings.Join(other, "\n")) != "" {
			normal = append(normal, head+strings.Join(other, "\n"))
		}
	}
	return strings.TrimSpace(strings.Join(append(breaking, normal...), "\n\n")), len(breaking) > 0
}

func (r Release) asset(name string) (string, error) {
	var found string
	for _, a := range r.Assets {
		if a.Name == name {
			if found != "" {
				return "", fmt.Errorf("duplicate release asset %s", name)
			}
			found = a.URL
		}
	}
	if found == "" {
		return "", fmt.Errorf("release asset %s missing", name)
	}
	return found, nil
}
