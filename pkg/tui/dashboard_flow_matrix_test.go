package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func requireUXMatrix(t *testing.T) {
	t.Helper()
	if os.Getenv("USYNC_UX_MATRIX") != "1" {
		t.Skip("set USYNC_UX_MATRIX=1 to run UX bug-hunt matrix cases")
	}
}

// DM-P31 (updated): With the ConflictBlocked state removed, a no-key provider
// (e.g. Playwright) is auto-selected when there are conflicts and no credentials,
// so pressing [r] → target select → [Enter] reaches plan preview without a
// dead-end credential error. This replaces the old test which expected Exa to be
// the auto-selected provider and planning to be blocked.
func TestDashboardFlowMatrix_MissingCredentialsBlocksBeforePlan(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, _ := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, doctor.ClientFinding{
		ID:         "antigravity",
		Name:       "Antigravity IDE",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json", Exists: true, ParseOK: true},
		},
	})
	m := NewDashboardModel(scanner, mgr, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Select Targets")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	view := final.View()
	// The dead-end plan error must never surface regardless of credential state.
	if strings.Contains(view, "Plan error: at least one credential profile is required") {
		t.Fatalf("DM-P31: credential dead-end plan error still present\n\n%s", view)
	}
}

func TestDashboardFlowMatrix_CredentialDeadEndOffersRecovery(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, _ := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(96, 28))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	// Default cursor lands on Playwright (first no-key provider). Navigate UP to Exa (4 steps).
	// Provider order: Exa(0) GitHub(1) Context7(2) Tavily(3) Playwright(4) Kubernetes(5) Terraform(6)
	tm.Send(keyMsg("up"))
	tm.Send(keyMsg("up"))
	tm.Send(keyMsg("up"))
	tm.Send(keyMsg("up"))
	// [Enter] selects Exa, runs offline validation, then advances to target select.
	tm.Send(keyMsg("enter"))
	waitForAll(t, tm, "Select Targets", "Credentials needed - press [k] to add", "[k] add credentials")
	tm.Send(keyMsg("k"))
	waitForText(t, tm, "Add Credentials - Exa AI Search")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(validExaCredential), Paste: true})
	waitForText(t, tm, "[**** supplied]")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Save them to disk")
	tm.Send(keyMsg("n"))
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if mgr.PlanCalls != 1 {
		t.Fatalf("DM-P70: expected recovery flow to plan once, got %d", mgr.PlanCalls)
	}
	if len(mgr.LastProfiles) != 1 {
		t.Fatalf("DM-P70: expected one credential profile to reach planner, got %#v", mgr.LastProfiles)
	}
}

func TestDashboardFlowMatrix_DeselectOnlyTargetBlocksPlan(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg(" "))
	// New UI renders entries as "[ ] <scope-badge> Name" — check name and status message separately.
	waitForAll(t, tm, "Antigravity CLI", "Select at least one target")
	tm.Send(keyMsg("enter"))

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if mgr.PlanCalls != 0 {
		t.Fatalf("DM-P12: expected no plan call after deselecting only target, got %d; selected=%v\n\n%s",
			mgr.PlanCalls, mgr.LastSelected, final.View())
	}
	if final.screen != screenTargetSelect {
		t.Fatalf("DM-P12: expected to remain on target select, got screen %d\n\n%s", final.screen, final.View())
	}
}

func TestDashboardFlowMatrix_RWithoutConflictDoesNotAdvance(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if final.screen != screenProviderReady {
		t.Fatalf("DM-P10: hidden r advanced without conflicts; expected provider readiness, got screen %d\n\n%s",
			final.screen, final.View())
	}
}

func TestDashboardFlowMatrix_NoKeyProviderCanPlanWithoutCredentials(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, _ := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, nil)
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

	if mgr.PlanCalls != 1 {
		t.Fatalf("DM-P05: expected no-key provider to reach planning once, got %d", mgr.PlanCalls)
	}
}

