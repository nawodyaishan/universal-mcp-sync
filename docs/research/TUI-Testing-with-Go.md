> From: https://chatgpt.com/c/6a10615a-0f78-83ab-90e6-c0540a018cd4

# chatgpt response

I’ll ground the guide in the current Charmbracelet testing APIs, then structure it as a practical playbook for your exact module: unit tests, golden tests, `teatest`, and full black-box CLI/TUI flows.

The guide will assume Bubble Tea v1 import paths (`github.com/charmbracelet/bubbletea`), not the newer `charm.land/.../v2` paths, because your `go.mod` is pinned to `github.com/charmbracelet/bubbletea v1.3.10`.

# Full Guide: Testing Go CLI/TUI Apps with Bubble Tea, Bubbles, Huh, Lip Gloss, and `teatest`

Audience: **Go CLI/TUI developers** building `github.com/nawodyaishan/universal-mcp-sync` with:

```go
github.com/charmbracelet/bubbletea v1.3.10
github.com/charmbracelet/bubbles v1.0.0
github.com/charmbracelet/huh v1.0.0
github.com/charmbracelet/lipgloss v1.1.0
github.com/charmbracelet/x/exp/teatest ...
```

This guide focuses on a **Playwright-like testing style for Bubble Tea apps**, using your existing stack. Bubble Tea apps are naturally testable because they are built around `Model`, `Init`, `Update`, and `View`; `Update` handles messages, and `View` renders the current UI. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/bubbletea)) `teatest` is the closest included tool to a Playwright-style test harness: it can run a Bubble Tea model, send messages or typed input, inspect output, assert golden files, and read the final model. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest))

---

## 1. What “Playwright-like” means for TUIs

In web Playwright, you usually do this:

```ts
await page.goto(...)
await page.getByText("Login").click()
await expect(page.getByText("Welcome")).toBeVisible()
```

For Bubble Tea, the equivalent is:

```go
tm := teatest.NewTestModel(t, NewModel(...), teatest.WithInitialTermSize(100, 30))

teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
	return bytes.Contains(b, []byte("Universal MCP Sync"))
})

tm.Type("github")
tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
	return bytes.Contains(b, []byte("Sync completed"))
})
```

The key idea: **do not sleep**. Wait for terminal output or model state. `teatest.WaitFor` keeps reading from an `io.Reader` until a condition matches; its defaults are 1 second duration and 50 ms check interval, and both can be changed with options. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest))

---

## 2. Recommended test pyramid

Use four layers:

```text
Layer 4: Binary / CLI smoke tests
  - Test compiled command behavior with os/exec
  - Optional real terminal testing with strider/tmux later

Layer 3: teatest integration tests
  - Run the real Bubble Tea model
  - Type input
  - Send key messages
  - Wait for rendered output
  - Assert final model

Layer 2: View / golden tests
  - Snapshot rendered views
  - Stabilize colors and terminal width

Layer 1: Unit tests
  - Pure functions
  - Update state transitions
  - Commands with fake services
  - Bubbles and Huh form state
```

Most bugs should be caught in layers 1–3. Only a small number of tests should run the compiled binary.

---

## 3. Suggested project layout

For `universal-mcp-sync`, design the app so the TUI model can be tested without starting the real CLI binary.

```text
universal-mcp-sync/
  cmd/
    universal-mcp-sync/
      main.go
  internal/
    app/
      model.go
      update.go
      view.go
      keys.go
      commands.go
      services.go
      model_test.go
      view_test.go
      teatest_test.go
    cli/
      root.go
      root_test.go
    testutil/
      fake_sync_service.go
      normalize.go
  testdata/
    ...
```

The important design rule is: **`main.go` should be thin**.

```go
package main

import (
	"fmt"
	"os"

	"github.com/nawodyaishan/universal-mcp-sync/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := app.NewModel(app.DefaultDeps())

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

That lets tests instantiate `app.NewModel(...)` directly.

---

## 4. Make the model dependency-injectable

Do not call network, file system, MCP APIs, or external commands directly from `Update`. Wrap them behind interfaces.

```go
package app

import "context"

type SyncService interface {
	ListTargets(ctx context.Context) ([]Target, error)
	Sync(ctx context.Context, target string) error
}

type Target struct {
	Name string
	Path string
}

type Deps struct {
	Sync SyncService
}

func DefaultDeps() Deps {
	return Deps{
		Sync: realSyncService{},
	}
}
```

Then the model owns dependencies:

```go
package app

