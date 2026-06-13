package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// DM-P80: every dispatched key produces a transcript line.
func TestRecorder_WritePerKeystroke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	rec, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	m := credentialEntryModel(t).WithRecorder(rec)
	keys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("a")},
		{Type: tea.KeyTab},
		{Type: tea.KeyEsc},
	}
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(DashboardModel)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := readJSONL(t, path)
	if len(got) < len(keys) {
		t.Fatalf("expected >= %d entries, got %d", len(keys), len(got))
	}
}

// DM-P81 (paste regression): paste events must never write the raw rune
// buffer into the transcript. Reproduces the 2026-05-25 incident where a
// credential paste appeared verbatim in the Key field of session line 4.
func TestRecorder_PasteIsMarkedAndRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paste.jsonl")
	rec, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	m := credentialEntryModel(t).WithRecorder(rec)
	paste := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(validExaCredential), Paste: true}
	next, _ := m.Update(paste)
	_ = next
	_ = rec.Close()
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, validExaCredential) {
		t.Fatalf("recorder leaked paste contents into transcript:\n%s", content)
	}
	// Go's encoding/json HTML-escapes "<" as < by default. Either form is
	// acceptable in the transcript; the contract is that the literal credential
	// must not appear.
	if !strings.Contains(content, "paste") {
		t.Errorf("expected paste marker in transcript, got:\n%s", content)
	}
}

// DM-P81: recorder redacts the Message field. We do not record raw key
// bytes by default, so the regression test verifies the redactor path with
// an explicit Record call carrying the credential.
func TestRecorder_RedactionGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	rec, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	// redact.Text matches UUIDs at word boundaries, so we wrap the credential
	// in spaces to ensure it is recognized — the same shape it would take if
	// logged from a structured error.
	rec.Record(RecordEntry{Kind: "error", Message: "auth failed for key " + validExaCredential + " — retry"})
	_ = rec.Close()
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), validExaCredential) {
		t.Fatalf("recorder leaked credential into transcript:\n%s", data)
	}
}

// DM-P82: pressing q (which returns tea.Quit) closes the recorder.
func TestRecorder_CloseOnQuit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	rec, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	m := credentialEntryModel(t).WithRecorder(rec)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should emit tea.Quit")
	}
	// After q the recorder should be closed; further Records must be no-ops.
	rec.Record(RecordEntry{Kind: "key", Key: "after-close"})
	_ = rec.Close() // idempotent
	_ = next
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "after-close") {
		t.Fatalf("recorder accepted writes after quit:\n%s", data)
	}
	// A final entry must be present.
	if !strings.Contains(string(data), "\"kind\":\"final\"") {
		t.Errorf("final entry missing from transcript:\n%s", data)
	}
}

// `● rec` header indicator appears when a recorder is attached.
func TestRecorder_HeaderIndicatorPresent(t *testing.T) {
	rec, err := NewSessionRecorder(filepath.Join(t.TempDir(), "s.jsonl"))
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	defer func() { _ = rec.Close() }()
	m := credentialEntryModel(t).WithRecorder(rec)
	view := m.View()
	if !strings.Contains(view, "● rec") {
		t.Errorf("expected '● rec' indicator in view, got:\n%s", view)
	}
}

func TestRecorder_PrivateFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	rec, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	_ = rec.Close()
	info, _ := os.Stat(path)
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("transcript mode = %o, want 0600", mode)
	}
}

func readJSONL(t *testing.T, path string) []RecordEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var out []RecordEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry RecordEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("parse line %q: %v", line, err)
		}
		out = append(out, entry)
	}
	return out
}
