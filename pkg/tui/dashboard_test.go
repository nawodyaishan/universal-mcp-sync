package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
)

// FakeScanner implements DashboardScanner for testing.
type FakeScanner struct {
	Report    doctor.Report
	Err       error
	ScanCalls int
}

func (s *FakeScanner) Scan(ctx context.Context) (doctor.Report, error) {
	s.ScanCalls++
	return s.Report, s.Err
}

func TestDashboardScanner_FakeScanner(t *testing.T) {
	ctx := context.Background()

	t.Run("returns injected report", func(t *testing.T) {
		expectedReport := doctor.Report{
			Platform: "test-platform",
			Warnings: []string{"test warning"},
		}
		scanner := &FakeScanner{Report: expectedReport}
		report, err := scanner.Scan(ctx)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if report.Platform != expectedReport.Platform {
			t.Errorf("expected platform %q, got %q", expectedReport.Platform, report.Platform)
		}
	})

	t.Run("returns injected error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		scanner := &FakeScanner{Err: expectedErr}
		_, err := scanner.Scan(ctx)

		if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
			t.Fatalf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestDashboardModel(t *testing.T) {
	scanner := &FakeScanner{
		Report: doctor.Report{Platform: "test"},
		Err:    nil,
	}

	model := NewDashboardModel(scanner, nil, nil)

	t.Run("initial state", func(t *testing.T) {
		if !model.scanning {
			t.Error("expected scanning to be true")
		}
		view := model.View()
		if !strings.Contains(view, "Scanning for AI") {
			t.Errorf("expected loading view, got %q", view)
		}
	})

	t.Run("init returns command", func(t *testing.T) {
		cmd := model.Init()
		if cmd == nil {
			t.Fatal("expected non-nil tea.Cmd")
		}
		msg := cmd()
		resultMsg, ok := msg.(scanResultMsg)
		if !ok {
			t.Fatalf("expected scanResultMsg, got %T", msg)
		}
		if resultMsg.report.Platform != "test" {
			t.Errorf("expected platform test, got %q", resultMsg.report.Platform)
		}
	})

	t.Run("update handles scan success", func(t *testing.T) {
		cmd := model.Init()
		msg := cmd()

		nextModel, cmd := model.Update(msg)
		updatedModel, ok := nextModel.(DashboardModel)
		if !ok {
			t.Fatal("expected DashboardModel")
		}

		if updatedModel.scanning {
			t.Error("expected scanning to be false")
		}
		if updatedModel.report.Platform != "test" {
			t.Errorf("expected platform test, got %q", updatedModel.report.Platform)
		}
		if cmd != nil {
			t.Error("expected nil command")
		}
	})

	t.Run("update handles scan error", func(t *testing.T) {
		errScanner := &FakeScanner{Err: errors.New("scan failed")}
		errModel := NewDashboardModel(errScanner, nil, nil)

		cmd := errModel.Init()
		msg := cmd()

		nextModel, _ := errModel.Update(msg)
		updatedModel := nextModel.(DashboardModel)

		if updatedModel.scanning {
			t.Error("expected scanning to be false")
		}
		if updatedModel.err == nil || updatedModel.err.Error() != "scan failed" {
			t.Errorf("expected scan failed error, got %v", updatedModel.err)
		}
		view := updatedModel.View()
		if !strings.Contains(view, "scan failed") {
			t.Errorf("expected error view, got %q", view)
		}
	})

	t.Run("quit keys", func(t *testing.T) {
		keys := []string{"q", "ctrl+c"}
		for _, key := range keys {
			keyMsg := makeKeyMsg(key)
			_, cmd := model.Update(keyMsg)
			if cmd == nil {
				t.Fatalf("expected tea.Quit for %q, got nil", key)
			}
		}
	})
}

func TestDashboardWelcomeChoice(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "test"}}
	model := NewDashboardModelWithWelcome(scanner, nil, nil)

	if cmd := model.Init(); cmd != nil {
		t.Fatal("welcome mode should not scan before a mode is selected")
	}
	view := model.View()
	if !strings.Contains(view, "Doctor Mode") || !strings.Contains(view, "Wizard Mode") {
		t.Fatalf("welcome view should explain both modes, got:\n%s", view)
	}

	next, cmd := model.Update(makeKeyMsg("enter"))
	started := next.(DashboardModel)
	if started.screen != screenDoctor || !started.scanning {
		t.Fatalf("enter should start doctor scan, got screen=%s scanning=%v", screenName(started.screen), started.scanning)
	}
	if cmd == nil {
		t.Fatal("doctor selection should return a scan command")
	}
	scanned, _ := started.Update(cmd())
	ready := scanned.(DashboardModel)
	if ready.scanning || ready.report.Platform != "test" {
		t.Fatalf("scan result not applied after welcome selection: %#v", ready.Snapshot())
	}

	model = NewDashboardModelWithWelcome(scanner, nil, nil)
	next, _ = model.Update(makeKeyMsg("down"))
	selectedWizard := next.(DashboardModel)
	next, cmd = selectedWizard.Update(makeKeyMsg("enter"))
	routed := next.(DashboardModel)
	if !routed.RouteToWizard {
		t.Fatal("wizard selection should set RouteToWizard")
	}
	if cmd == nil {
		t.Fatal("wizard selection should quit dashboard so main can launch wizard")
	}
}