import tea "github.com/charmbracelet/bubbletea"

type screen int

const (
	screenHome screen = iota
	screenTargets
	screenSyncing
	screenDone
	screenError
)

type Model struct {
	deps Deps

	screen screen
	width  int
	height int

	targets []Target
	cursor  int
	err     error
	status  string
}

func NewModel(deps Deps) Model {
	return Model{
		deps:   deps,
		screen: screenHome,
		status: "Ready",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

---

## 5. Unit-test `Update` first

Bubble Tea’s `Update` method receives messages and returns the updated model plus an optional command. This makes state transition tests cheap and fast. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/bubbletea))

Example `Update`:

```go
package app

import tea "github.com/charmbracelet/bubbletea"

type targetsLoadedMsg struct {
	targets []Target
}

type syncFinishedMsg struct {
	err error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "down", "j":
			if m.cursor < len(m.targets)-1 {
				m.cursor++
			}
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "enter":
			if m.screen == screenTargets && len(m.targets) > 0 {
				m.screen = screenSyncing
				target := m.targets[m.cursor].Name
				return m, m.syncCmd(target)
			}
		}

	case targetsLoadedMsg:
		m.targets = msg.targets
		m.screen = screenTargets
		return m, nil

	case syncFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.screen = screenDone
		m.status = "Sync completed"
		return m, nil
	}

	return m, nil
}
```

Unit test:

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateMovesCursorDown(t *testing.T) {
	m := NewModel(Deps{})
	m.targets = []Target{
		{Name: "local"},
		{Name: "remote"},
	}
	m.screen = screenTargets

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("expected no command")
	}

	got := next.(Model)

	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
}

func TestUpdateDoesNotMoveCursorPastEnd(t *testing.T) {
	m := NewModel(Deps{})
	m.targets = []Target{
		{Name: "local"},
		{Name: "remote"},
	}
	m.screen = screenTargets
	m.cursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := next.(Model)

	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := NewModel(Deps{})

	next, _ := m.Update(tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	})

	got := next.(Model)

	if got.width != 120 || got.height != 40 {
		t.Fatalf("size = %dx%d, want 120x40", got.width, got.height)
	}
}
```

---

## 6. Test commands with fake services

Commands should be small wrappers around injected dependencies.

```go
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) loadTargetsCmd() tea.Cmd {
	return func() tea.Msg {
		targets, err := m.deps.Sync.ListTargets(context.Background())
		if err != nil {
			return syncFinishedMsg{err: err}
		}
		return targetsLoadedMsg{targets: targets}
	}
}

func (m Model) syncCmd(target string) tea.Cmd {
	return func() tea.Msg {
		err := m.deps.Sync.Sync(context.Background(), target)
		return syncFinishedMsg{err: err}
	}
}
```

Fake service:

```go
package app

import (
	"context"
	"errors"
)

type fakeSyncService struct {
	targets []Target
	err     error
	synced  string
}

func (f *fakeSyncService) ListTargets(ctx context.Context) ([]Target, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.targets, nil
}

func (f *fakeSyncService) Sync(ctx context.Context, target string) error {
	f.synced = target
	return f.err
}

func TestSyncCmdSuccess(t *testing.T) {
	fake := &fakeSyncService{}

	m := NewModel(Deps{Sync: fake})
	cmd := m.syncCmd("local")

	msg := cmd()
	done := msg.(syncFinishedMsg)

	if done.err != nil {
		t.Fatalf("err = %v, want nil", done.err)
	}

	if fake.synced != "local" {
		t.Fatalf("synced = %q, want local", fake.synced)
	}
}

func TestSyncCmdFailure(t *testing.T) {
	fake := &fakeSyncService{
		err: errors.New("network unavailable"),
	}

	m := NewModel(Deps{Sync: fake})
	msg := m.syncCmd("remote")()

	done := msg.(syncFinishedMsg)

	if done.err == nil {
		t.Fatal("expected error")
	}
}
```

---

## 7. Make `View` deterministic

Your `View` should render based only on model state. Avoid calling time, random, network, file system, or terminal APIs from `View`.

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	errorStyle = lipgloss.NewStyle().Bold(true)
)

func (m Model) View() string {
	switch m.screen {
	case screenHome:
		return titleStyle.Render("Universal MCP Sync") +
			"\n\nPress enter to load targets.\nPress q to quit."

	case screenTargets:
		var b strings.Builder
		b.WriteString(titleStyle.Render("Select sync target"))
		b.WriteString("\n\n")

		for i, target := range m.targets {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", cursor, target.Name)
		}

		b.WriteString("\nenter: sync • q: quit")
		return b.String()

	case screenSyncing:
		return "Syncing..."

	case screenDone:
		return "Sync completed"

	case screenError:
		return errorStyle.Render("Error: " + m.err.Error())
	}

	return ""
}
```

