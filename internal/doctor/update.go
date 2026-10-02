package doctor

import "github.com/abevz/dibs/internal/update"

func EvaluateUpdate(cache update.Cache, version string) Result {
	r := cache.Cached(version, false)
	if r != nil && r.Available {
		if cmp, err := update.Compare(r.Latest, version); err == nil && cmp > 0 {
			return Result{Name: "Release update", Status: "WARN", Message: "update available: " + r.Latest, Hint: "Run dibs update --check; migrate consumers before a breaking release"}
		}
	}
	return Result{Name: "Release update", Status: "ok", Message: "No cached newer release (offline-safe; use dibs update --check to refresh)"}
}