func makeKeyMsg(s string) tea.KeyMsg {
	if s == "ctrl+c" {
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	if s == "up" {
		return tea.KeyMsg{Type: tea.KeyUp}
	}
	if s == "down" {
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestDashboardRedaction(t *testing.T) {
	scanner := &FakeScanner{
		Report: doctor.Report{
			Platform: "test",
			Warnings: []string{"test_sk_secretkey123"},
			Clients: []doctor.ClientFinding{
				{
					Name:          "TestClient",
					Installed:     true,
					Confidence:    doctor.ConfidenceHigh,
					EffectivePath: "/users/test/.config/test_sk_secretkey456.json",
					Issues:        []string{"found token test_sk_secretkey789"},
				},
			},
		},
	}
	model := NewDashboardModel(scanner, nil, nil)
	cmd := model.Init()
	msg := cmd()
	nextModel, _ := model.Update(msg)
	view := nextModel.View()

	if strings.Contains(view, "test_sk_secretkey") {
		t.Errorf("expected sensitive data to be redacted in view, got %q", view)
	}
}

// --- Phase 8 unit tests ---

// FakeDashboardManager implements DashboardManager for testing.
type FakeDashboardManager struct {
	Plan           app.SavedPlan
	Preflight      app.SavedPlanPreflight
	Result         app.ApplyResult
	ValidErr       error
	PlanErr        error
	ApplyErr       error
	home           string
	ValidateCalls  int
	PlanCalls      int
	PreflightCalls int
	ApplyCalls     int
	LastProfiles   []provider.CredentialProfile
	LastSelected   map[config.AppID]bool
	LastAssign     map[config.AppID]int
	LastTargetPath app.TargetPathOverrides
	LastTargetFile app.TargetFileOverrides
	AppliedPlans   []app.SavedPlan

	LastBuildPlanOpts app.SavedPlanOptions
	LastPreflightOpts app.SavedPlanApplyOptions
	LastApplyOpts     app.SavedPlanApplyOptions
}

func (f *FakeDashboardManager) PrepareProvider(prov provider.MCPProvider, profiles []provider.CredentialProfile,
	selected map[config.AppID]bool, assign map[config.AppID]int) (app.ExecutionPlan, error) {
	return f.PrepareProviderWithTargetPaths(prov, profiles, selected, assign, nil)
}

func (f *FakeDashboardManager) PrepareProviderWithTargetPaths(prov provider.MCPProvider, profiles []provider.CredentialProfile,
	selected map[config.AppID]bool, assign map[config.AppID]int, targetPaths app.TargetPathOverrides) (app.ExecutionPlan, error) {
	targetFiles := make(app.TargetFileOverrides)
	for id, path := range targetPaths {
		targetFiles[id] = []config.TargetFile{{Path: path}}
	}
	if len(targetFiles) == 0 {
		targetFiles = nil
	}
	return f.PrepareProviderWithTargetFiles(prov, profiles, selected, assign, targetFiles)
}

func (f *FakeDashboardManager) PrepareProviderWithTargetFiles(prov provider.MCPProvider, profiles []provider.CredentialProfile,
	selected map[config.AppID]bool, assign map[config.AppID]int, targetFiles app.TargetFileOverrides) (app.ExecutionPlan, error) {
	f.PlanCalls++
	f.LastProfiles = append([]provider.CredentialProfile(nil), profiles...)
	f.LastSelected = copyAppSelection(selected)
	f.LastAssign = copyAssignments(assign)
	f.LastTargetFile = copyTargetFileOverrides(targetFiles)
	f.LastTargetPath = firstTargetPaths(targetFiles)
	if len(profiles) == 0 && len(prov.RequiredCredentials()) > 0 {
		return app.ExecutionPlan{}, errors.New("at least one credential profile is required")
	}
	plan := app.ExecutionPlan{}
	for appID, files := range targetFiles {
		for _, file := range files {
			plan.Operations = append(plan.Operations, app.Operation{
				AppID:           appID,
				AppName:         config.AppName(appID),
				FileLabel:       file.Label,
				Path:            file.Path,
				Kind:            file.Kind,
				Scope:           file.Scope,
				GitWarning:      file.GitWarning,
				CredentialLabel: "test",
				ProviderID:      string(prov.ID()),
				WillCreate:      !file.Exists,
			})
		}
	}
	return plan, f.PlanErr
}
func (f *FakeDashboardManager) BuildSavedPlan(plan app.ExecutionPlan, opts app.SavedPlanOptions) (app.SavedPlan, error) {
	f.LastBuildPlanOpts = opts
	saved := f.Plan
	if opts.PlanID != "" {
		saved.PlanID = opts.PlanID
	}
	if !opts.CreatedAt.IsZero() {
		saved.CreatedAt = opts.CreatedAt
		if saved.ExpiresAt.IsZero() {
			saved.ExpiresAt = opts.CreatedAt.Add(24 * time.Hour)
		}
	}
	if opts.ProviderID != "" {
		saved.ProviderID = opts.ProviderID
	}
	if len(saved.Operations) == 0 && len(plan.Operations) > 0 {
		for _, op := range plan.Operations {
			action := app.PlanActionUpdate
			if op.WillCreate {
				action = app.PlanActionCreate
			}
			saved.Operations = append(saved.Operations, app.PlanOperation{
				TargetID:    string(op.AppID),
				TargetName:  op.AppName,
				TargetScope: op.Scope,
				Action:      action,
				ProviderID:  string(op.ProviderID),
				FileKind:    string(op.Kind),
				FilePath:    op.Path,
				Transport:   "http",
				Manager:     app.PlanManagerFile,
				Redacted:    op.AppName + ": test plan",
				WillCreate:  op.WillCreate,
				GitWarning:  op.GitWarning,
			})
		}
	}
	return saved, f.PlanErr
}
func (f *FakeDashboardManager) PreflightSavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.SavedPlanPreflight, error) {
	f.PreflightCalls++
	f.LastPreflightOpts = opts
	return f.Preflight, nil
}
func (f *FakeDashboardManager) ApplySavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.ApplyResult, error) {
	f.ApplyCalls++
	f.LastApplyOpts = opts
	f.AppliedPlans = append(f.AppliedPlans, plan)
	result := f.Result
	if len(result.UpdatedTargets) == 0 && len(result.SkippedTargets) == 0 &&
		len(result.BackupPaths) == 0 && len(result.Verification) == 0 &&
		len(result.Warnings) == 0 && len(result.RolledBack) == 0 &&
		len(result.RollbackFailed) == 0 {
		for _, op := range plan.Operations {
			switch {
			case op.FilePath != "":
				result.UpdatedTargets = append(result.UpdatedTargets, op.FilePath)
			case len(op.CLICommand) > 0:
				result.UpdatedTargets = append(result.UpdatedTargets, strings.Join(op.CLICommand, " "))
			}
		}
	}
	return result, f.ApplyErr
}
func (f *FakeDashboardManager) Validate(ctx context.Context, prov provider.MCPProvider, profiles []provider.CredentialProfile, live bool) (validate.Report, error) {
	f.ValidateCalls++
	if f.ValidErr != nil {
		return validate.Report{}, f.ValidErr
	}
	return validate.Report{Results: []validate.Result{{Status: validate.StatusOK}}}, nil
}
func (f *FakeDashboardManager) HomeDir() string { return f.home }

func copyAppSelection(in map[config.AppID]bool) map[config.AppID]bool {
	if in == nil {
		return nil
	}
	out := make(map[config.AppID]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyAssignments(in map[config.AppID]int) map[config.AppID]int {
	if in == nil {
		return nil
	}
	out := make(map[config.AppID]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyTargetFileOverrides(in app.TargetFileOverrides) app.TargetFileOverrides {
	if in == nil {
		return nil
	}
	out := make(app.TargetFileOverrides, len(in))
	for k, v := range in {
		out[k] = append([]config.TargetFile(nil), v...)
	}
	return out
}

func firstTargetPaths(in app.TargetFileOverrides) app.TargetPathOverrides {
	if len(in) == 0 {
		return nil
	}
	out := make(app.TargetPathOverrides)
	for appID, files := range in {
		if len(files) > 0 {
			out[appID] = files[0].Path
		}
	}
	return out
}

func newFakeManager(t *testing.T) *FakeDashboardManager {
	t.Helper()
	return &FakeDashboardManager{home: t.TempDir()}
}

// DM-P41 — RenderedProviderIndices returns all indices regardless of conflict state.
// ComputeReadiness no longer emits ConflictBlocked; conflicts are handled at the
// target-select level. This test confirms all items are renderable in both cases.
func TestRenderedProviderIndices_AllItemsRenderable(t *testing.T) {
	items := []ProviderReadinessItem{
		{State: ProviderStateReady},
		{State: ProviderStateMissingCredential},
		{State: ProviderStateRuntimeMissing},
		{State: ProviderStateNoKeyNeeded},
	}
	// With conflicts: all items are still renderable.
	got := RenderedProviderIndices(items, true)
	if len(got) != len(items) {
		t.Fatalf("with conflicts: want all %d indices renderable, got %v", len(items), got)
	}
	// Without conflicts: same.
	got = RenderedProviderIndices(items, false)
	if len(got) != len(items) {
		t.Fatalf("without conflicts: want all %d indices, got %v", len(items), got)
	}
}

// Phase 13 DM-P41 — cursor helpers skip across the rendered set.
func TestRenderedProviderIndices_CursorHelpersSkipHidden(t *testing.T) {
	rendered := []int{1, 3, 4}
	if got := nextRenderedIndex(rendered, 0); got != 1 {
		t.Errorf("next from 0: want 1 got %d", got)
	}
	if got := nextRenderedIndex(rendered, 1); got != 3 {
		t.Errorf("next from 1: want 3 got %d", got)
	}
	if got := nextRenderedIndex(rendered, 4); got != 4 {
		t.Errorf("next from last: want clamp 4 got %d", got)
	}
	if got := prevRenderedIndex(rendered, 4); got != 3 {
		t.Errorf("prev from 4: want 3 got %d", got)
	}
	if got := prevRenderedIndex(rendered, 3); got != 1 {
		t.Errorf("prev from 3: want 1 got %d", got)
	}
	if got := prevRenderedIndex(rendered, 1); got != 1 {
		t.Errorf("prev from first: want clamp 1 got %d", got)
	}
}

func TestComputeReadiness_AllFiveStates(t *testing.T) {
	allProviders := manifest.AllProviders()
	if len(allProviders) == 0 {
		t.Skip("no providers registered")
	}

	// no-key-needed: provider with no credentials (e.g. Playwright)
	noKeyProviders := []manifest.ProviderMeta{}
	for _, p := range allProviders {
		if len(p.Credentials) == 0 {
			noKeyProviders = append(noKeyProviders, p)
			break
		}
	}

	// ready: provider with credentials + matching profile
	readyProviders := []manifest.ProviderMeta{}
	readyProfiles := []provider.CredentialProfile{}
	for _, p := range allProviders {
		if len(p.Credentials) > 0 {
			readyProviders = append(readyProviders, p)
			readyProfiles = append(readyProfiles, provider.CredentialProfile{ProviderID: string(p.ID)})
			break
		}
	}

	// missing-credentials: provider with credentials but no profile
	missingProviders := []manifest.ProviderMeta{}
	for _, p := range allProviders {
		if len(p.Credentials) > 0 {
			missingProviders = append(missingProviders, p)
			break
		}
	}

	tests := []struct {
		name      string
		providers []manifest.ProviderMeta
		report    doctor.Report
		profiles  []provider.CredentialProfile
		wantState ProviderState
	}{
		{
			name:      "no-key-needed",
			providers: noKeyProviders,
			wantState: ProviderStateNoKeyNeeded,
		},
		{
			name:      "ready",
			providers: readyProviders,
			profiles:  readyProfiles,
			wantState: ProviderStateReady,
		},
		{
			name:      "missing-credentials",
			providers: missingProviders,
			wantState: ProviderStateMissingCredential,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.providers) == 0 {
				t.Skip("no provider for this state")
			}
			items := ComputeReadiness(tc.providers, tc.report, tc.profiles)
			if len(items) == 0 {
				t.Fatal("expected at least one readiness item")
			}
			if items[0].State != tc.wantState {
				t.Errorf("expected state %q, got %q", tc.wantState, items[0].State)
			}
		})
	}
}

func TestDashboardModel_PAdvancesToProviderReady(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "test"}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)

	// complete scan
	scanMsg := scanResultMsg{report: doctor.Report{Platform: "test"}}
	next, _ := model.Update(scanMsg)
	m := next.(DashboardModel)

	// press p
	keyMsg := makeKeyMsg("p")
	next2, cmd := m.Update(keyMsg)
	m2 := next2.(DashboardModel)

	if m2.screen != screenProviderReady {
		t.Errorf("expected screenProviderReady, got %d", m2.screen)
	}
	if cmd == nil {
		t.Error("expected readinessCmd returned")
	}
}

func TestDashboardModel_EscReturnsFromProviderReady(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)

	// force screen to screenProviderReady
	model.scanning = false
	model.screen = screenProviderReady

	next, _ := model.Update(makeKeyMsg("esc"))
	m := next.(DashboardModel)
	if m.screen != screenDoctor {
		t.Errorf("expected screenDoctor after esc, got %d", m.screen)
	}
}

