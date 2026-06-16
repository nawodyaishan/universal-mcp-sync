package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

const validExaCredential = "11111111-1111-1111-1111-111111111111"

func TestCredentialEntry_EnterValidatesRequired(t *testing.T) {
	m := credentialEntryModel(t)

	next, cmd := m.Update(keyMsg("enter"))
	if cmd != nil {
		t.Fatal("expected no command when credential validation fails")
	}
	got := next.(DashboardModel)
	if got.screen != screenCredentialEntry {
		t.Fatalf("expected credential entry to remain open, got %s", screenName(got.screen))
	}
	if got.credEntry == nil || got.credEntry.submitErr == nil {
		t.Fatal("expected submit error for missing credential")
	}
	if len(got.profiles) != 0 {
		t.Fatalf("expected profiles unchanged, got %#v", got.profiles)
	}
}

func TestCredentialEntry_KFromProviderReadyOpensOverlay(t *testing.T) {
	scanner, mgr, _ := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, nil)
	m.scanning = false
	m.report = scanner.Report
	m.screen = screenProviderReady
	m.readiness = []ProviderReadinessItem{{
		Meta: manifest.ProviderMeta{
			ID:   "exa",
			Name: "Exa AI Search",
			Credentials: []manifest.CredentialAcquisition{{
				Key:    "EXA_API_KEY",
				GetURL: "https://dashboard.exa.ai/api-keys",
			}},
		},
		State: ProviderStateMissingCredential,
	}}

	next, cmd := m.Update(keyMsg("k"))
	if cmd != nil {
		t.Fatal("opening credential entry should not emit command")
	}
	opened := next.(DashboardModel)
	if opened.screen != screenCredentialEntry {
		t.Fatalf("expected credential entry screen, got %s\n\n%s", screenName(opened.screen), opened.View())
	}
	if opened.credReturnTo != screenProviderReady {
		t.Fatalf("expected return screen provider ready, got %s", screenName(opened.credReturnTo))
	}
}

func TestCredentialEntry_SubmitAddsProfileAndMasksView(t *testing.T) {
	m := credentialEntryModel(t)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(validExaCredential), Paste: true})
	withValue := next.(DashboardModel)
	view := withValue.View()
	if strings.Contains(view, validExaCredential) {
		t.Fatalf("credential entry rendered raw credential:\n%s", view)
	}
	if !strings.Contains(view, "[**** supplied]") {
		t.Fatalf("credential entry did not render masked supplied state:\n%s", view)
	}

	next, cmd := withValue.Update(keyMsg("enter"))
	if cmd != nil {
		t.Fatal("credential submit should not emit async command in the in-memory slice")
	}
	pending := next.(DashboardModel)
	if pending.credEntry == nil || !pending.credEntry.savePending {
		t.Fatal("expected save-prompt overlay after submit")
	}
	if pending.screen != screenCredentialEntry {
		t.Fatalf("save prompt must remain on credential entry, got %s", screenName(pending.screen))
	}
	// Decline save-to-disk; verify model returns to caller screen with the
	// in-memory profile intact.
	declined, cmd := pending.Update(keyMsg("n"))
	if cmd != nil {
		t.Fatal("decline save should not emit command")
	}
	submitted := declined.(DashboardModel)
	if submitted.screen != screenTargetSelect {
		t.Fatalf("expected return to target select, got %s", screenName(submitted.screen))
	}
	if submitted.credEntry != nil {
		t.Fatal("expected credential entry state cleared after declining save")
	}
	if len(submitted.profiles) != 1 {
		t.Fatalf("expected one parsed profile, got %#v", submitted.profiles)
	}
	if submitted.profiles[0].Values["EXA_API_KEY"] != validExaCredential {
		t.Fatalf("profile did not retain submitted credential value: %#v", submitted.profiles[0])
	}
	if submitted.readiness[0].State != ProviderStateReady {
		t.Fatalf("expected readiness recomputed to ready, got %s", submitted.readiness[0].State)
	}
}

func TestCredentialEntry_EscRestoresPriorScreenUnchanged(t *testing.T) {
	m := credentialEntryModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(validExaCredential), Paste: true})
	withValue := next.(DashboardModel)

	next, cmd := withValue.Update(keyMsg("esc"))
	if cmd != nil {
		t.Fatal("esc should not emit command")
	}
	cancelled := next.(DashboardModel)
	if cancelled.screen != screenTargetSelect {
		t.Fatalf("expected target select after cancel, got %s", screenName(cancelled.screen))
	}
	if cancelled.credEntry != nil {
		t.Fatal("expected credential entry state cleared after cancel")
	}
	if len(cancelled.profiles) != 0 {
		t.Fatalf("expected profiles unchanged after cancel, got %#v", cancelled.profiles)
	}
}

func TestCredentialEntry_TabCyclesFields(t *testing.T) {
	state := &credentialEntryState{
		fields: []credentialField{
			{Spec: provider.CredentialSpec{Key: "ONE"}},
			{Spec: provider.CredentialSpec{Key: "TWO"}},
		},
	}
	state.moveNext()
	if state.cursor != 1 {
		t.Fatalf("tab: expected cursor 1, got %d", state.cursor)
	}
	state.moveNext()
	if state.cursor != 0 {
		t.Fatalf("tab wrap: expected cursor 0, got %d", state.cursor)
	}
	state.movePrev()
	if state.cursor != 1 {
		t.Fatalf("shift-tab wrap: expected cursor 1, got %d", state.cursor)
	}
}

func credentialEntryModel(t *testing.T) DashboardModel {
	t.Helper()
	scanner, mgr, _ := happyFlowSetup(t)
	m := NewDashboardModel(scanner, mgr, nil)
	m.scanning = false
	m.report = scanner.Report
	m.screen = screenTargetSelect
	m.readiness = []ProviderReadinessItem{{
		Meta: manifest.ProviderMeta{
			ID:   "exa",
			Name: "Exa AI Search",
			Credentials: []manifest.CredentialAcquisition{{
				Key:    "EXA_API_KEY",
				GetURL: "https://dashboard.exa.ai/api-keys",
			}},
		},
		State: ProviderStateMissingCredential,
	}}
	m.providerCursor = 0
	m.selectedProv = 0
	m.selectedTargets = defaultSelectedTargets(m.report, nil, false)
	m.selectedClients = deriveSelectedClients(allTargetEntries(m.report, nil, false), m.selectedTargets)

	next, cmd := m.Update(keyMsg("k"))
	if cmd != nil {
		t.Fatal("opening credential entry should not emit command")
	}
	opened := next.(DashboardModel)
	if opened.screen != screenCredentialEntry {
		t.Fatalf("expected credential entry screen, got %s\n\n%s", screenName(opened.screen), opened.View())
	}
	return opened
}