func TestDashboardFlowMatrix_ConflictChosenPathReachesPlan(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	const chosenPath = "/home/.gemini/config/mcp_config.json"
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("j"))
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Resolve Conflict")
	tm.Send(keyMsg("1"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if got := mgr.LastTargetPath[config.AppAntigravity]; got != chosenPath {
		t.Fatalf("DM-P19A: expected chosen path %q to reach planning, got %q", chosenPath, got)
	}
}

func TestDashboardFlowMatrix_RFromTargetRowOpensConflict(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))
	waitForAll(t, tm, "Select Targets", "[r] resolve conflicts")
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Resolve Conflict")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if final.screen != screenConflictResolve {
		t.Fatalf("DM-P36: r from target row should open conflict resolver, got screen %d\n\n%s", final.screen, final.View())
	}
	if mgr.PlanCalls != 0 {
		t.Fatalf("DM-P36: resolving shortcut should not start planning, got %d plan calls", mgr.PlanCalls)
	}
}

func TestDashboardFlowMatrix_ConflictSecondPathReachesPlan(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	const chosenPath = "/home/.gemini/antigravity/mcp_config.json"
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("j"))
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Resolve Conflict")
	tm.Send(keyMsg("2"))
	waitForAll(t, tm, "Select Targets", "Antigravity IDE alternate")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if got := mgr.LastTargetPath[config.AppAntigravity]; got != chosenPath {
		t.Fatalf("DM-P19B: expected chosen path %q to reach planning, got %q", chosenPath, got)
	}
}

func TestDashboardFlowMatrix_SkippedConflictIsExcluded(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("j"))
	tm.Send(keyMsg("r"))
	waitForText(t, tm, "Resolve Conflict")
	tm.Send(keyMsg("s"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if mgr.LastSelected[config.AppAntigravity] {
		t.Fatalf("DM-P20: skipped conflict should not be selected for planning: %#v", mgr.LastSelected)
	}
	if _, exists := mgr.LastTargetPath[config.AppAntigravity]; exists {
		t.Fatalf("DM-P20: skipped conflict should not pass target path override: %#v", mgr.LastTargetPath)
	}
}

func TestDashboardFlowMatrix_WorkspaceToggleChangesTargets(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(96, 28))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("i"))
	waitForText(t, tm, "Cursor project")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if !targetFilesContainPath(mgr.LastTargetFile[config.AppCursor], "/repo/.cursor/mcp.json") {
		t.Fatalf("DM-P14: workspace toggle did not pass project target to plan: %#v", mgr.LastTargetFile[config.AppCursor])
	}
}

func TestDashboardFlowMatrix_WorkspaceOffExcludesProjectTargets(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(96, 28))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	files := mgr.LastTargetFile[config.AppCursor]
	if targetFilesContainPath(files, "/repo/.cursor/mcp.json") {
		t.Fatalf("DM-P32: workspace-off planning included project target: %#v", files)
	}
	if !targetFilesContainPath(files, "/home/.cursor/mcp.json") {
		t.Fatalf("DM-P32: workspace-off planning missed global target: %#v", files)
	}
}

func TestDashboardFlowMatrix_WorkspaceTargetShowsScopeWarning(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(96, 28))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg("i"))
	tm.Send(keyMsg("enter"))
	waitForAll(t, tm, "Plan Preview", "scope: project", "source control")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestDashboardFlowMatrix_MultiFileClientCanSelectOneCandidate(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := antigravityMultiFileSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(96, 28))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")
	tm.Send(keyMsg(" "))
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Plan Preview")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	files := mgr.LastTargetFile[config.AppAntigravityCLI]
	if targetFilesContainPath(files, "/home/.gemini/antigravity-cli/mcp_config.json") {
		t.Fatalf("DM-P34: deselected first target still reached planning: %#v", files)
	}
	if !targetFilesContainPath(files, "/home/.gemini/config/mcp_config.json") {
		t.Fatalf("DM-P34: selected second target did not reach planning: %#v", files)
	}
}

func matrixConflictClient() doctor.ClientFinding {
	return doctor.ClientFinding{
		ID:         "antigravity",
		Name:       "Antigravity IDE",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json", Exists: true, ParseOK: true},
			{Label: "alternate", Path: "/home/.gemini/antigravity/mcp_config.json", Exists: true, ParseOK: true},
		},
	}
}

