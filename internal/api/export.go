package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	coordinatorexport "github.com/abevz/af-coordinator/internal/export"
)

func handleExportJSONL(db *sql.DB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		if err := coordinatorexport.WriteJSONL(r.Context(), db, w); err != nil {
			logger.Error("failed to export jsonl", "error", err)
		}
	}
}
