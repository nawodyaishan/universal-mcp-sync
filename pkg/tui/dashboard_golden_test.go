package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
)

// injectWidth returns the model after injecting a fixed terminal width.
func injectWidth(m DashboardModel) DashboardModel {
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(DashboardModel)
}

func TestGoldenScreenDoctor(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{
		Platform: "darwin",
		Clients: []doctor.ClientFinding{
			{ID: "antigravity-cli", Name: "Antigravity CLI", Confidence: doctor.ConfidenceHigh, Installed: true},
		},
	}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenScreenProviderReady(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenProviderReady
	m.readiness = ComputeReadiness(nil, doctor.Report{}, nil)
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenScreenTargetSelect(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenTargetSelect
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenScreenPlanPreview(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	plan := app.SavedPlan{PlanID: "test-plan-id-golden", ProviderID: "exa"}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenPlanPreview
	m.currentPlan = &plan
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenScreenConflictResolve(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	c := doctor.ClientFinding{
		ID:   "antigravity",
		Name: "Antigravity IDE",
		Candidates: []doctor.CandidateFinding{
			{Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json",
				Exists: true, ParseOK: true, Providers: []string{"exa"}},
			{Label: "alternate-symlink", Path: "/home/.gemini/antigravity/mcp_config.json",
				Exists: true, IsSymlink: true, Resolved: "/home/.gemini/antigravity-data/mcp_config.json",
				ParseOK: true},
		},
	}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenConflictResolve
	m.resolveTarget = &c
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenScreenApplyResult(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	result := app.ApplyResult{UpdatedTargets: []string{"/home/.gemini/antigravity-cli/mcp_config.json"}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenApplyResult
	m.applyResult = &result
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenHelpOverlayProviderReady(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenProviderReady
	m.showHelp = true
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenHelpOverlayTargetSelect(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenTargetSelect
	m.showHelp = true
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}

func TestGoldenPlanPreview_FiveTargets80Cols(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "darwin"}}
	plan := app.SavedPlan{
		PlanID:     "test-plan-five-targets",
		ProviderID: "exa",
		Operations: []app.PlanOperation{
			{TargetName: "Claude Code", Action: app.PlanActionUpdate, FilePath: "/home/.claude.json", TargetScope: "user", Redacted: "Claude Code: update exa [cli, credential=test]"},
			{TargetName: "Cursor", Action: app.PlanActionUpdate, FilePath: "/home/.cursor/mcp.json", TargetScope: "global", Redacted: "Cursor: update exa [http, credential=test]"},
			{TargetName: "VS Code", Action: app.PlanActionUpdate, FilePath: "/home/.config/Code/User/mcp.json", TargetScope: "user", Redacted: "VS Code: update exa [http, credential=test]"},
			{TargetName: "Antigravity CLI", Action: app.PlanActionUpdate, FilePath: "/home/.gemini/antigravity-cli/mcp_config.json", TargetScope: "user", Redacted: "Antigravity CLI: update exa [http, credential=test]"},
			{TargetName: "Codex CLI", Action: app.PlanActionCreate, FilePath: "/home/.codex/config.toml", TargetScope: "user", Redacted: "Codex CLI: create exa [http, credential=test]"},
		},
	}
	m := NewDashboardModel(scanner, nil, nil)
	m.scanning = false
	m.screen = screenPlanPreview
	m.currentPlan = &plan
	m = injectWidth(m)
	golden.RequireEqual(t, []byte(m.View()))
}
