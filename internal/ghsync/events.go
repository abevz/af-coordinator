package ghsync

import (
	"encoding/json"
	"fmt"

	"github.com/abevz/dibs/internal/core"
)

type CloseRecord struct {
	Event      core.Event
	Resolution string `json:"resolution"`
	Branch     string `json:"branch"`
	PRURL      string `json:"pr_url"`
	CommitSHA  string `json:"commit_sha"`
}

func LatestClose(events []core.Event) (CloseRecord, error) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType == "issue_operator_closed" {
			return CloseRecord{}, fmt.Errorf("latest close was operator-close; GitHub publication supports issue close and issue run")
		}
		if events[i].EventType != "issue_closed" {
			continue
		}
		var record CloseRecord
		if err := json.Unmarshal([]byte(events[i].PayloadJSON), &record); err != nil {
			return CloseRecord{}, fmt.Errorf("invalid close event: %w", err)
		}
		record.Event = events[i]
		if record.Resolution != "done" && record.Resolution != "cancelled" {
			return CloseRecord{}, fmt.Errorf("close event has no valid resolution")
		}
		return record, nil
	}
	return CloseRecord{}, fmt.Errorf("closed issue has no issue_closed event")
}

// A close note exists only when note_added immediately preceded the close in
// the same transaction. A second note by the same actor in the same second
// remains ambiguous until the close event stores the note ID (afc-90).
func HasCloseNoteEvent(events []core.Event, closed core.Event) bool {
	for _, event := range events {
		if event.Sequence == closed.Sequence-1 && event.EventType == "note_added" &&
			event.Actor == closed.Actor && event.CreatedAt == closed.CreatedAt {
			return true
		}
	}
	return false
}

func ClosingNote(events []core.Event, notes []core.Note, closed core.Event) string {
	if !HasCloseNoteEvent(events, closed) {
		return ""
	}
	var body string
	for _, note := range notes {
		if note.Author == closed.Actor && note.CreatedAt == closed.CreatedAt {
			body = note.Body
		}
	}
	return body
}
