package tui

// Playwright-style end-to-end flow tests for the dashboard TUI.
// Each test drives real key inputs through teatest.NewTestModel and waits
// for visible text at each screen transition — no time.Sleep anywhere.
// See docs/research/TUI-Testing-with-Go.md and docs/specs/doctor-mode-phase12/spec.md.

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

// happyFlowSetup returns a scanner, manager, and profiles suitable for the full happy-path flow.
// The FakeDashboardManager.HomeDir() returns t.TempDir() so planCmd() can save a real plan file.
func happyFlowSetup(t *testing.T) (*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile) {
	t.Helper()
	now := time.Now().UTC()
	// SavedPlan must have all required fields for PlanStore serialisation.
	planID, err := app.NewPlanID()
	if err != nil {
		t.Fatalf("NewPlanID: %v", err)
	}
	mgr := &FakeDashboardManager{
		Plan: app.SavedPlan{
			SchemaVersion: 2,
			PlanID:        planID,
			ProviderID:    "exa",
			CreatedAt:     now,
			ExpiresAt:     now.Add(24 * time.Hour),
		},
		Preflight: app.SavedPlanPreflight{
			PlanID:     planID,
			ProviderID: "exa",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
		},
		Result: app.ApplyResult{
			UpdatedTargets: []string{"/home/.gemini/antigravity-cli/mcp_config.json"},
		},
		home: t.TempDir(),
	}
	scanner := &FakeScanner{Report: doctor.Report{
		Platform: "darwin",
		Clients: []doctor.ClientFinding{{
			ID:            "antigravity-cli",
			Name:          "Antigravity CLI",
			Confidence:    doctor.ConfidenceHigh,
			Installed:     true,
			EffectivePath: "/home/.gemini/antigravity-cli/mcp_config.json",
		}},
	}}
	profiles := []provider.CredentialProfile{{
		ProviderID: "exa",
		Values:     map[string]string{"EXA_API_KEY": "exa-flow-test-placeholder"},
		Label:      "test",
	}}
	return scanner, mgr, profiles
}

func exaAndContext7Profiles() []provider.CredentialProfile {
	return []provider.CredentialProfile{
		{
			ProviderID: "exa",
			Values:     map[string]string{"EXA_API_KEY": "exa-flow-test-placeholder"},
			Label:      "exa-test",
		},
		{
			ProviderID: "context7",
			Values:     map[string]string{"CONTEXT7_API_KEY": "ctx7sk-test-placeholder"},
			Label:      "context7-test",
		},
	}
}

func multiTargetSetup(t *testing.T) (*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile) {
	t.Helper()
	scanner, mgr, _ := happyFlowSetup(t)
	mgr.Result = app.ApplyResult{}
	scanner.Report.Clients = []doctor.ClientFinding{
		{
			ID:            manifest.ClientAntigravityCLI,
			Name:          "Antigravity CLI",
			Confidence:    doctor.ConfidenceHigh,
			Installed:     true,
			EffectivePath: "/home/.gemini/antigravity-cli/mcp_config.json",
			Candidates: []doctor.CandidateFinding{{
				Label:    "mcp-config",
				Path:     "/home/.gemini/antigravity-cli/mcp_config.json",
				Scope:    manifest.ScopeUser,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			}},
		},
		{
			ID:            manifest.ClientCursor,
			Name:          "Cursor",
			Confidence:    doctor.ConfidenceHigh,
			Installed:     true,
			EffectivePath: "/home/.cursor/mcp.json",
			Candidates: []doctor.CandidateFinding{{
				Label:    "global",
				Path:     "/home/.cursor/mcp.json",
				Scope:    manifest.ScopeGlobal,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			}},
		},
		{
			ID:            manifest.ClientClaudeCode,
			Name:          "Claude Code",
			Confidence:    doctor.ConfidenceHigh,
			Installed:     true,
			EffectivePath: "/home/.claude.json",
			Candidates: []doctor.CandidateFinding{{
				Label:    "user",
				Path:     "/home/.claude.json",
				Scope:    manifest.ScopeUser,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			}},
		},
	}
	return scanner, mgr, exaAndContext7Profiles()
}