func cursorWorkspaceSetup(t *testing.T) (*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile) {
	t.Helper()
	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = []doctor.ClientFinding{{
		ID:            manifest.ClientCursor,
		Name:          "Cursor",
		Confidence:    doctor.ConfidenceHigh,
		Installed:     true,
		EffectivePath: "/home/.cursor/mcp.json",
		Candidates: []doctor.CandidateFinding{
			{
				Label:    "global",
				Path:     "/home/.cursor/mcp.json",
				Scope:    manifest.ScopeGlobal,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			},
			{
				Label:    "project",
				Path:     "/repo/.cursor/mcp.json",
				Scope:    manifest.ScopeProject,
				ParseOK:  true,
				Writable: true,
			},
		},
	}}
	return scanner, mgr, profiles
}

func antigravityMultiFileSetup(t *testing.T) (*FakeScanner, *FakeDashboardManager, []provider.CredentialProfile) {
	t.Helper()
	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = []doctor.ClientFinding{{
		ID:            manifest.ClientAntigravityCLI,
		Name:          "Antigravity CLI",
		Confidence:    doctor.ConfidenceHigh,
		Installed:     true,
		EffectivePath: "/home/.gemini/antigravity-cli/mcp_config.json",
		Candidates: []doctor.CandidateFinding{
			{
				Label:    "mcp-config",
				Path:     "/home/.gemini/antigravity-cli/mcp_config.json",
				Scope:    manifest.ScopeUser,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			},
			{
				Label:    "legacy-gemini-config",
				Path:     "/home/.gemini/config/mcp_config.json",
				Scope:    manifest.ScopeLegacy,
				Exists:   true,
				ParseOK:  true,
				Writable: true,
			},
		},
	}}
	return scanner, mgr, profiles
}

func targetFilesContainPath(files []config.TargetFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func copyTargetSelections(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sameTargetSelections(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		if right[k] != v {
			return false
		}
	}
	return true
}

func entriesContainPath(entries []targetEntry, path string) bool {
	for _, entry := range entries {
		if entry.path == path {
			return true
		}
	}
	return false
}

// DM-P40 — providers must be selectable even when client conflicts exist.
// Previously ComputeReadiness marked every provider as conflict-blocked whenever
// any client had a path conflict, which hid all providers. Now providers always
// reflect their actual state (ready/missing-credentials/runtime-missing); client
// conflicts are resolved at the target-select level, not the provider level.
func TestDashboardFlowMatrix_ProvidersVisibleDespiteClientConflict(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 40))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	// Conflict banner should appear AND providers should still be listed.
	waitForAll(t, tm, "Provider Readiness", "Conflicts detected")

	// Press Enter — should trigger offline validation (provider is selectable).
	tm.Send(keyMsg("enter"))
	waitForText(t, tm, "Select Targets")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	view := final.View()

	// Validation must have run (provider was selectable).
	if mgr.ValidateCalls == 0 {
		t.Errorf("DM-P40: validation should run when provider is selected despite conflict, got 0 calls\n\n%s", view)
	}
}

func TestDashboardFlowMatrix_DoubleEnterDuringValidationIsSafe(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.screen = screenProviderReady
	m.readiness = []ProviderReadinessItem{{Meta: manifest.ProviderMeta{ID: "exa", Name: "Exa"}, State: ProviderStateReady}}

	next, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("DM-P42: first enter should start validation")
	}
	validating := next.(DashboardModel)
	if !validating.validating {
		t.Fatal("DM-P42: first enter did not mark validating=true")
	}
	if !strings.Contains(validating.View(), "Validating...") {
		t.Fatalf("DM-P42: validating state missing visible feedback\n\n%s", validating.View())
	}
	if strings.Contains(validating.View(), "[Enter] select") || strings.Contains(validating.View(), "[v] live validate") {
		t.Fatalf("DM-P42: footer still advertises validation keys while validating\n\n%s", validating.View())
	}

	next, cmd2 := validating.Update(keyMsg("enter"))
	if cmd2 != nil {
		t.Fatal("DM-P42: second enter should be ignored while validation is in flight")
	}

	msg := cmd()
	if _, ok := msg.(validationResultMsg); !ok {
		t.Fatalf("DM-P42: expected validationResultMsg, got %T", msg)
	}
	if mgr.ValidateCalls != 1 {
		t.Fatalf("DM-P42: expected exactly 1 validation call after double-enter, got %d", mgr.ValidateCalls)
	}
	if next.(DashboardModel).screen != screenProviderReady {
		t.Fatalf("DM-P42: second enter should not silently advance screens")
	}
}