func TestDashboardModel_LiveValidationDebounce(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)
	model.scanning = false
	model.screen = screenProviderReady
	model.readiness = []ProviderReadinessItem{{State: ProviderStateReady}}

	// first v starts validation
	next, cmd := model.Update(makeKeyMsg("v"))
	m := next.(DashboardModel)
	if !m.validating {
		t.Error("expected validating=true after first v")
	}
	if cmd == nil {
		t.Error("expected cmd from first v")
	}

	// second v while validating is a no-op
	next2, cmd2 := m.Update(makeKeyMsg("v"))
	m2 := next2.(DashboardModel)
	_ = m2
	if cmd2 != nil {
		t.Error("expected nil cmd from second v while validating")
	}
}

func TestDashboardModel_ValidationFailBlocksPlan(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)
	model.scanning = false
	model.screen = screenProviderReady
	model.validating = true

	failReport := validate.Report{Results: []validate.Result{{Status: validate.StatusFailed, Message: "bad key"}}}
	next, _ := model.Update(validationResultMsg{report: failReport, live: false, err: nil})
	m := next.(DashboardModel)

	if m.screen == screenTargetSelect {
		t.Error("expected screen to NOT advance to screenTargetSelect on validation failure")
	}
}

func TestDashboardModel_PlanToPreflightChain(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	planID, _ := app.NewPlanID()
	mgr := newFakeManager(t)
	mgr.Plan = app.SavedPlan{PlanID: planID, ProviderID: "exa"}
	model := NewDashboardModel(scanner, mgr, nil)
	model.scanning = false
	model.screen = screenTargetSelect
	model.planning = true

	next, cmd := model.Update(planCreatedMsg{plan: mgr.Plan, path: "/tmp/plan.json"})
	m := next.(DashboardModel)

	if m.planning {
		t.Error("expected planning=false after planCreatedMsg")
	}
	if !m.preflighting {
		t.Error("expected preflighting=true after planCreatedMsg success")
	}
	if cmd == nil {
		t.Error("expected preflightCmd returned from planCreatedMsg handler")
	}
}