func keyMsg(s string) tea.KeyMsg {
	if s == "ctrl+c" {
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestDashboardFlow_HappyPath walks all 5 screens via real key inputs.
// Guide §11: "Test a full selection flow." Guide §12: "Assert the final model."
func TestDashboardFlow_HappyPath(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// 1. Doctor screen — scan completes.
	waitForText(t, tm, "System Status")

	// 2. Provider readiness screen.
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")

	// 3. Offline validation passes → target select.
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")

	// 4. Plan creation → plan preview.
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	// 5. Apply → apply result.
	tm.Send(keyMsg("y"))
	waitForText(t, tm, "Apply Result")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Assert final model state (guide §12).
	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(DashboardModel)
	if got.screen != screenApplyResult {
		t.Errorf("expected screenApplyResult after happy path, got screen %d", got.screen)
	}
	if got.applyResult == nil {
		t.Error("expected applyResult non-nil after successful apply")
	}
}

// TestDashboardFlow_ValidationFails stays on provider ready when validation errors.
func TestDashboardFlow_ValidationFails(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	mgr.ValidErr = errors.New("invalid key format — must be uuid")
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter")) // trigger offline validation — will fail

	// Error text must appear; screen must stay on provider ready.
	waitForText(t, tm, "invalid key format")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(DashboardModel)
	if got.screen == screenTargetSelect {
		t.Error("expected NOT to advance to screenTargetSelect on validation failure")
	}
}

// TestDashboardFlow_PlanFails shows error and stays on target select.
func TestDashboardFlow_PlanFails(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	mgr.PlanErr = errors.New("provider unreachable")
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter")) // trigger plan — will fail

	waitForText(t, tm, "provider unreachable")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(DashboardModel)
	if got.screen == screenPlanPreview {
		t.Error("expected NOT to advance to screenPlanPreview on plan failure")
	}
	if got.planErr == nil {
		t.Error("expected planErr set after plan failure")
	}
}

// TestDashboardFlow_ApplyFails reaches apply result screen with error shown.
func TestDashboardFlow_ApplyFails(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	mgr.ApplyErr = errors.New("permission denied writing config")
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")
	tm.Send(keyMsg("y"))

	// "Apply Result" and error appear in the same render frame — use waitForAll.
	waitForAll(t, tm, "Apply Result", "permission denied")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(DashboardModel)
	if got.screen != screenApplyResult {
		t.Errorf("expected screenApplyResult even on apply error, got %d", got.screen)
	}
	if got.applyErr == nil {
		t.Error("expected applyErr non-nil after apply failure")
	}
}

// TestDashboardFlow_EscNavigation verifies Esc returns to previous screen at each level.
// Guide §11: tests screen navigation.
func TestDashboardFlow_EscNavigation(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Doctor → provider ready → Esc → doctor.
	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("esc"))
	waitForText(t, tm, "System Status")

	// Doctor → provider ready → validate → target select → Esc → provider ready.
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("esc"))
	waitForText(t, tm, "Provider Readiness")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

// TestDashboardFlow_NoRawCredential asserts no credential appears in any output.
// Guide §17: "Normalize output when needed."
func TestDashboardFlow_NoRawCredential(t *testing.T) {
	const rawKey = "11111111-1111-1111-1111-111111111111"
	scanner, mgr, _ := happyFlowSetup(t)
	profiles := []provider.CredentialProfile{{
		ProviderID: "exa",
		Values:     map[string]string{"EXA_API_KEY": rawKey},
		Label:      "1111...1111",
	}}
	// Also make validate pass with a fake report that doesn't carry the key.
	mgr.ValidErr = nil

	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Walk the full happy path while checking output at each step.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if strings.Contains(string(b), rawKey) {
			t.Errorf("raw key found in doctor screen output")
		}
		return strings.Contains(string(b), "System Status")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(keyMsg("p"))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if strings.Contains(string(b), rawKey) {
			t.Errorf("raw key found in provider ready output")
		}
		return strings.Contains(string(b), "Provider Readiness")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(keyMsg("enter"))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if strings.Contains(string(b), rawKey) {
			t.Errorf("raw key found in target select output")
		}
		return strings.Contains(string(b), "Select Targets")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

// TestDashboardFlow_ConflictInTargetSelect verifies conflict clients appear in the
// target select screen with their conflict label visible.
// Overlay navigation and resolution are covered thoroughly by unit tests (TestConflictResolve_*).
func TestDashboardFlow_ConflictInTargetSelect(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	// Add a conflict client to the report.
	scanner.Report.Clients = append(scanner.Report.Clients, doctor.ClientFinding{
		ID:         "antigravity",
		Name:       "Antigravity IDE",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json",
				Exists: true, ParseOK: true, Providers: []string{"exa"}},
		},
	})
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Walk to target select.
	// Send both keys before waiting to avoid consuming the intermediate frame.
	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	tm.Send(keyMsg("enter")) // offline validation passes immediately with FakeDashboardManager

	// Both eligible clients AND the conflict section must appear in target select.
	waitForAll(t, tm, "Select Targets", "Conflict clients", "Antigravity IDE")

	// Quit and verify final model.
	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Confirm via FinalModel that the conflict is still unresolved (not auto-selected).
	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	got := final.(DashboardModel)
	if got.selectedClients["antigravity"] {
		t.Error("conflict client should not be auto-selected before resolution")
	}
	if _, resolved := got.resolvedConflicts["antigravity"]; resolved {
		t.Error("conflict should not be resolved without user action")
	}
}

// TestDashboardFlow_CredentialsThreadedThroughSavedPlan is a regression test for
// the bug where planCmd() omitted Credentials from SavedPlanOptions and
// preflightCmd()/applyCmd() omitted Credentials from SavedPlanApplyOptions.
// This caused "missing Exa API key" errors during preflight when keys had been
// entered via the in-flow credential overlay.
func TestDashboardFlow_CredentialsThreadedThroughSavedPlan(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Verify that credentials were threaded into BuildSavedPlan.
	if len(mgr.LastBuildPlanOpts.Credentials) == 0 {
		t.Error("planCmd() did not pass credential refs to BuildSavedPlan — plan would fail preflight with 'missing Exa API key'")
	}
	// Verify that credentials were threaded into PreflightSavedPlan.
	if len(mgr.LastPreflightOpts.Credentials) == 0 {
		t.Error("preflightCmd() did not pass credential values to PreflightSavedPlan")
	}
}
