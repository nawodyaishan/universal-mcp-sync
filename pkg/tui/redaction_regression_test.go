package tui

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/audit"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
)

var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	regexp.MustCompile(`ctx7sk[-_][A-Za-z0-9]{8,}`),
	regexp.MustCompile(`tvly-[A-Za-z0-9]{8,}`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
}

func assertNoRawCredential(t *testing.T, subject string) {
	t.Helper()
	for _, re := range credentialPatterns {
		if loc := re.FindString(subject); loc != "" {
			t.Errorf("raw credential %q found in output", loc)
		}
	}
}

const fakeUUID = "11111111-1111-1111-1111-111111111111"

func TestRedactionRegression(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

	t.Run("FormatSavedPlan", func(t *testing.T) {
		planID, _ := app.NewPlanID()
		plan := app.SavedPlan{
			PlanID:     planID,
			ProviderID: "exa",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			Operations: []app.PlanOperation{{
				TargetName:    "Antigravity CLI",
				Action:        app.PlanActionUpdate,
				ProviderID:    "exa",
				Redacted:      "Antigravity CLI: update exa [http, credential=1111...1111]",
				CredentialRef: "ref-1",
			}},
			Credentials: []app.CredentialRef{{
				Key:   "EXA_API_KEY",
				Label: "1111...1111",
			}},
		}
		assertNoRawCredential(t, app.FormatSavedPlan(plan, now))
	})

	t.Run("FormatApplyResult", func(t *testing.T) {
		result := app.ApplyResult{
			UpdatedTargets: []string{"/home/.gemini/antigravity-cli/mcp_config.json"},
			Warnings:       []string{"credential=" + fakeUUID[:8] + "..."},
		}
		assertNoRawCredential(t, app.FormatApplyResult(result))
	})

	t.Run("FormatPlan", func(t *testing.T) {
		plan := app.ExecutionPlan{
			Warnings: []string{"skipped credential " + fakeUUID[:8] + "..."},
		}
		assertNoRawCredential(t, app.FormatPlan(plan))
	})

	t.Run("AuditEntryJSON", func(t *testing.T) {
		entry := audit.Entry{
			Timestamp: now,
			Command:   "apply",
			Targets:   []string{"exa"},
			ExitCode:  0,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertNoRawCredential(t, string(data))
	})

	t.Run("DashboardView_AllScreens", func(t *testing.T) {
		profiles := []provider.CredentialProfile{{
			ProviderID: "exa",
			Values:     map[string]string{"EXA_API_KEY": fakeUUID},
			Label:      "1111...1111",
		}}
		scanner := &FakeScanner{Report: doctor.Report{Platform: "test"}}
		m := NewDashboardModel(scanner, nil, profiles)
		m.scanning = false
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m = next.(DashboardModel)

		for _, screen := range []dashboardScreen{
			screenWelcome, screenDoctor, screenProviderReady, screenTargetSelect,
			screenPlanPreview, screenApplyResult,
		} {
			m.screen = screen
			assertNoRawCredential(t, m.View())
		}
	})

	t.Run("DoctorFormatReport", func(t *testing.T) {
		report := doctor.Report{
			Platform: "darwin",
			Clients: []doctor.ClientFinding{{
				ID:                  "antigravity-cli",
				Name:                "Antigravity CLI",
				Confidence:          doctor.ConfidenceHigh,
				Installed:           true,
				ConfiguredProviders: []string{"exa"},
			}},
		}
		assertNoRawCredential(t, doctor.FormatReport(report))
	})

	t.Run("ValidateFormatReport", func(t *testing.T) {
		report := validate.Report{
			Results: []validate.Result{{
				Status:  validate.StatusOK,
				Mode:    validate.ModeOffline,
				Message: "key format ok",
			}},
		}
		assertNoRawCredential(t, validate.FormatReport(report))
	})
}

// FakeValidationService implements a validate.Service-like interface for testing.
type FakeValidationService struct{}

func (FakeValidationService) Validate(ctx context.Context, prov provider.MCPProvider, profiles []provider.CredentialProfile, live bool) (validate.Report, error) {
	return validate.Report{Results: []validate.Result{{Status: validate.StatusOK}}}, nil
}
