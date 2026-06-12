package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/tui"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/uxexplore"
)

func runReplayCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("usync replay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var emitMatrix bool
	var realtime bool
	var againstFixture string
	flags.BoolVar(&emitMatrix, "emit-matrix", false, "emit a Phase-12-format matrix-row stub for the transcript")
	flags.BoolVar(&realtime, "realtime", false, "preserve recorded timing instead of replaying as fast as possible (no-op in current build)")
	flags.StringVar(&againstFixture, "against-fixture", "happy-path-exa", "uxexplore fixture name to replay against")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: usync replay [flags] <transcript.jsonl>")
		return 2
	}
	transcript := flags.Arg(0)
	entries, err := parseTranscript(transcript)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	spec, err := pickFixture(againstFixture)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	driver, err := uxexplore.NewDriver(spec)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "driver: %v\n", err)
		return 1
	}

	finalView, finalScreen, finalPC, err := replayAgainstFixture(driver, entries)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "replay: %v\n", err)
		return 1
	}

	digest := sha256Hex(finalView)
	if emitMatrix {
		_, _ = fmt.Fprintln(stdout, emitMatrixStub(spec.Name, entries, finalScreen, finalPC, digest))
		return 0
	}
	_, _ = fmt.Fprintln(stdout, finalView)
	_, _ = fmt.Fprintf(stdout, "\n# final screen=%s precondition_class=%s digest=%s\n", finalScreen, finalPC, digest)

	// Compare digest from final entry if present.
	expected := ""
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Kind == "final" && entries[i].Digest != "" {
			expected = entries[i].Digest
			break
		}
	}
	if expected != "" && expected != digest {
		_, _ = fmt.Fprintf(stderr, "replay: final-state digest mismatch (transcript=%s, replay=%s)\n", expected, digest)
		return 1
	}
	return 0
}

func parseTranscript(path string) ([]tui.RecordEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer func() { _ = f.Close() }()
	var out []tui.RecordEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry tui.RecordEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse transcript line: %w", err)
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func pickFixture(name string) (uxexplore.FixtureSpec, error) {
	for _, f := range uxexplore.EnumerateFixtures() {
		if f.Name == name {
			return f, nil
		}
	}
	return uxexplore.FixtureSpec{}, fmt.Errorf("unknown fixture %q (see EnumerateFixtures)", name)
}

func replayAgainstFixture(d *uxexplore.Driver, entries []tui.RecordEntry) (string, string, string, error) {
	m, err := d.StartModel(context.Background())
	if err != nil {
		return "", "", "", err
	}
	if shouldPrimeDoctorMode(m, entries) {
		var quit bool
		m, quit, err = replayDashboardKey(m, buildReplayKey("d"))
		if err != nil {
			return "", "", "", err
		}
		if quit {
			snap := m.Snapshot()
			return m.View(), snap.Screen, snap.BlockReason, nil
		}
	}
	for _, e := range entries {
		if e.Kind != "key" {
			continue
		}
		if strings.HasPrefix(e.Screen, "Wizard") {
			continue
		}
		msg := buildReplayKey(e.Key)
		var quit bool
		m, quit, err = replayDashboardKey(m, msg)
		if err != nil {
			return "", "", "", fmt.Errorf("replay key %q: %w", e.Key, err)
		}
		if quit || m.RouteToWizard {
			break
		}
	}
	snap := m.Snapshot()
	return m.View(), snap.Screen, snap.BlockReason, nil
}

func shouldPrimeDoctorMode(m tui.DashboardModel, entries []tui.RecordEntry) bool {
	if m.Snapshot().Screen != "Welcome" {
		return false
	}
	for _, e := range entries {
		if e.Kind != "key" {
			continue
		}
		return e.Screen == "" || e.Screen == "Doctor"
	}
	return false
}

func replayDashboardKey(m tui.DashboardModel, msg tea.KeyMsg) (tui.DashboardModel, bool, error) {
	next, cmd := m.Update(msg)
	dm, ok := next.(tui.DashboardModel)
	if !ok {
		return m, false, fmt.Errorf("model is not DashboardModel")
	}
	for steps := 0; steps < 16 && cmd != nil; steps++ {
		follow := cmd()
		if follow == nil {
			return dm, false, nil
		}
		if _, isQuit := follow.(tea.QuitMsg); isQuit {
			return dm, true, nil
		}
		next, cmd = dm.Update(follow)
		var ok bool
		dm, ok = next.(tui.DashboardModel)
		if !ok {
			return dm, false, fmt.Errorf("model is not DashboardModel after command")
		}
	}
	return dm, false, nil
}

func buildReplayKey(label string) tea.KeyMsg {
	switch strings.ToLower(label) {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space", " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(label)}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func emitMatrixStub(fixture string, entries []tui.RecordEntry, finalScreen, finalPC, digest string) string {
	var b strings.Builder
	id := "DM-RP" + sha256Hex(digest)[:8]
	fmt.Fprintf(&b, "## %s — replay of %s\n\n", id, fixture)
	fmt.Fprintf(&b, "- Fixture: `%s`\n", fixture)
	fmt.Fprintf(&b, "- Final state: `%s/%s`\n", finalScreen, finalPC)
	b.WriteString("- Key sequence:\n")
	for _, e := range entries {
		if e.Kind == "key" {
			fmt.Fprintf(&b, "  - `%s` on `%s`\n", e.Key, e.Screen)
		}
	}
	fmt.Fprintf(&b, "- View digest: `%s`\n", digest)
	b.WriteString("- Expected: _(human-filled)_\n")
	b.WriteString("- Invariants: I-01, I-13, I-17\n")
	return b.String()
}