func TestDashboardModel_PreflightResultAdvances(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)
	model.scanning = false
	model.preflighting = true

	next, _ := model.Update(preflightResultMsg{preflight: app.SavedPlanPreflight{}})
	m := next.(DashboardModel)

	if m.screen != screenPlanPreview {
		t.Errorf("expected screenPlanPreview, got %d", m.screen)
	}
	if m.preflighting {
		t.Error("expected preflighting=false after preflightResultMsg")
	}
}

func TestDashboardModel_PreflightErrorShowsError(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)
	model.preflighting = true

	next, _ := model.Update(preflightResultMsg{err: errors.New("preflight failed")})
	m := next.(DashboardModel)

	if m.screen == screenPlanPreview {
		t.Error("expected screen NOT to advance on preflight error")
	}
	if m.planErr == nil {
		t.Error("expected planErr set on preflight error")
	}
}

func TestDashboardModel_ApplyResultTriggersRescan(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{}}
	mgr := newFakeManager(t)
	model := NewDashboardModel(scanner, mgr, nil)
	model.applying = true

	next, cmd := model.Update(dashApplyResultMsg{result: app.ApplyResult{}})
	m := next.(DashboardModel)

	if m.screen != screenApplyResult {
		t.Errorf("expected screenApplyResult, got %d", m.screen)
	}
	if cmd == nil {
		t.Error("expected rescan cmd after apply result")
	}
}

