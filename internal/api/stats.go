package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/report"
	"github.com/abevz/dibs/internal/store"
)

func handleStats(st store.CoordinatorStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		top := 0
		if raw := r.URL.Query().Get("top"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, core.ErrValidationFailed, "top must be a positive integer")
				return
			}
			top = n
		}
		reportResult, err := report.Build(r.Context(), st, report.Query{
			By: r.URL.Query().Get("by"), Top: top,
			Project: r.URL.Query().Get("project"),
			Repo:    r.URL.Query().Get("repo"),
			Since:   r.URL.Query().Get("since"),
			Until:   r.URL.Query().Get("until"),
		}, time.Now().UTC())
		if err != nil {
			if apiErr, ok := errAsAPIError(err); ok {
				switch apiErr.Code {
				case core.ErrNotFound:
					writeError(w, http.StatusNotFound, apiErr.Code, apiErr.Message)
					return
				case core.ErrValidationFailed:
					writeError(w, http.StatusBadRequest, apiErr.Code, apiErr.Message)
					return
				}
			}
			logger.Error("build stats report", "error", err)
			writeInternalError(w, err, "failed to build statistics report")
			return
		}
		response := map[string]any{"report": reportResult}
		if safetyStore, ok := st.(interface {
			SafetySnapshot(context.Context, time.Time) (core.SafetySnapshot, error)
		}); ok {
			safety, err := safetyStore.SafetySnapshot(r.Context(), time.Now().UTC())
			if err != nil {
				logger.Error("build safety snapshot", "error", err)
				writeInternalError(w, err, "failed to build safety snapshot")
				return
			}
			response["safety"] = safety
		}
		writeJSON(w, http.StatusOK, response)
	}
}