Simple view test:

```go
package app

import (
	"strings"
	"testing"
)

func TestViewHome(t *testing.T) {
	m := NewModel(Deps{})
	out := m.View()

	if !strings.Contains(out, "Universal MCP Sync") {
		t.Fatalf("view does not contain title:\n%s", out)
	}

	if !strings.Contains(out, "Press q to quit") {
		t.Fatalf("view does not contain quit help:\n%s", out)
	}
}

func TestViewTargetsShowsCursor(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenTargets
	m.targets = []Target{
		{Name: "local"},
		{Name: "remote"},
	}
	m.cursor = 1

	out := m.View()

	if !strings.Contains(out, "> remote") {
		t.Fatalf("view does not show selected target:\n%s", out)
	}
}
```

---

## 8. Stabilize colors for CI

Golden terminal tests often fail because local terminals and CI report different color capabilities. Charm’s own `teatest` guide recommends forcing a stable color profile with `lipgloss.SetColorProfile(termenv.Ascii)` and adding `*.golden -text` to `.gitattributes` to avoid line-ending changes. ([Charm](https://charm.land/blog/teatest/))

Create `internal/app/test_setup_test.go`:

```go
package app

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}
```

Because this imports `github.com/muesli/termenv` directly, `go mod tidy` may move it from indirect to direct. That is fine.

Add `.gitattributes`:

```gitattributes
*.golden -text
```

---

## 9. Golden tests for views

You already have `github.com/charmbracelet/x/exp/golden` indirectly through the Charm testing stack. The package compares output to golden files and supports updating golden files with `-update`. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/golden))

```go
package app

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

func TestViewTargetsGolden(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenTargets
	m.targets = []Target{
		{Name: "local"},
		{Name: "remote"},
		{Name: "production"},
	}
	m.cursor = 1

	golden.RequireEqual(t, m.View())
}
```

First run:

```bash
go test ./internal/app -run TestViewTargetsGolden -update
```

Normal run:

```bash
go test ./...
```

Commit the generated file under `testdata`.

---

## 10. `teatest`: integration testing your model

`teatest.NewTestModel` creates a testable Bubble Tea model. Its API includes `Output`, `FinalOutput`, `FinalModel`, `Send`, `Type`, `WaitFinished`, `Quit`, and `WithInitialTermSize`. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest))

Basic test:

```go
package app

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTUIHomeScreen(t *testing.T) {
	m := NewModel(Deps{})

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(
		t,
		tm.Output(),
		func(b []byte) bool {
			return bytes.Contains(b, []byte("Universal MCP Sync"))
		},
		teatest.WithDuration(2*time.Second),
		teatest.WithCheckInterval(25*time.Millisecond),
	)

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})

	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
```

Use this pattern for “screen appears”, “type text”, “press enter”, “wait for result”, and “assert final state”.

---

## 11. Test a full selection flow

```go
package app

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTUISyncSelectedTarget(t *testing.T) {
	fake := &fakeSyncService{
		targets: []Target{
			{Name: "local"},
			{Name: "remote"},
		},
	}

	m := NewModel(Deps{Sync: fake})
	m.screen = screenTargets
	m.targets = fake.targets

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("local")) &&
			bytes.Contains(b, []byte("remote"))
	}, teatest.WithDuration(time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Sync completed"))
	}, teatest.WithDuration(2*time.Second))

	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	if fake.synced != "remote" {
		t.Fatalf("synced = %q, want remote", fake.synced)
	}
}
```

Note: this assumes your app quits after sync completion. If it does not, send `q` before `WaitFinished`.

---

## 12. Assert the final model

Charm’s guide shows that `FinalModel` returns the model after the program has finished. ([Charm](https://charm.land/blog/teatest/))

```go
func TestFinalModelAfterQuit(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenDone
	m.status = "Sync completed"

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(Model)

	if got.status != "Sync completed" {
		t.Fatalf("status = %q, want Sync completed", got.status)
	}
}
```

---

## 13. Test final output with a golden file

`teatest.RequireEqualOutput` compares output with a golden file and can update golden files with the `-update` flag. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest))

