package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func TestSetupFormSyncsToContext(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	registry := provider.DefaultRegistry()
	ctx := &wizardContext{
		manager:  manager,
		registry: registry,
		selected: make(map[config.AppID]bool),
	}
	sf := newSetupForm(ctx, "11111111-1111-1111-1111-111111111111")
	sf.selectedProvider = "exa"
	sf.selectedSlice = []config.AppID{config.AppClaudeDesktop}
	sf.syncToContext()

	if !ctx.selected[config.AppClaudeDesktop] {
		t.Fatal("expected Claude Desktop to be selected in context")
	}
	if len(ctx.profiles) != 1 || ctx.profiles[0].Values["EXA_API_KEY"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected profile to be synced, got %#v", ctx.profiles)
	}
}

func TestModel(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	m := NewModel(manager, nil, "")

	// Init
	m.Init()

	// Update WindowSize
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100})
	if m2.(Model).width != 100 {
		t.Errorf("expected width 100, got %d", m2.(Model).width)
	}

	// View
	m2.View()

	// Err
	if m2.(Model).Err() != nil {
		t.Errorf("expected nil error")
	}
}

func TestWizardViewUsesModernShell(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	m := NewModel(manager, nil, "")

	view := m.View()
	for _, want := range []string{"Wizard Mode", "guided setup", "Setup", "providers", "target apps"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected wizard view to contain %q\n\n%s", want, view)
		}
	}
}

func TestModelPreloaded(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	keys := []string{"11111111-1111-1111-1111-111111111111"}
	m := NewModel(manager, keys, "")
	if !m.ctx.isPreloaded {
		t.Error("expected preloaded true")
	}
	if len(m.ctx.profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(m.ctx.profiles))
	}
}

func TestPreviewModel_EnterStartsApply(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)

	next, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, ok := next.(previewModel)
	if !ok {
		t.Fatal("expected previewModel")
	}
	if !p.applying {
		t.Error("expected applying=true after enter")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (batch of apply+spinner tick)")
	}
}

func TestPreviewModel_ResultTransitionsToNext(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)
	pm.applying = true

	result := app.ApplyResult{}
	next, cmd := pm.Update(applyResultMsg{result: result, err: nil})
	p, ok := next.(previewModel)
	if !ok {
		t.Fatal("expected previewModel")
	}
	if p.applying {
		t.Error("expected applying=false after result")
	}
	if ctx.err != nil {
		t.Errorf("expected no error, got %v", ctx.err)
	}
	if cmd == nil {
		t.Error("expected signalNext cmd")
	}
	// Verify the cmd fires a nextMsg
	msg := cmd()
	if _, ok := msg.(nextMsg); !ok {
		t.Errorf("expected nextMsg, got %T", msg)
	}
}

func TestPreviewModel_ErrorStaysOnPreview(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)
	pm.applying = true

	applyErr := errors.New("disk full")
	next, cmd := pm.Update(applyResultMsg{err: applyErr})
	p, ok := next.(previewModel)
	if !ok {
		t.Fatal("expected previewModel")
	}
	if p.applying {
		t.Error("expected applying=false after error result")
	}
	if ctx.err == nil || ctx.err.Error() != "disk full" {
		t.Errorf("expected ctx.err='disk full', got %v", ctx.err)
	}
	if cmd != nil {
		msg := cmd()
		if _, isNext := msg.(nextMsg); isNext {
			t.Error("must not advance stage on error")
		}
	}
}

func TestPreviewModel_BlocksInputWhileApplying(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)
	pm.applying = true

	next, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p, ok := next.(previewModel)
	if !ok {
		t.Fatal("expected previewModel")
	}
	if !p.applying {
		t.Error("applying should remain true while blocked")
	}
	if cmd != nil {
		t.Error("no cmd expected when input is blocked")
	}
}

func TestPreviewModel_SpinnerTickUpdatesFrame(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)
	pm.applying = true

	tick := pm.spinner.Tick() // invoke the Cmd to get the TickMsg
	next, cmd := pm.Update(tick)
	_, ok := next.(previewModel)
	if !ok {
		t.Fatal("expected previewModel")
	}
	if cmd == nil {
		t.Error("spinner should schedule another tick while applying")
	}
}

func TestPreviewModel_SpinnerTickIgnoredWhenNotApplying(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	ctx := &wizardContext{manager: manager}
	pm := newPreviewModel(ctx)
	// applying == false

	tick := pm.spinner.Tick()
	_, cmd := pm.Update(tick)
	if cmd != nil {
		t.Error("spinner tick should be ignored when not applying")
	}
}

func TestRenderPreviewPlanDoesNotExposeStdioArgs(t *testing.T) {
	plan := app.ExecutionPlan{
		Operations: []app.Operation{
			{
				AppName:         "Example App",
				FileLabel:       "global config",
				Path:            "/tmp/test/config.json",
				CredentialLabel: "redacted-key",
				ProviderID:      "example",
				Config: provider.MCPConfig{
					Type:    provider.TransportStdio,
					Command: "mcp-server",
					Args:    []string{"--api-key", "super-secret-token"},
				},
			},
		},
	}

	view := renderPreviewPlan(plan, "/tmp/test")
	if strings.Contains(view, "super-secret-token") || strings.Contains(view, "--api-key") {
		t.Fatalf("preview must not expose stdio args containing secrets\n\n%s", view)
	}
	if !strings.Contains(view, "stdio command mcp-server") {
		t.Fatalf("expected preview to keep the stdio command context\n\n%s", view)
	}
}

func TestModel_SubModelStatePreserved(t *testing.T) {
	manager, _ := app.NewManager("/tmp/test", nil, nil)
	keys := []string{"11111111-1111-1111-1111-111111111111"}
	m := NewModel(manager, keys, "")

	// Manually advance to assignments stage so cursor navigation is active
	m.stage = stageAssignments
	m.ctx.selected = map[config.AppID]bool{config.AppClaudeDesktop: true, config.AppClaudeCode: true}

	// Move cursor down
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model2 := m2.(Model)
	if model2.assignments.cursor != 1 {
		t.Errorf("expected cursor=1 after down key, got %d", model2.assignments.cursor)
	}

	// Move cursor up
	m3, _ := model2.Update(tea.KeyMsg{Type: tea.KeyUp})
	model3 := m3.(Model)
	if model3.assignments.cursor != 0 {
		t.Errorf("expected cursor=0 after up key, got %d", model3.assignments.cursor)
	}
}