func TestDashboardModel_NoRawCredentialInView(t *testing.T) {
	rawKey := "11111111-1111-1111-1111-111111111111"
	scanner := &FakeScanner{Report: doctor.Report{Platform: "test"}}
	mgr := newFakeManager(t)
	profiles := []provider.CredentialProfile{{ProviderID: "exa", Values: map[string]string{"EXA_API_KEY": rawKey}}}
	model := NewDashboardModel(scanner, mgr, profiles)
	model.scanning = false

	model.credEntry = &credentialEntryState{
		providerID:   "exa",
		providerName: "Exa AI Search",
		fields: []credentialField{{
			Spec:  provider.CredentialSpec{Key: "EXA_API_KEY", Label: "Exa API Key", Secret: true},
			Value: rawKey,
		}},
	}
	screens := []dashboardScreen{screenWelcome, screenDoctor, screenProviderReady, screenTargetSelect, screenCredentialEntry, screenPlanPreview, screenApplyResult}
	for _, s := range screens {
		model.screen = s
		v := model.View()
		if strings.Contains(v, rawKey) {
			t.Errorf("raw key found in View() for screen %d:\n%s", s, v)
		}
	}
}

func TestDashboardModel_NilManagerDisablesPhase8(t *testing.T) {
	scanner := &FakeScanner{Report: doctor.Report{Platform: "test"}}
	model := NewDashboardModel(scanner, nil, nil)
	model.scanning = false

	next, _ := model.Update(makeKeyMsg("p"))
	m := next.(DashboardModel)
	if m.screen != screenDoctor {
		t.Errorf("expected screenDoctor with nil manager, got %d", m.screen)
	}
}

func TestDashboardModel_RescanRebuildsSelectionsAndPreservesResolutions(t *testing.T) {
	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	scanner.Report.Clients = append(scanner.Report.Clients, matrixConflictClient())
	model := NewDashboardModel(scanner, mgr, profiles)
	model.scanning = false
	model.screen = screenApplyResult
	model.includeWorkspace = true
	model.providerCursor = 99
	model.clientCursor = 99
	model.selectedTargets = map[string]bool{"stale": true}
	model.resolvedConflicts = map[manifest.ClientID]ConflictResolution{
		"antigravity": {
			ChosenPath:  "/home/.gemini/config/mcp_config.json",
			ChosenLabel: "repo-current",
		},
	}

	next, _ := model.Update(scanResultMsg{report: scanner.Report})
	updated := next.(DashboardModel)

	if !updated.includeWorkspace {
		t.Fatal("expected includeWorkspace to survive rescan")
	}
	if _, ok := updated.resolvedConflicts["antigravity"]; !ok {
		t.Fatal("expected resolved conflict to survive rescan")
	}
	rendered := RenderedProviderIndices(updated.readiness, false)
	validCursor := false
	for _, idx := range rendered {
		if updated.providerCursor == idx {
			validCursor = true
			break
		}
	}
	if !validCursor && len(rendered) > 0 {
		t.Fatalf("expected providerCursor clamped to a valid rendered index, got %d (rendered: %v)", updated.providerCursor, rendered)
	}
	if updated.clientCursor >= len(allTargetEntries(updated.report, updated.resolvedConflicts, updated.includeWorkspace)) {
		t.Fatalf("expected clientCursor clamped into range, got %d", updated.clientCursor)
	}
	if updated.selectedTargets["stale"] {
		t.Fatalf("expected stale selection to be cleared on rescan, got %v", updated.selectedTargets)
	}

	entries := allTargetEntries(updated.report, updated.resolvedConflicts, updated.includeWorkspace)
	if !entriesContainPath(entries, "/repo/.cursor/mcp.json") {
		t.Fatalf("expected workspace target to remain visible after rescan: %#v", entries)
	}
	if !updated.selectedClients["antigravity"] {
		t.Fatalf("expected resolved conflict client to be re-selected after rescan: %#v", updated.selectedClients)
	}

	foundResolvedTarget := false
	for _, entry := range entries {
		if entry.clientID == "antigravity" && !entry.isConflict {
			foundResolvedTarget = updated.selectedTargets[entry.id]
		}
	}
	if !foundResolvedTarget {
		t.Fatalf("expected resolved conflict target to be present and selected after rescan: entries=%#v selected=%v", entries, updated.selectedTargets)
	}
}

func TestRenderDashboardHelpOverlay_ScreenAware(t *testing.T) {
	welcomeHelp := renderDashboardHelpOverlay(screenWelcome)
	providerHelp := renderDashboardHelpOverlay(screenProviderReady)
	targetHelp := renderDashboardHelpOverlay(screenTargetSelect)

	if !strings.Contains(welcomeHelp, "Help - Welcome") || !strings.Contains(welcomeHelp, "start wizard mode") {
		t.Fatalf("expected welcome help to describe mode selection, got:\n%s", welcomeHelp)
	}

	if !strings.Contains(providerHelp, "Help - Provider Readiness") {
		t.Fatalf("expected provider help title, got:\n%s", providerHelp)
	}
	if !strings.Contains(providerHelp, "v") || !strings.Contains(providerHelp, "run live validation") {
		t.Fatalf("expected provider help to include live validation, got:\n%s", providerHelp)
	}
	if strings.Contains(providerHelp, "toggle workspace targets") {
		t.Fatalf("provider help should not include target-select-only keys:\n%s", providerHelp)
	}

	if !strings.Contains(targetHelp, "Help - Select Targets") {
		t.Fatalf("expected target help title, got:\n%s", targetHelp)
	}
	if !strings.Contains(targetHelp, "space") || !strings.Contains(targetHelp, "toggle workspace targets") {
		t.Fatalf("expected target help to include selection/workspace keys, got:\n%s", targetHelp)
	}
	if strings.Contains(targetHelp, "run live validation") {
		t.Fatalf("target help should not include provider-only keys:\n%s", targetHelp)
	}
}

