package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

// DM-P69: Footer recovery after plan error — when planErr is set, don't advertise [Enter] plan
func TestRenderTargetSelect_FooterRecoveryAfterPlanError(t *testing.T) {
	now := time.Now().UTC()
	planID, _ := app.NewPlanID()
	mgr := &FakeDashboardManager{
		Plan: app.SavedPlan{
			SchemaVersion: 2,
			PlanID:        planID,
			ProviderID:    "exa",
			CreatedAt:     now,
			ExpiresAt:     now.Add(24 * time.Hour),
		},
		home: t.TempDir(),
	}
	scanner := &FakeScanner{}
	scanner.Report.Clients = []doctor.ClientFinding{{
		ID:            "test-client",
		Name:          "Test Client",
		Confidence:    doctor.ConfidenceHigh,
		Installed:     true,
		EffectivePath: "/home/.test/mcp.json",
	}}
	profiles := []provider.CredentialProfile{{
		ProviderID: "exa",
		Values:     map[string]string{"EXA_API_KEY": "test-key"},
		Label:      "test",
	}}

	m := NewDashboardModel(scanner, mgr, profiles)
	// Prepare the model state for Target Select with plan error
	m.screen = screenTargetSelect
	m.planErr = errors.New("provider unreachable")

	view := m.View()

	// DM-P69: Footer should NOT advertise [Enter] plan when planErr is set
	if strings.Contains(view, "[Enter] plan") {
		t.Fatalf("DM-P69: footer should not advertise [Enter] plan when planErr is set:\n%s", view)
	}

	// But should still show navigation and back options
	if !strings.Contains(view, "[Esc] back") {
		t.Fatalf("DM-P69: footer should advertise [Esc] back for recovery:\n%s", view)
	}

	// And should show the error
	if !strings.Contains(view, "Plan error") {
		t.Fatalf("DM-P69: should display plan error:\n%s", view)
	}
}

// Test that footer is correct when there's no plan error
func TestRenderTargetSelect_FooterNormalState(t *testing.T) {
	// DM-P69: Simpler test - just verify footer logic without full model setup
	// When planErr is nil and targets would be selectable, [Enter] plan should show
	hasErrors := false
	canPlan := true
	footer := actionBarTargetSelect(true, false, false, canPlan, hasErrors, false, false)
	if !strings.Contains(footer, "[Enter] plan") {
		t.Fatalf("DM-P69: footer should show [Enter] plan when canPlan=true and no error:\n%s", footer)
	}
}

func TestFooterGuidanceRows(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "provider missing credentials",
			got:  actionBarProviderReady(false, true, false, true),
			want: guidanceFor(screenProviderReady, footerMissingCredentials),
		},
		{
			name: "target missing credentials",
			got:  actionBarTargetSelect(false, false, false, false, false, true, false),
			want: guidanceFor(screenTargetSelect, footerMissingCredentials),
		},
		{
			name: "target no targets",
			got:  actionBarTargetSelect(false, false, false, false, false, false, true),
			want: guidanceFor(screenTargetSelect, footerNoTargets),
		},
		{
			name: "target plan error",
			got:  actionBarTargetSelect(false, false, false, false, true, false, false),
			want: guidanceFor(screenTargetSelect, footerPlanError),
		},
		{
			name: "doctor scan error",
			got:  actionBarDoctor(true, true),
			want: guidanceFor(screenDoctor, footerScanError),
		},
		{
			name: "doctor manager missing",
			got:  actionBarDoctor(false, false),
			want: guidanceFor(screenDoctor, footerManagerMissing),
		},
		{
			name: "plan preview applying",
			got: func() string {
				m := NewDashboardModel(&FakeScanner{}, &FakeDashboardManager{home: t.TempDir()}, nil)
				m.screen = screenPlanPreview
				m.applying = true
				return m.renderPlanPreview()
			}(),
			want: guidanceFor(screenPlanPreview, footerApplying),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.got, tc.want) {
				t.Fatalf("expected guidance %q in:\n%s", tc.want, tc.got)
			}
		})
	}
}