```go
package app

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestFinalOutputGolden(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenDone
	m.status = "Sync completed"

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(100, 30),
	)

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(time.Second)))
	if err != nil {
		t.Fatal(err)
	}

	teatest.RequireEqualOutput(t, out)
}
```

Generate or update:

```bash
go test ./internal/app -run TestFinalOutputGolden -update
```

Then run normally:

```bash
go test ./internal/app -run TestFinalOutputGolden
```

---

## 14. Testing text input flows

For text entry, use `tm.Type`.

Example model:

```go
type Model struct {
	// ...
	filter string
}
```

Example update:

```go
case tea.KeyMsg:
	switch msg.Type {
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		return m, nil

	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m, nil
	}
```

Test:

```go
func TestTUITypeFilter(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenTargets
	m.targets = []Target{
		{Name: "github"},
		{Name: "gitlab"},
		{Name: "local"},
	}

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(100, 30),
	)

	tm.Type("git")

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("git"))
	}, teatest.WithDuration(time.Second))

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})

	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
```

---

## 15. Testing Huh forms

Huh is built on Bubble Tea, supports standalone forms, can be integrated into Bubble Tea apps, and a `huh.Form` is itself a `tea.Model`. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/huh)) That means you can treat Huh forms like nested Bubble Tea components.

Example:

```go
package app

import (
	"fmt"

	"github.com/charmbracelet/huh"
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigModel struct {
	form *huh.Form
}

func NewConfigModel() ConfigModel {
	return ConfigModel{
		form: huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Key("server").
					Title("MCP server URL"),
				huh.NewConfirm().
					Key("enabled").
					Title("Enable sync?"),
			),
		),
	}
}

func (m ConfigModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m ConfigModel) View() string {
	if m.form.State == huh.StateCompleted {
		return fmt.Sprintf(
			"Configured: %s",
			m.form.GetString("server"),
		)
	}

	return m.form.View()
}
```

Test through `teatest`:

```go
func TestHuhConfigForm(t *testing.T) {
	m := NewConfigModel()

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("MCP server URL"))
	}, teatest.WithDuration(time.Second))

	tm.Type("http://localhost:3000")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Enable sync?"))
	}, teatest.WithDuration(time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Configured: http://localhost:3000"))
	}, teatest.WithDuration(time.Second))

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})
}
```

For accessibility-specific form behavior, Huh also supports accessible mode, which avoids Bubble Tea’s renderer and uses basic prompting to make screen-reader interaction easier. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/huh))

---

## 16. Testing Bubbles components

Bubbles provides common Bubble Tea components such as text inputs, lists, tables, progress, paginator, viewport, help, and key bindings. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/bubbletea)) Test them either through your parent model or directly if your wrapper contains business behavior.

Example with a text input:

```go
package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type SearchModel struct {
	input textinput.Model
}

func NewSearchModel() SearchModel {
	input := textinput.New()
	input.Placeholder = "Search targets"
	input.Focus()

	return SearchModel{
		input: input,
	}
}

func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m SearchModel) View() string {
	return m.input.View()
}

func TestSearchInputUpdatesValue(t *testing.T) {
	m := NewSearchModel()

	next, _ := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("a"),
	})

	got := next.(SearchModel)

	if got.input.Value() != "a" {
		t.Fatalf("input = %q, want a", got.input.Value())
	}
}
```

---

## 17. Normalize output when needed

Terminal output may include ANSI escape codes, cursor movement, and full-screen redraws. For some tests, do not compare raw output. Wait for stable substrings instead.

```go
func containsAll(b []byte, values ...string) bool {
	for _, value := range values {
		if !bytes.Contains(b, []byte(value)) {
			return false
		}
	}
	return true
}
```

Usage:

```go
teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
	return containsAll(b,
		"Universal MCP Sync",
		"local",
		"remote",
	)
}, teatest.WithDuration(time.Second))
```

For golden tests, prefer deterministic views and fixed terminal sizes.

---

## 18. Avoid flaky TUI tests

Use these rules:

```text
Do:
  - Use fixed terminal sizes.
  - Use fake services.
  - Use WaitFor instead of time.Sleep.
  - Use deterministic colors.
  - Keep golden files small.
  - Assert key UI text, not every escape sequence.

Avoid:
  - Real network calls.
  - Real home directory config.
  - Random order maps in View.
  - Current time in View.
  - Spinners in golden tests unless frozen.
  - Sleeping without waiting for a condition.
```