func TestDashboardModel_HelpOverlayTogglesWithoutLosingState(t *testing.T) {
	tests := []DashboardModel{
		func() DashboardModel {
			scanner := &FakeScanner{Report: doctor.Report{Platform: "test", Clients: []doctor.ClientFinding{{ID: "antigravity-cli", Name: "Antigravity CLI", Confidence: doctor.ConfidenceHigh, Installed: true}}}}
			m := NewDashboardModel(scanner, nil, nil)
			m.scanning = false
			m.report = scanner.Report
			m.screen = screenDoctor
			return m
		}(),
		func() DashboardModel {
			scanner, mgr, profiles := cursorWorkspaceSetup(t)
			m := NewDashboardModel(scanner, mgr, profiles)
			m.scanning = false
			m.report = scanner.Report
			m.screen = screenTargetSelect
			m.includeWorkspace = true
			m.selectedTargets = defaultSelectedTargets(m.report, nil, true)
			return m
		}(),
	}

	for _, model := range tests {
		before := model.View()
		next, _ := model.Update(makeKeyMsg("?"))
		withHelp := next.(DashboardModel)
		if !withHelp.showHelp {
			t.Fatal("expected help overlay to open")
		}
		if !strings.Contains(withHelp.View(), "Help -") {
			t.Fatalf("expected help overlay content, got:\n%s", withHelp.View())
		}

		back, _ := withHelp.Update(makeKeyMsg("?"))
		closed := back.(DashboardModel)
		if closed.showHelp {
			t.Fatal("expected help overlay to close")
		}
		if closed.View() != before {
			t.Fatalf("expected view to be restored after closing help\nbefore:\n%s\n\nafter:\n%s", before, closed.View())
		}
	}
}

func TestDashboardModel_UnmappedKeysNoOpAcrossScreens(t *testing.T) {
	keys := []tea.KeyMsg{
		makeKeyMsg("x"),
		makeKeyMsg("z"),
		makeKeyMsg("5"),
	}
	screens := []dashboardScreen{
		screenWelcome,
		screenDoctor,
		screenProviderReady,
		screenTargetSelect,
		screenConflictResolve,
		screenPlanPreview,
		screenApplyResult,
	}

	for _, screen := range screens {
		for _, key := range keys {
			t.Run(screenName(screen)+"/"+key.String(), func(t *testing.T) {
				model := dashboardModelForScreen(t, screen)
				before := dashboardSnapshot(model)

				next, cmd := model.Update(key)
				if cmd != nil {
					t.Fatalf("DM-P51: unmapped key %q on %s should not emit a command", key.String(), screenName(screen))
				}

				after := dashboardSnapshot(next.(DashboardModel))
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("DM-P51: unmapped key %q changed model on %s\nbefore: %#v\nafter:  %#v", key.String(), screenName(screen), before, after)
				}
			})
		}
	}
}

func TestDashboardModel_QuitKeysFromAnyScreen(t *testing.T) {
	keys := []tea.KeyMsg{
		makeKeyMsg("q"),
		makeKeyMsg("ctrl+c"),
	}
	screens := []dashboardScreen{
		screenWelcome,
		screenDoctor,
		screenProviderReady,
		screenTargetSelect,
		screenConflictResolve,
		screenCredentialEntry,
		screenPlanPreview,
		screenApplyResult,
	}

	for _, screen := range screens {
		for _, key := range keys {
			t.Run(screenName(screen)+"/"+key.String(), func(t *testing.T) {
				model := dashboardModelForScreen(t, screen)
				_, cmd := model.Update(key)
				if cmd == nil {
					t.Fatalf("expected quit command for %q on %s", key.String(), screenName(screen))
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatalf("DM-P52/53: expected tea.QuitMsg for %q on %s, got %T", key.String(), screenName(screen), cmd())
				}
			})
		}
	}
}

func TestDashboardModel_ConflictResolveUnmappedDigitsNoOp(t *testing.T) {
	keys := []tea.KeyMsg{
		makeKeyMsg("0"),
		makeKeyMsg("3"),
		makeKeyMsg("x"),
		{Type: tea.KeyF1},
	}

	for _, key := range keys {
		t.Run(key.String(), func(t *testing.T) {
			model := dashboardModelForScreen(t, screenConflictResolve)
			before := dashboardSnapshot(model)

			next, cmd := model.Update(key)
			if cmd != nil {
				t.Fatalf("DM-P62: unmapped key %q should not emit a command on conflict resolve", key.String())
			}
			afterModel := next.(DashboardModel)
			after := dashboardSnapshot(afterModel)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("DM-P62: unmapped key %q changed model\nbefore: %#v\nafter:  %#v", key.String(), before, after)
			}
			if afterModel.screen != screenConflictResolve {
				t.Fatalf("DM-P62: expected to remain on conflict resolve, got %d", afterModel.screen)
			}
		})
	}
}

// --- Phase 12: conflict resolution unit tests ---

func conflictReport() doctor.Report {
	return doctor.Report{
		Platform: "darwin",
		Clients: []doctor.ClientFinding{
			{
				ID: "antigravity", Name: "Antigravity IDE",
				Confidence: doctor.ConfidenceConflict,
				Candidates: []doctor.CandidateFinding{
					{Label: "repo-current", Path: "/home/.gemini/config/mcp_config.json",
						Exists: true, ParseOK: true, Providers: []string{"exa"}},
					{Label: "alternate-symlink", Path: "/home/.gemini/antigravity/mcp_config.json",
						Exists: true, IsSymlink: true, Resolved: "/home/.gemini/antigravity-data/mcp_config.json",
						ParseOK: true},
				},
			},
			{
				ID: "antigravity-cli", Name: "Antigravity CLI",
				Confidence: doctor.ConfidenceHigh, Installed: true,
				EffectivePath: "/home/.gemini/antigravity-cli/mcp_config.json",
			},
		},
	}
}