func TestDashboardFlowMatrix_DoubleYDuringApplyIsSafe(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.screen = screenPlanPreview
	m.currentPlan = &mgr.Plan

	next, cmd := m.Update(keyMsg("y"))
	if cmd == nil {
		t.Fatal("DM-P43: first y should start apply")
	}
	applying := next.(DashboardModel)
	if !applying.applying {
		t.Fatal("DM-P43: first y did not mark applying=true")
	}
	if !strings.Contains(applying.View(), "Applying...") {
		t.Fatalf("DM-P43: applying state missing visible feedback\n\n%s", applying.View())
	}
	if strings.Contains(applying.View(), "[y]") || strings.Contains(applying.View(), "Press [y] to apply") {
		t.Fatalf("DM-P43: footer still advertises apply confirmation while apply is in flight\n\n%s", applying.View())
	}

	_, cmd2 := applying.Update(keyMsg("y"))
	if cmd2 != nil {
		t.Fatal("DM-P43: second y should be ignored while apply is in flight")
	}

	msg := cmd()
	if _, ok := msg.(dashApplyResultMsg); !ok {
		t.Fatalf("DM-P43: expected dashApplyResultMsg, got %T", msg)
	}
	if mgr.ApplyCalls != 1 {
		t.Fatalf("DM-P43: expected exactly 1 apply call after double-y, got %d", mgr.ApplyCalls)
	}
}

func TestDashboardFlowMatrix_DoubleRescanIsSafe(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.screen = screenApplyResult
	m.applyResult = &mgr.Result

	next, cmd := m.Update(keyMsg("r"))
	if cmd == nil {
		t.Fatal("DM-P44: first r should start rescanning")
	}
	rescanning := next.(DashboardModel)
	if !rescanning.scanning {
		t.Fatal("DM-P44: first r did not mark scanning=true")
	}
	if !strings.Contains(rescanning.View(), "Rescanning...") {
		t.Fatalf("DM-P44: rescanning state missing visible feedback\n\n%s", rescanning.View())
	}
	if strings.Contains(rescanning.View(), "[r] rescan") {
		t.Fatalf("DM-P44: footer still advertises rescan while scan is in flight\n\n%s", rescanning.View())
	}

	_, cmd2 := rescanning.Update(keyMsg("r"))
	if cmd2 != nil {
		t.Fatal("DM-P44: second r should be ignored while scan is in flight")
	}

	msg := cmd()
	if _, ok := msg.(scanResultMsg); !ok {
		t.Fatalf("DM-P44: expected scanResultMsg, got %T", msg)
	}
	if scanner.ScanCalls != 1 {
		t.Fatalf("DM-P44: expected exactly 1 rescan after double-r, got %d", scanner.ScanCalls)
	}
}

func TestDashboardFlowMatrix_EscFromProviderReadyKeepsReport(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("esc"))
	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if scanner.ScanCalls != 1 {
		t.Fatalf("DM-P46: esc from provider ready should not rescan, got %d scans", scanner.ScanCalls)
	}
	if final.screen != screenDoctor {
		t.Fatalf("DM-P46: expected Doctor screen after esc, got %d", final.screen)
	}
	if len(final.report.Clients) != len(scanner.Report.Clients) {
		t.Fatalf("DM-P46: expected report to be preserved, got %#v", final.report.Clients)
	}
}

func TestDashboardFlowMatrix_EscPreservesTargetSelections(t *testing.T) {
	scanner, mgr, profiles := antigravityMultiFileSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.report = scanner.Report
	m.screen = screenProviderReady
	m.readiness = []ProviderReadinessItem{{Meta: manifest.ProviderMeta{ID: "exa", Name: "Exa"}, State: ProviderStateReady}}

	next, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("DM-P47: provider enter should start validation")
	}
	validated, _ := next.(DashboardModel).Update(cmd())
	targetSelect := validated.(DashboardModel)
	if targetSelect.screen != screenTargetSelect {
		t.Fatalf("DM-P47: expected target select after validation, got %d", targetSelect.screen)
	}

	toggled, _ := targetSelect.Update(keyMsg(" "))
	afterToggle := toggled.(DashboardModel)
	beforeEsc := copyTargetSelections(afterToggle.selectedTargets)

	back, _ := afterToggle.Update(keyMsg("esc"))
	backToProvider := back.(DashboardModel)
	if backToProvider.screen != screenProviderReady {
		t.Fatalf("DM-P47: esc should return to provider ready, got %d", backToProvider.screen)
	}

	reopen, cmd := backToProvider.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("DM-P47: re-enter should start validation again")
	}
	revalidated, _ := reopen.(DashboardModel).Update(cmd())
	final := revalidated.(DashboardModel)

	if final.screen != screenTargetSelect {
		t.Fatalf("DM-P47: expected target select after re-entry, got %d", final.screen)
	}
	if !sameTargetSelections(beforeEsc, final.selectedTargets) {
		t.Fatalf("DM-P47: selectedTargets not preserved across esc/re-entry: before=%v after=%v", beforeEsc, final.selectedTargets)
	}
}