`teatest.WaitFor`, `WithDuration`, and `WithCheckInterval` are designed for condition-based waiting rather than fixed sleeps. ([Go Packages](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest))

---

## 19. Test CLI behavior separately from TUI behavior

If your CLI has flags such as:

```bash
universal-mcp-sync --config ./config.yaml
universal-mcp-sync sync --target local
universal-mcp-sync tui
```

test the non-TUI command behavior without Bubble Tea.

A simple pattern:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestRootHelp(t *testing.T) {
	cmd := NewRootCommand()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(out.Bytes(), []byte("universal-mcp-sync")) {
		t.Fatalf("help output missing app name:\n%s", out.String())
	}
}
```

For TUI mode, test the model with `teatest`, not the CLI parser.

---

## 20. Optional: true black-box terminal tests

Your current stack already supports strong in-process Bubble Tea testing. For true “run the compiled binary inside a terminal” tests, you can optionally add a tool like `strider`.

`strider` runs binaries inside isolated tmux sessions, sends keystrokes, captures screen output, and asserts against it; it describes its waits as Playwright-like auto-waiting locators. It requires Go 1.24+, tmux 3.0+, and a Unix-like OS. ([GitHub](https://github.com/cboone/strider))

Optional example:

```go
func TestBinaryTUI(t *testing.T) {
	term := strider.Open(t, "./universal-mcp-sync",
		strider.WithArgs("tui"),
		strider.WithSize(100, 30),
		strider.WithEnv("NO_COLOR=1"),
	)

	term.WaitFor(strider.Text("Universal MCP Sync"))
	term.Press(strider.Enter)
	term.WaitFor(strider.Text("Select sync target"))
}
```

Use this sparingly. Most tests should stay in `teatest`.

---

## 21. GitHub Actions CI example

```yaml
name: test

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.2'
          cache: true

      - name: Test
        env:
          NO_COLOR: "1"
          TERM: xterm-256color
        run: go test ./...
```

If you later add `strider`, install tmux:

```yaml
      - name: Install tmux
        run: sudo apt-get update && sudo apt-get install -y tmux
```

---

## 22. Practical naming convention

Use clear test names:

```text
TestUpdateMovesCursorDown
TestUpdateStartsSyncOnEnter
TestViewTargetsGolden
TestTUIHomeScreen
TestTUISyncSelectedTarget
TestHuhConfigForm
TestCLIHelp
```

Use package-local tests for private model state:

```go
package app
```

Use external tests only when testing public API:

```go
package app_test
```

---

## 23. Recommended helper file

Create `internal/app/test_helpers_test.go`:

```go
package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/charmbracelet/x/exp/teatest"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func waitForText(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}

func waitForAll(t *testing.T, tm *teatest.TestModel, values ...string) {
	t.Helper()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		for _, value := range values {
			if !bytes.Contains(b, []byte(value)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}
```

Usage:

```go
func TestTUIShowsTargets(t *testing.T) {
	m := NewModel(Deps{})
	m.screen = screenTargets
	m.targets = []Target{
		{Name: "local"},
		{Name: "remote"},
	}

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))

	waitForAll(t, tm, "Select sync target", "local", "remote")
}
```

---

## 24. What to test for `universal-mcp-sync`

A good starting suite:

```text
Model state:
  - starts on home screen
  - window size updates layout
  - j/down moves cursor down
  - k/up moves cursor up
  - cursor does not overflow
  - enter starts sync
  - sync success shows done screen
  - sync failure shows error screen
  - q quits

View:
  - home screen golden
  - target list golden
  - syncing screen golden
  - success screen golden
  - error screen golden

Commands:
  - load targets success
  - load targets failure
  - sync success
  - sync failure

teatest:
  - home screen appears
  - target list interaction
  - select remote target and sync
  - error flow
  - Huh config form flow

CLI:
  - --help
  - invalid args
  - non-interactive sync command
  - config path flag
```

---

## 25. Final recommendation

For your current dependency set, use this default strategy:

```text
Use plain Go tests for Update, commands, and View.
Use golden tests for stable visual layouts.
Use teatest for Playwright-like in-process TUI flows.
Use os/exec for non-interactive CLI smoke tests.
Optionally add strider only when you need real binary + real terminal coverage.
```

The most important architectural choice is to keep your Bubble Tea model independent from `main.go` and external services. Once your model can be created with fake dependencies, `teatest` gives you almost everything you need for reliable, Playwright-style TUI testing.