func TestAllTargetEntries_IncludesResolvedConflict(t *testing.T) {
	report := conflictReport()
	resolved := map[manifest.ClientID]ConflictResolution{
		"antigravity": {ChosenPath: "/home/.gemini/config/mcp_config.json", ChosenLabel: "repo-current"},
	}
	entries := allTargetEntries(report, resolved, false)
	for _, e := range entries {
		if e.clientID == "antigravity" && e.isConflict {
			t.Error("resolved conflict should not be isConflict=true")
		}
		if e.clientID == "antigravity" && !e.isConflict {
			return // found as eligible — pass
		}
	}
	t.Error("resolved conflict client not found in entries")
}

func TestAllTargetEntries_ExcludesSkippedConflict(t *testing.T) {
	report := conflictReport()
	resolved := map[manifest.ClientID]ConflictResolution{
		"antigravity": {Skipped: true},
	}
	entries := allTargetEntries(report, resolved, false)
	for _, e := range entries {
		if e.clientID == "antigravity" {
			t.Errorf("skipped conflict should not appear in entries, got %+v", e)
		}
	}
}

func TestConflictClient_CursorReachesConflict(t *testing.T) {
	report := conflictReport()
	scanner := &FakeScanner{Report: report}
	mgr := newFakeManager(t)
	m := NewDashboardModel(scanner, mgr, nil)
	m.scanning = false
	m.screen = screenTargetSelect
	m.report = report

	entries := allTargetEntries(report, m.resolvedConflicts, false)
	conflictIdx := -1
	for i, e := range entries {
		if e.isConflict {
			conflictIdx = i
			break
		}
	}
	if conflictIdx < 0 {
		t.Fatal("expected conflict entry in allTargetEntries")
	}
	m.clientCursor = conflictIdx
	if !entries[m.clientCursor].isConflict {
		t.Errorf("cursor should be on conflict entry")
	}
}

func TestConflictClient_ROpensOverlay(t *testing.T) {
	report := conflictReport()
	scanner := &FakeScanner{Report: report}
	mgr := newFakeManager(t)
	m := NewDashboardModel(scanner, mgr, nil)
	m.scanning = false
	m.screen = screenTargetSelect
	m.report = report

	// Position cursor on the conflict entry.
	entries := allTargetEntries(report, nil, false)
	for i, e := range entries {
		if e.isConflict {
			m.clientCursor = i
			break
		}
	}

	next, _ := m.Update(makeKeyMsg("r"))
	updated := next.(DashboardModel)
	if updated.screen != screenConflictResolve {
		t.Errorf("expected screenConflictResolve, got %d", updated.screen)
	}
	if updated.resolveTarget == nil {
		t.Error("expected resolveTarget set after pressing r on conflict")
	}
}

func setupConflictResolveScreen(t *testing.T) DashboardModel {
	t.Helper()
	report := conflictReport()
	scanner := &FakeScanner{Report: report}
	mgr := newFakeManager(t)
	m := NewDashboardModel(scanner, mgr, nil)
	m.scanning = false
	m.screen = screenConflictResolve
	m.report = report
	c := report.Clients[0] // antigravity conflict
	m.resolveTarget = &c
	return m
}

type dashboardStateSnapshot struct {
	screen             dashboardScreen
	scanning           bool
	showHelp           bool
	providerCursor     int
	selectedProv       int
	clientCursor       int
	includeWorkspace   bool
	validating         bool
	planning           bool
	preflighting       bool
	applying           bool
	routeToWizard      bool
	placeholderMsg     string
	selectedTargets    map[string]bool
	selectedClients    map[manifest.ClientID]bool
	resolvedConflicts  map[manifest.ClientID]ConflictResolution
	resolveTargetID    string
	currentPlanID      string
	applyUpdatedTarget []string
	planErr            string
	validErr           string
	applyErr           string
}

func dashboardSnapshot(m DashboardModel) dashboardStateSnapshot {
	s := dashboardStateSnapshot{
		screen:            m.screen,
		scanning:          m.scanning,
		showHelp:          m.showHelp,
		providerCursor:    m.providerCursor,
		selectedProv:      m.selectedProv,
		clientCursor:      m.clientCursor,
		includeWorkspace:  m.includeWorkspace,
		validating:        m.validating,
		planning:          m.planning,
		preflighting:      m.preflighting,
		applying:          m.applying,
		routeToWizard:     m.RouteToWizard,
		placeholderMsg:    m.placeholderMsg,
		selectedTargets:   copyStringBoolMap(m.selectedTargets),
		selectedClients:   copyClientSelection(m.selectedClients),
		resolvedConflicts: copyResolvedConflicts(m.resolvedConflicts),
	}
	if m.resolveTarget != nil {
		s.resolveTargetID = string(m.resolveTarget.ID)
	}
	if m.currentPlan != nil {
		s.currentPlanID = m.currentPlan.PlanID
	}
	if m.applyResult != nil {
		s.applyUpdatedTarget = append([]string(nil), m.applyResult.UpdatedTargets...)
	}
	if m.planErr != nil {
		s.planErr = m.planErr.Error()
	}
	if m.validErr != nil {
		s.validErr = m.validErr.Error()
	}
	if m.applyErr != nil {
		s.applyErr = m.applyErr.Error()
	}
	return s
}

