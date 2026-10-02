package update

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

type version struct {
	numbers [3]uint64
	pre     []string
}

func parseVersion(s string) (version, error) {
	var v version
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return v, fmt.Errorf("%q is not a release version (source builds use dev)", s)
	}
	for i := range 3 {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return v, err
		}
		v.numbers[i] = n
	}
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
		for _, p := range v.pre {
			if p == "" {
				return v, fmt.Errorf("invalid prerelease %q", s)
			}
			if allDigits(p) && len(p) > 1 && p[0] == '0' {
				return v, fmt.Errorf("invalid prerelease %q", s)
			}
		}
	}
	return v, nil
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// Compare compares release versions, including numeric prerelease identifiers.
func Compare(a, b string) (int, error) {
	x, e := parseVersion(a)
	if e != nil {
		return 0, e
	}
	y, e := parseVersion(b)
	if e != nil {
		return 0, e
	}
	for i := range 3 {
		if x.numbers[i] < y.numbers[i] {
			return -1, nil
		}
		if x.numbers[i] > y.numbers[i] {
			return 1, nil
		}
	}
	if len(x.pre) == 0 && len(y.pre) > 0 {
		return 1, nil
	}
	if len(y.pre) == 0 && len(x.pre) > 0 {
		return -1, nil
	}
	for i := 0; i < len(x.pre) && i < len(y.pre); i++ {
		a, b := x.pre[i], y.pre[i]
		if a == b {
			continue
		}
		an, bn := allDigits(a), allDigits(b)
		if an && bn {
			if len(a) < len(b) || len(a) == len(b) && a < b {
				return -1, nil
			}
			return 1, nil
		}
		if an {
			return -1, nil
		}
		if bn {
			return 1, nil
		}
		if a < b {
			return -1, nil
		}
		return 1, nil
	}
	if len(x.pre) < len(y.pre) {
		return -1, nil
	}
	if len(x.pre) > len(y.pre) {
		return 1, nil
	}
	return 0, nil
}

func channel(installed string, includePre bool) string {
	v, _ := parseVersion(installed)
	if includePre || len(v.pre) > 0 {
		return "prerelease"
	}
	return "stable"
}
