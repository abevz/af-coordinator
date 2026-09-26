package main

import (
	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/ghsync"
)

// Keep thin wrappers for the original CLI-focused test entrypoints while the
// shared behavior lives in ghsync for both CLI and MCP.
type closeRecord = ghsync.CloseRecord

func latestClose(events []core.Event) (closeRecord, error) { return ghsync.LatestClose(events) }
func closingNote(events []core.Event, notes []core.Note, closed core.Event) string {
	return ghsync.ClosingNote(events, notes, closed)
}
func hasCloseNoteEvent(events []core.Event, closed core.Event) bool {
	return ghsync.HasCloseNoteEvent(events, closed)
}