func dashboardModelForScreen(t *testing.T, screen dashboardScreen) DashboardModel {
	t.Helper()

	scanner, mgr, profiles := cursorWorkspaceSetup(t)
	base := NewDashboardModel(scanner, mgr, profiles)
	base.scanning = false
	base.report = scanner.Report
	base.screen = screen
	base.readiness = []ProviderReadinessItem{{Meta: manifest.ProviderMeta{ID: "exa", Name: "Exa"}, State: ProviderStateReady}}
	base.currentPlan = &app.SavedPlan{PlanID: "test-plan", ProviderID: "exa"}
	base.planPreflight = &app.SavedPlanPreflight{PlanID: "test-plan", ProviderID: "exa"}
	base.applyResult = &app.ApplyResult{UpdatedTargets: []string{"/tmp/target.json"}}
	base.selectedTargets = defaultSelectedTargets(base.report, nil, true)
	base.selectedClients = deriveSelectedClients(allTargetEntries(base.report, nil, true), base.selectedTargets)
	base.includeWorkspace = true

	switch screen {
	case screenConflictResolve:
		return setupConflictResolveScreen(t)
	case screenProviderReady:
		base.providerCursor = 0
		base.selectedProv = 0
	case screenTargetSelect:
		base.clientCursor = 0
	case screenPlanPreview:
		base.currentPlan = &app.SavedPlan{PlanID: "test-plan", ProviderID: "exa"}
	case screenCredentialEntry:
		base.credReturnTo = screenTargetSelect
		base.credEntry = newCredentialEntryState(provider.NewExaProvider(), manifest.ProviderMeta{
			ID:   "exa",
			Name: "Exa AI Search",
			Credentials: []manifest.CredentialAcquisition{{
				Key:    "EXA_API_KEY",
				GetURL: "https://dashboard.exa.ai/api-keys",
			}},
		})
	case screenApplyResult:
		base.applyResult = &app.ApplyResult{UpdatedTargets: []string{"/tmp/target.json"}}
	}
	return base
}

func screenName(screen dashboardScreen) string {
	switch screen {
	case screenWelcome:
		return "welcome"
	case screenDoctor:
		return "doctor"
	case screenProviderReady:
		return "provider-ready"
	case screenTargetSelect:
		return "target-select"
	case screenPlanPreview:
		return "plan-preview"
	case screenApplyResult:
		return "apply-result"
	case screenConflictResolve:
		return "conflict-resolve"
	case screenCredentialEntry:
		return "credential-entry"
	default:
		return "unknown"
	}
}

func copyStringBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyClientSelection(in map[manifest.ClientID]bool) map[manifest.ClientID]bool {
	if in == nil {
		return nil
	}
	out := make(map[manifest.ClientID]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyResolvedConflicts(in map[manifest.ClientID]ConflictResolution) map[manifest.ClientID]ConflictResolution {
	if in == nil {
		return nil
	}
	out := make(map[manifest.ClientID]ConflictResolution, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestConflictResolve_1MovesToEligible(t *testing.T) {
	m := setupConflictResolveScreen(t)
	next, _ := m.Update(makeKeyMsg("1"))
	updated := next.(DashboardModel)

	r, ok := updated.resolvedConflicts["antigravity"]
	if !ok {
		t.Fatal("expected resolution stored for 'antigravity'")
	}
	if r.Skipped {
		t.Error("expected Skipped=false")
	}
	if r.ChosenLabel != "repo-current" {
		t.Errorf("expected ChosenLabel=repo-current, got %q", r.ChosenLabel)
	}
	if !updated.selectedClients["antigravity"] {
		t.Error("expected resolved client auto-selected")
	}
	if updated.screen != screenTargetSelect {
		t.Errorf("expected screenTargetSelect after resolution, got %d", updated.screen)
	}
	if updated.resolveTarget != nil {
		t.Error("expected resolveTarget cleared after resolution")
	}
}

func TestConflictResolve_2UsesSecondCandidate(t *testing.T) {
	m := setupConflictResolveScreen(t)
	next, _ := m.Update(makeKeyMsg("2"))
	updated := next.(DashboardModel)

	r, ok := updated.resolvedConflicts["antigravity"]
	if !ok {
		t.Fatal("expected resolution stored")
	}
	if r.ChosenLabel != "alternate-symlink" {
		t.Errorf("expected ChosenLabel=alternate-symlink, got %q", r.ChosenLabel)
	}
}

func TestConflictResolve_SSkipsClient(t *testing.T) {
	m := setupConflictResolveScreen(t)
	next, _ := m.Update(makeKeyMsg("s"))
	updated := next.(DashboardModel)

	r, ok := updated.resolvedConflicts["antigravity"]
	if !ok {
		t.Fatal("expected resolution stored")
	}
	if !r.Skipped {
		t.Error("expected Skipped=true after pressing s")
	}
	if updated.selectedClients["antigravity"] {
		t.Error("expected skipped client NOT in selectedClients")
	}
	if updated.screen != screenTargetSelect {
		t.Errorf("expected screenTargetSelect, got %d", updated.screen)
	}
}

func TestConflictResolve_EscCancels(t *testing.T) {
	m := setupConflictResolveScreen(t)
	before := len(m.resolvedConflicts)
	next, _ := m.Update(makeKeyMsg("esc"))
	updated := next.(DashboardModel)

	if len(updated.resolvedConflicts) != before {
		t.Error("Esc should not change resolvedConflicts")
	}
	if updated.screen != screenTargetSelect {
		t.Errorf("expected screenTargetSelect after Esc, got %d", updated.screen)
	}
	if updated.resolveTarget != nil {
		t.Error("expected resolveTarget cleared after Esc")
	}
}