func TestDashboardFlowMatrix_EscFromPlanPreviewKeepsSelections(t *testing.T) {
	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.report = scanner.Report
	m.screen = screenProviderReady
	m.readiness = []ProviderReadinessItem{{Meta: manifest.ProviderMeta{ID: "exa", Name: "Exa"}, State: ProviderStateReady}}

	next, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("DM-P48: provider enter should start validation")
	}
	validated, _ := next.(DashboardModel).Update(cmd())
	targetSelect := validated.(DashboardModel)

	withWorkspace, _ := targetSelect.Update(keyMsg("i"))
	workspaceModel := withWorkspace.(DashboardModel)
	if !workspaceModel.includeWorkspace {
		t.Fatal("DM-P48: workspace toggle did not stay on")
	}

	toggled, _ := workspaceModel.Update(keyMsg(" "))
	afterToggle := toggled.(DashboardModel)
	beforeEsc := copyTargetSelections(afterToggle.selectedTargets)

	planned, cmd := afterToggle.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("DM-P48: enter should start plan creation")
	}
	afterPlan, preflightCmd := planned.(DashboardModel).Update(cmd())
	if preflightCmd == nil {
		t.Fatal("DM-P48: successful plan creation should start preflight")
	}
	preview, _ := afterPlan.(DashboardModel).Update(preflightCmd())
	planPreview := preview.(DashboardModel)
	if planPreview.screen != screenPlanPreview {
		t.Fatalf("DM-P48: expected plan preview, got %d", planPreview.screen)
	}

	back, _ := planPreview.Update(keyMsg("esc"))
	final := back.(DashboardModel)
	if final.screen != screenTargetSelect {
		t.Fatalf("DM-P48: esc should return to target select, got %d", final.screen)
	}
	if !final.includeWorkspace {
		t.Fatal("DM-P48: includeWorkspace was not preserved")
	}
	if !sameTargetSelections(beforeEsc, final.selectedTargets) {
		t.Fatalf("DM-P48: selectedTargets not preserved across plan preview esc: before=%v after=%v", beforeEsc, final.selectedTargets)
	}

	entries := allTargetEntries(final.report, final.resolvedConflicts, final.includeWorkspace)
	if !entriesContainPath(entries, "/repo/.cursor/mcp.json") {
		t.Fatalf("DM-P48: workspace target missing after esc: %#v", entries)
	}
}

func TestDashboardFlowMatrix_ResolvedConflictSurvivesRescan(t *testing.T) {
	scanner, mgr, profiles := happyFlowSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.screen = screenApplyResult
	m.includeWorkspace = true
	m.resolvedConflicts = map[manifest.ClientID]ConflictResolution{
		"antigravity": {
			ChosenPath:  "/home/.gemini/config/mcp_config.json",
			ChosenLabel: "repo-current",
		},
	}

	next, _ := m.Update(scanResultMsg{report: scanner.Report})
	updated := next.(DashboardModel)
	entries := allTargetEntries(updated.report, updated.resolvedConflicts, updated.includeWorkspace)

	for _, entry := range entries {
		if entry.clientID == "antigravity" {
			if entry.isConflict {
				t.Fatalf("DM-P49: resolved conflict reappeared as unresolved after rescan: %#v", entry)
			}
			if !updated.selectedTargets[entry.id] {
				t.Fatalf("DM-P49: resolved conflict target lost selection after rescan: %v", updated.selectedTargets)
			}
			return
		}
	}
	t.Fatalf("DM-P49: resolved conflict client missing after rescan: %#v", entries)
}

