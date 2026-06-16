package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func TestSaveCredentialsProfiles_WritesFileWithPrivatePermissions(t *testing.T) {
	home := t.TempDir()
	profiles := []provider.CredentialProfile{{
		ProviderID: "exa",
		Label:      "1111...1111",
		Values:     map[string]string{"EXA_API_KEY": "abc-def"},
	}}
	path, err := SaveCredentialsProfiles(home, profiles, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("SaveCredentialsProfiles: %v", err)
	}
	expected := filepath.Join(home, ".config", "usync", "credentials.toml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}

	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if mode := parentInfo.Mode().Perm(); mode != 0o700 {
		t.Errorf("parent dir mode = %o, want 0700", mode)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"[[profiles]]",
		`provider = "exa"`,
		`label    = "1111...1111"`,
		"[profiles.values]",
		`EXA_API_KEY = "abc-def"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("credentials.toml missing %q\ncontent:\n%s", want, content)
		}
	}
}

func TestSaveCredentialsProfiles_DeterministicOrdering(t *testing.T) {
	home := t.TempDir()
	profiles := []provider.CredentialProfile{
		{ProviderID: "exa", Label: "b", Values: map[string]string{"K": "1"}},
		{ProviderID: "exa", Label: "a", Values: map[string]string{"K": "1"}},
	}
	now := time.Unix(0, 0).UTC()
	path, err := SaveCredentialsProfiles(home, profiles, now)
	if err != nil {
		t.Fatalf("SaveCredentialsProfiles: %v", err)
	}
	first, _ := os.ReadFile(path)
	if _, err := SaveCredentialsProfiles(home, profiles, now); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatalf("save output is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// DM-P76: pressing [s][y] from the save prompt writes credentials.toml under
// the manager's home directory with 0600 permissions and the submitted
// credential redacted only at display time (the saved file contains the raw
// secret since the file is the source-of-truth for future runs).
func TestCredentialEntry_SavePromptYWritesFileWithPermissions(t *testing.T) {
	m := credentialEntryModel(t)
	// Override manager home so the save target is the test tempdir.
	tmpHome := t.TempDir()
	fake := m.manager.(*FakeDashboardManager)
	fake.home = tmpHome

	withValue, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(validExaCredential), Paste: true})
	submitted, _ := withValue.(DashboardModel).Update(keyMsg("enter"))
	pending := submitted.(DashboardModel)
	if pending.credEntry == nil || !pending.credEntry.savePending {
		t.Fatal("expected save-prompt overlay after submit")
	}
	saved, cmd := pending.Update(keyMsg("y"))
	if cmd != nil {
		t.Fatal("save should not emit async command")
	}
	finalM := saved.(DashboardModel)
	if finalM.screen != screenTargetSelect {
		t.Fatalf("expected to return to target select after save, got %s", screenName(finalM.screen))
	}
	path := CredentialsFilePath(tmpHome)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("credentials.toml not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), validExaCredential) {
		t.Errorf("written file missing credential value:\n%s", data)
	}
}

func TestRenderCredentialsTOML_ProfileOrderingDeterministic(t *testing.T) {
	profiles := []provider.CredentialProfile{
		{ProviderID: "z-provider", Label: "x", Values: map[string]string{"B": "2", "A": "1"}},
		{ProviderID: "a-provider", Label: "y", Values: map[string]string{"K": "v"}},
	}
	out := renderCredentialsTOML(mergeCredentialProfiles(nil, profiles))
	aPos := strings.Index(out, `provider = "a-provider"`)
	zPos := strings.Index(out, `provider = "z-provider"`)
	if aPos < 0 || zPos < 0 || aPos > zPos {
		t.Fatalf("profiles not sorted alphabetically:\n%s", out)
	}
	bPos := strings.Index(out, "B = ")
	aValPos := strings.Index(out, "A = ")
	if aValPos < 0 || bPos < 0 || aValPos > bPos {
		t.Fatalf("value keys not sorted within block:\n%s", out)
	}
}
