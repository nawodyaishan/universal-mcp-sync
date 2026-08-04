package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

// RecordEntry is one event in a session transcript. The schema is intentionally
// flat so JSONL output is greppable.
type RecordEntry struct {
	Time        time.Time `json:"time"`
	Kind        string    `json:"kind"`          // "key" | "screen" | "error" | "final" | "start"
	Key         string    `json:"key,omitempty"` // for kind=key
	Screen      string    `json:"screen,omitempty"`
	PC          string    `json:"pc,omitempty"`
	BlockReason string    `json:"block_reason,omitempty"`
	Message     string    `json:"message,omitempty"` // always passed through redact.Text
	Digest      string    `json:"view_digest,omitempty"`
}

// SessionRecorder is a JSONL transcript writer for a dashboard session. All
// strings passed to Record go through redact.Text before serialization so
// credential values never appear in the transcript even if the caller forgets
// to redact upstream.
type SessionRecorder struct {
	mu       sync.Mutex
	path     string
	enc      *json.Encoder
	file     *os.File
	redactor func(string) string
	closed   bool
}

// DefaultRecorderPath returns the canonical path for a recording when the
// user passed --record with no argument: artifacts/journeys/usync-<ts>.jsonl.
func DefaultRecorderPath() string {
	return filepath.Join("artifacts", "journeys", fmt.Sprintf("usync-%s.jsonl", time.Now().UTC().Format("20060102T150405Z")))
}

// NewSessionRecorder opens path with private permissions and returns a
// recorder ready to accept entries. The parent directory is created with
// 0o700; the file with 0o600.
func NewSessionRecorder(path string) (*SessionRecorder, error) {
	if path == "" {
		path = DefaultRecorderPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create recorder dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open recorder file: %w", err)
	}
	return &SessionRecorder{
		path:     path,
		enc:      json.NewEncoder(f),
		file:     f,
		redactor: redact.Text,
	}, nil
}

// Path returns the absolute path of the transcript file.
func (r *SessionRecorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Record appends an entry. Redaction is always applied to Message, Key,
// BlockReason — every string field that could carry user-pasted or
// model-surfaced credential values. The recorder is the last line of
// defense; assume callers have not redacted.
func (r *SessionRecorder) Record(e RecordEntry) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if r.redactor != nil {
		if e.Message != "" {
			e.Message = r.redactor(e.Message)
		}
		if e.Key != "" {
			e.Key = sanitizeKeyLabel(r.redactor(e.Key))
		}
		if e.BlockReason != "" {
			e.BlockReason = r.redactor(e.BlockReason)
		}
	}
	_ = r.enc.Encode(e)
}

// sanitizeKeyLabel collapses any key label that — after redaction — still
// looks like pasted free-form content (longer than a typical key name like
// "shift+tab") into a fixed "<paste>" marker. This guarantees pastes never
// reach the transcript as raw runes even if the redactor pattern misses.
func sanitizeKeyLabel(label string) string {
	if len(label) > 16 {
		return "<paste>"
	}
	return label
}

// Close flushes the file and marks the recorder shut so subsequent Record
// calls are no-ops.
func (r *SessionRecorder) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// recordKey emits a key entry from a tea.KeyMsg, after redaction. Pasted
// content is replaced with "<paste>" before Record() runs so the raw rune
// buffer never enters the redaction pipeline.
func recordKey(r *SessionRecorder, msg tea.KeyMsg, snap DashboardSnapshot, digest string) {
	if r == nil {
		return
	}
	label := msg.String()
	if msg.Paste {
		label = "<paste>"
	}
	r.Record(RecordEntry{
		Kind:        "key",
		Key:         label,
		Screen:      snap.Screen,
		PC:          dashboardPC(snap),
		BlockReason: snap.BlockReason,
		Digest:      digest,
	})
}

// isTeaQuit reports whether a tea.Cmd is tea.Quit by comparing function
// pointer identity. Required because invoking arbitrary commands has side
// effects, but Quit detection happens before the runtime processes the cmd.
func isTeaQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	return reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()
}

// dashboardPC mirrors the explorer's PreconditionClass classifier for use in
// recorder transcripts. It is intentionally a copy rather than a cross-package
// dependency so pkg/tui stays free of pkg/uxexplore.
func dashboardPC(s DashboardSnapshot) string {
	switch {
	case s.HasScanError:
		return "scan-error"
	case s.HasApplyError:
		return "apply-error"
	case s.HasPlanError:
		return "plan-error"
	case s.HasValidationError:
		return "network-failure"
	case s.RuntimeMissing:
		return "runtime-missing"
	case s.ConflictUnresolved:
		return "conflict-unresolved"
	case s.MissingCredentials:
		return "missing-credentials"
	case s.NoTargetsSelected:
		return "no-targets-selected"
	}
	return "ok"
}