func TestDashboardFlowMatrix_RescanRebuildsTargetSelections(t *testing.T) {
	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	m.scanning = false
	m.screen = screenApplyResult
	m.includeWorkspace = false
	entries := allTargetEntries(scanner.Report, nil, false)
	m.selectedTargets = map[string]bool{}
	for _, entry := range entries {
		if entry.path == "/home/.cursor/mcp.json" {
			continue
		}
		m.selectedTargets[entry.id] = true
	}

	next, _ := m.Update(scanResultMsg{report: scanner.Report})
	updated := next.(DashboardModel)

	postEntries := allTargetEntries(updated.report, updated.resolvedConflicts, updated.includeWorkspace)
	expected := defaultSelectedTargets(updated.report, updated.resolvedConflicts, updated.includeWorkspace)
	if !sameTargetSelections(expected, updated.selectedTargets) {
		t.Fatalf("DM-P50: expected rescan to rebuild default selections, want=%v got=%v", expected, updated.selectedTargets)
	}
	for _, entry := range postEntries {
		if entry.path == "/home/.cursor/mcp.json" && !updated.selectedTargets[entry.id] {
			t.Fatalf("DM-P50: default global target should be re-selected after rescan: %v", updated.selectedTargets)
		}
	}
}

func TestDashboardFlowMatrix_BatchApplyThreeTargets(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := multiTargetSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)
	// 80×36: the redesigned UI uses more vertical chrome (heading subtitle, metrics bar)
	// so 24 lines is too small to show 3 targets simultaneously without scrolling.
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 36))

	waitForText(t, tm, "System Status")
	tm.Send(keyMsg("p"))
	waitForText(t, tm, "Provider Readiness")
	tm.Send(keyMsg("enter"))
	waitForAll(t, tm, "Select Targets", "Antigravity CLI", "Cursor", "Claude Code")
	tm.Send(keyMsg("enter"))
	waitForAll(t, tm, "Saved MCP plan", "Antigravity CLI", "cursor", "Claude Code")
	tm.Send(keyMsg("y"))
	waitForText(t, tm, "Apply Result")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if mgr.ApplyCalls != 1 {
		t.Fatalf("DM-P56: expected exactly 1 apply call, got %d", mgr.ApplyCalls)
	}
	if len(mgr.AppliedPlans) != 1 || len(mgr.AppliedPlans[0].Operations) != 3 {
		t.Fatalf("DM-P56: expected one applied plan with 3 operations, got %#v", mgr.AppliedPlans)
	}
	if final.applyResult == nil || len(final.applyResult.UpdatedTargets) != 3 {
		t.Fatalf("DM-P56: expected apply result with 3 updated targets, got %#v", final.applyResult)
	}
}

