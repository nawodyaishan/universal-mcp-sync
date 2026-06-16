package tui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
)

type fakeProdDashboardManager struct {
	*app.Manager
	validator validate.Service
}

func (m fakeProdDashboardManager) Validate(ctx context.Context, prov provider.MCPProvider, profiles []provider.CredentialProfile, live bool) (validate.Report, error) {
	return m.validator.ValidateProfiles(ctx, prov, profiles, live)
}

func (m fakeProdDashboardManager) HomeDir() string {
	return m.Manager.HomeDir
}

func requireUXFakeProd(t *testing.T) (string, string) {
	t.Helper()
	if os.Getenv("USYNC_UX_FAKE_PROD") != "1" {
		t.Skip("set USYNC_UX_FAKE_PROD=1 inside the fake-prod Docker harness")
	}
	home := os.Getenv("USYNC_UX_HOME")
	workspace := os.Getenv("USYNC_UX_WORKSPACE")
	if home == "" || workspace == "" {
		t.Fatal("USYNC_UX_HOME and USYNC_UX_WORKSPACE are required")
	}
	return home, workspace
}

func newFakeProdDashboard(t *testing.T, profiles []provider.CredentialProfile) DashboardModel {
	t.Helper()
	home, workspace := requireUXFakeProd(t)
	manager, err := app.NewManager(home, nil, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	validator, err := validate.NewService(manager.HomeDir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	scanner := NewProductionScanner(home, workspace)
	return NewDashboardModel(scanner, fakeProdDashboardManager{Manager: manager, validator: validator}, profiles)
}

func TestDashboardFakeProdMatrix_MissingCredentialsBlocksBeforePlan(t *testing.T) {
	m := newFakeProdDashboard(t, nil)

	scan := m.Init()
	next, _ := m.Update(scan())
	m = next.(DashboardModel)

	next, readyCmd := m.Update(keyMsg("p"))
	m = next.(DashboardModel)
	if readyCmd == nil {
		t.Fatalf("DM-P31/docker: expected provider readiness command\n\n%s", m.View())
	}
	next, _ = m.Update(readyCmd())
	m = next.(DashboardModel)

	next, _ = m.Update(keyMsg("r"))
	m = next.(DashboardModel)
	if m.screen != screenTargetSelect {
		t.Fatalf("DM-P31/docker: expected target select after conflict route, got screen %d\n\n%s", m.screen, m.View())
	}

	next, planCmd := m.Update(keyMsg("enter"))
	m = next.(DashboardModel)
	if planCmd != nil {
		next, _ = m.Update(planCmd())
		m = next.(DashboardModel)
	}

	view := m.View()
	if strings.Contains(view, "Plan error: at least one credential profile is required") {
		t.Fatalf("DM-P31/docker: missing credentials reached a dead-end plan error\n\n%s", view)
	}
	if strings.Contains(view, "[Enter] plan") {
		t.Fatalf("DM-P31/docker: footer still advertises plan while credentials are missing\n\n%s", view)
	}
}

func TestDashboardFakeProdMatrix_BatchSkipOnIdentical(t *testing.T) {
	const key = "11111111-1111-1111-1111-111111111111"

	dashboard := newFakeProdDashboard(t, nil)
	scan := dashboard.Init()
	next, _ := dashboard.Update(scan())
	dashboard = next.(DashboardModel)
	next, readyCmd := dashboard.Update(keyMsg("p"))
	dashboard = next.(DashboardModel)
	if readyCmd == nil {
		t.Fatalf("DM-P57/docker: expected provider readiness command\n\n%s", dashboard.View())
	}
	next, _ = dashboard.Update(readyCmd())
	dashboard = next.(DashboardModel)
	next, _ = dashboard.Update(keyMsg("enter"))
	dashboard = next.(DashboardModel)
	if dashboard.screen != screenTargetSelect {
		t.Fatalf("DM-P57/docker: expected target select after provider step, got screen %d\n\n%s", dashboard.screen, dashboard.View())
	}

	_ = dashboard.buildAppSelection()
	targetFiles := dashboard.selectedTargetFiles()
	selected := map[config.AppID]bool{
		config.AppAntigravityCLI: true,
	}
	targetFiles = app.TargetFileOverrides{
		config.AppAntigravityCLI: targetFiles[config.AppAntigravityCLI],
	}

	home, _ := requireUXFakeProd(t)
	manager, err := app.NewManager(home, nil, nil)
	if err != nil {
		t.Fatalf("DM-P57/docker: NewManager: %v", err)
	}
	prov := provider.NewExaProvider()
	profiles, err := prov.ParseMultiValue("EXA_API_KEY", key)
	if err != nil {
		t.Fatalf("DM-P57/docker: ParseMultiValue: %v", err)
	}
	assignments := app.DefaultAssignments(selected, len(profiles))
	credentialRefs := make([]app.CredentialRef, 0, len(profiles))
	for _, profile := range profiles {
		credentialRefs = append(credentialRefs, app.CredentialRef{
			Key:    "EXA_API_KEY",
			Label:  profile.Label,
			EnvVar: "EXA_API_KEY",
		})
	}

	runApply := func(planID string, createdAt time.Time) app.ApplyResult {
		t.Helper()
		executionPlan, err := manager.PrepareProviderWithTargetFiles(prov, profiles, selected, assignments, targetFiles)
		if err != nil {
			t.Fatalf("DM-P57/docker: PrepareProviderWithTargetFiles: %v", err)
		}
		savedPlan, err := manager.BuildSavedPlan(executionPlan, app.SavedPlanOptions{
			PlanID:            planID,
			CreatedAt:         createdAt,
			UsyncVersion:      "test",
			ProviderID:        string(prov.ID()),
			Credentials:       credentialRefs,
			UseInputVariables: true,
		})
		if err != nil {
			t.Fatalf("DM-P57/docker: BuildSavedPlan: %v", err)
		}
		result, err := manager.ApplySavedPlan(savedPlan, app.SavedPlanApplyOptions{
			AutoApprove: true,
			Credentials: map[string]string{"EXA_API_KEY": key},
		})
		if err != nil {
			t.Fatalf("DM-P57/docker: ApplySavedPlan: %v", err)
		}
		return result
	}

	first := runApply("dm-p57-first", time.Now().UTC())
	if len(first.UpdatedTargets) == 0 {
		t.Fatalf("DM-P57/docker: first apply should update at least one target, got %#v", first)
	}

	second := runApply("dm-p57-second", time.Now().UTC().Add(time.Second))
	if len(second.UpdatedTargets) != 0 {
		t.Fatalf("DM-P57/docker: second apply should not rewrite targets, got updated=%#v", second.UpdatedTargets)
	}
	if len(second.SkippedTargets) == 0 {
		t.Fatalf("DM-P57/docker: second apply should report unchanged targets, got %#v", second)
	}

	if !strings.Contains(app.FormatApplyResult(second), "Unchanged (") {
		t.Fatalf("DM-P57/docker: apply result missing Unchanged section\n\n%s", app.FormatApplyResult(second))
	}
}