func TestDashboardFlowMatrix_SequentialProvidersInOneSession(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := multiTargetSetup(t)
	m := NewDashboardModel(scanner, mgr, profiles)

	next, _ := m.Update(scanResultMsg{report: scanner.Report})
	m = next.(DashboardModel)
	next, readyCmd := m.Update(keyMsg("p"))
	m = next.(DashboardModel)
	if readyCmd == nil {
		t.Fatal("DM-P58: expected readiness command for first provider pass")
	}
	next, _ = m.Update(readyCmd())
	m = next.(DashboardModel)
	next, validCmd := m.Update(keyMsg("enter"))
	m = next.(DashboardModel)
	if validCmd == nil {
		t.Fatal("DM-P58: expected validation command for first provider pass")
	}
	next, _ = m.Update(validCmd())
	m = next.(DashboardModel)
	next, planCmd := m.Update(keyMsg("enter"))
	m = next.(DashboardModel)
	if planCmd == nil {
		t.Fatal("DM-P58: expected plan command for first provider pass")
	}
	next, preflightCmd := m.Update(planCmd())
	m = next.(DashboardModel)
	if preflightCmd == nil {
		t.Fatal("DM-P58: expected preflight command for first provider pass")
	}
	next, _ = m.Update(preflightCmd())
	m = next.(DashboardModel)
	next, applyCmd := m.Update(keyMsg("y"))
	m = next.(DashboardModel)
	if applyCmd == nil {
		t.Fatal("DM-P58: expected apply command for first provider pass")
	}
	next, rescanCmd := m.Update(applyCmd())
	m = next.(DashboardModel)
	if rescanCmd == nil {
		t.Fatal("DM-P58: expected rescan command for first provider pass")
	}
	next, _ = m.Update(rescanCmd())
	m = next.(DashboardModel)

	// Resume from the same rescanned dashboard state and switch providers. This locks
	// provider cleanliness across consecutive applies on one dashboard model.
	m.screen = screenDoctor
	next, readyCmd = m.Update(keyMsg("p"))
	m = next.(DashboardModel)
	if readyCmd == nil {
		t.Fatal("DM-P58: expected readiness command for second provider pass")
	}
	next, _ = m.Update(readyCmd())
	m = next.(DashboardModel)
	context7Index := -1
	for i, item := range m.readiness {
		if item.Meta.ID == manifest.ProviderContext7 {
			context7Index = i
			break
		}
	}
	if context7Index < 0 {
		t.Fatalf("DM-P58: context7 provider not present in readiness list: %#v", m.readiness)
	}
	for m.providerCursor < context7Index {
		next, _ = m.Update(keyMsg("j"))
		m = next.(DashboardModel)
	}
	if m.providerCursor != context7Index {
		t.Fatalf("DM-P58: expected cursor on context7 index %d, got %d", context7Index, m.providerCursor)
	}
	next, validCmd = m.Update(keyMsg("enter"))
	m = next.(DashboardModel)
	if validCmd == nil {
		t.Fatal("DM-P58: expected validation command for second provider pass")
	}
	next, _ = m.Update(validCmd())
	m = next.(DashboardModel)
	next, planCmd = m.Update(keyMsg("enter"))
	m = next.(DashboardModel)
	if planCmd == nil {
		t.Fatal("DM-P58: expected plan command for second provider pass")
	}
	next, preflightCmd = m.Update(planCmd())
	m = next.(DashboardModel)
	if preflightCmd == nil {
		t.Fatal("DM-P58: expected preflight command for second provider pass")
	}
	next, _ = m.Update(preflightCmd())
	m = next.(DashboardModel)
	next, applyCmd = m.Update(keyMsg("y"))
	m = next.(DashboardModel)
	if applyCmd == nil {
		t.Fatal("DM-P58: expected apply command for second provider pass")
	}
	next, rescanCmd = m.Update(applyCmd())
	m = next.(DashboardModel)
	if rescanCmd == nil {
		t.Fatal("DM-P58: expected rescan command for second provider pass")
	}
	next, _ = m.Update(rescanCmd())
	m = next.(DashboardModel)

	if len(mgr.AppliedPlans) != 2 {
		t.Fatalf("DM-P58: expected 2 applied plans, got %d", len(mgr.AppliedPlans))
	}
	if mgr.AppliedPlans[0].ProviderID != "exa" || mgr.AppliedPlans[1].ProviderID != "context7" {
		t.Fatalf("DM-P58: expected provider sequence exa -> context7, got %q -> %q", mgr.AppliedPlans[0].ProviderID, mgr.AppliedPlans[1].ProviderID)
	}
	for _, op := range mgr.AppliedPlans[1].Operations {
		if op.ProviderID != "context7" {
			t.Fatalf("DM-P58: second plan leaked non-context7 operation: %#v", op)
		}
	}
}

// DM-P60: Single-candidate conflict overlay
func TestDashboardModel_SingleCandidateConflict(t *testing.T) {
	// DM-P60: Test that single-candidate conflict renders properly
	conflict := doctor.ClientFinding{
		ID:         "single",
		Name:       "Single",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "one", Path: "/home/one", Exists: true, ParseOK: true},
		},
	}
	candidates := conflictCandidatesForDisplay(conflict)
	if len(candidates) != 1 {
		t.Fatalf("DM-P60: expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Label != "one" {
		t.Fatalf("DM-P60: expected label 'one', got %q", candidates[0].Label)
	}
}

// DM-P61: Three-candidate conflict shows only first two
func TestDashboardModel_ThreeCandidateConflictShowsTwo(t *testing.T) {
	// DM-P61: Test that only first two candidates are displayed and usable
	conflict := doctor.ClientFinding{
		ID:         "three",
		Name:       "Three",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "first", Path: "/home/1", Exists: true, ParseOK: true},
			{Label: "second", Path: "/home/2", Exists: true, ParseOK: true},
			{Label: "third", Path: "/home/3", Exists: true, ParseOK: true},
		},
	}
	candidates := conflictCandidatesForDisplay(conflict)
	if len(candidates) != 2 {
		t.Fatalf("DM-P61: expected 2 candidates max, got %d", len(candidates))
	}
	if candidates[0].Label != "first" || candidates[1].Label != "second" {
		t.Fatalf("DM-P61: expected first two candidates, got %v", candidates)
	}
}

// DM-P63: Sequential conflicts - test resolution storage
func TestDashboardModel_SequentialConflictResolutions(t *testing.T) {
	// DM-P63: Test that multiple conflict resolutions are stored correctly
	scanner := &FakeScanner{}
	mgr := &FakeDashboardManager{}
	profiles := []provider.CredentialProfile{{ProviderID: "exa", Values: map[string]string{"EXA_API_KEY": "test"}}}

	conflict1 := doctor.ClientFinding{
		ID:         "conflict1",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "a", Path: "/home/c1/a", Exists: true, ParseOK: true},
			{Label: "b", Path: "/home/c1/b", Exists: true, ParseOK: true},
		},
	}
	conflict2 := doctor.ClientFinding{
		ID:         "conflict2",
		Confidence: doctor.ConfidenceConflict,
		Candidates: []doctor.CandidateFinding{
			{Label: "x", Path: "/home/c2/x", Exists: true, ParseOK: true},
			{Label: "y", Path: "/home/c2/y", Exists: true, ParseOK: true},
		},
	}
	scanner.Report.Clients = []doctor.ClientFinding{conflict1, conflict2}

	m := NewDashboardModel(scanner, mgr, profiles)
	// Manually store resolutions as the test would
	m.resolvedConflicts = map[manifest.ClientID]ConflictResolution{
		"conflict1": {ChosenPath: "/home/c1/a", ChosenLabel: "a"},
		"conflict2": {ChosenPath: "/home/c2/y", ChosenLabel: "y"},
	}

	if m.resolvedConflicts["conflict1"].ChosenPath != "/home/c1/a" {
		t.Fatalf("DM-P63: conflict1 resolution not stored correctly")
	}
	if m.resolvedConflicts["conflict2"].ChosenPath != "/home/c2/y" {
		t.Fatalf("DM-P63: conflict2 resolution not stored correctly")
	}
}

// DM-P64: Apply error offers recovery (rescan)
func TestDashboardFlowMatrix_ApplyErrorOffersRecovery(t *testing.T) {
	requireUXMatrix(t)

	scanner, mgr, profiles := happyFlowSetup(t)
	mgr.ApplyErr = errors.New("simulated apply failure")
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
	waitForAll(t, tm, "Apply Result", "simulated apply failure", "[r] rescan")

	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(DashboardModel)
	if final.applyErr == nil {
		t.Fatalf("DM-P64: applyErr should be set after apply failure")
	}
}

// DM-P65: Esc on ApplyResult is a no-op
func TestDashboardModel_EscOnApplyResult(t *testing.T) {
	// DM-P65: Test that Esc on ApplyResult is a no-op
	m := DashboardModel{screen: screenApplyResult}
	nextM, cmd := m.Update(keyMsg("esc"))
	nextModel := nextM.(DashboardModel)

	if nextModel.screen != screenApplyResult {
		t.Fatalf("DM-P65: Esc on ApplyResult should stay on ApplyResult, got screen %d", nextModel.screen)
	}
	if cmd != nil {
		t.Fatalf("DM-P65: Esc on ApplyResult should return no command, got %v", cmd)
	}
}

// DM-P66: Wizard route works even when scan error is present
func TestDashboardModel_WizardRouteOnError(t *testing.T) {
	// DM-P66: Test that 'w' works on Doctor screen even with error
	m := DashboardModel{screen: screenDoctor, err: errors.New("scan failed")}
	nextM, cmd := m.Update(keyMsg("w"))
	nextModel := nextM.(DashboardModel)

	if !nextModel.RouteToWizard {
		t.Fatalf("DM-P66: 'w' should set RouteToWizard even with error set")
	}
	if cmd == nil {
		t.Fatalf("DM-P66: 'w' should return tea.Quit command")
	}
}
