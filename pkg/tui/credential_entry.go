package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
)

type credentialEntryState struct {
	providerID    string
	providerName  string
	fields        []credentialField
	cursor        int
	submitErr     error
	savePending   bool
	saveErr       error
	saveSucceeded bool
	savedPath     string
	pendingProfs  []provider.CredentialProfile
}

type credentialField struct {
	Spec    provider.CredentialSpec
	HelpURL string
	Value   string
}

func newCredentialEntryState(prov provider.MCPProvider, meta manifest.ProviderMeta) *credentialEntryState {
	fields := make([]credentialField, 0, len(prov.RequiredCredentials()))
	for _, spec := range prov.RequiredCredentials() {
		fields = append(fields, credentialField{
			Spec:    spec,
			HelpURL: credentialHelpURLFromMeta(meta, spec.Key),
		})
	}
	return &credentialEntryState{
		providerID:   prov.ID(),
		providerName: prov.Name(),
		fields:       fields,
	}
}

func credentialHelpURLFromMeta(meta manifest.ProviderMeta, key string) string {
	for _, cred := range meta.Credentials {
		if cred.Key == key {
			if cred.GetURL != "" {
				return cred.GetURL
			}
			return cred.DocsURL
		}
	}
	return meta.DocsURL
}

func (s *credentialEntryState) activeField() *credentialField {
	if s == nil || len(s.fields) == 0 || s.cursor < 0 || s.cursor >= len(s.fields) {
		return nil
	}
	return &s.fields[s.cursor]
}

func (s *credentialEntryState) moveNext() {
	if s == nil || len(s.fields) == 0 {
		return
	}
	s.cursor = (s.cursor + 1) % len(s.fields)
}

func (s *credentialEntryState) movePrev() {
	if s == nil || len(s.fields) == 0 {
		return
	}
	s.cursor = (s.cursor - 1 + len(s.fields)) % len(s.fields)
}

func (s *credentialEntryState) appendText(text string) {
	field := s.activeField()
	if field == nil {
		return
	}
	field.Value += text
	s.submitErr = nil
}

func (s *credentialEntryState) backspace() {
	field := s.activeField()
	if field == nil || field.Value == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(field.Value)
	if size <= 0 {
		field.Value = ""
		return
	}
	field.Value = field.Value[:len(field.Value)-size]
	s.submitErr = nil
}

func (m DashboardModel) handleKeyCredentialEntry(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.credEntry == nil {
		m.screen = m.credReturnTo
		return m, nil
	}
	if m.credEntry.savePending {
		return m.dispatchCredentialSavePrompt(msg)
	}
	if msg.Paste {
		m.credEntry.appendText(string(msg.Runes))
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.credEntry = nil
		m.screen = m.credReturnTo
	case "tab":
		m.credEntry.moveNext()
	case "shift+tab":
		m.credEntry.movePrev()
	case "enter":
		return m.submitCredentialEntry()
	case "backspace":
		m.credEntry.backspace()
	default:
		if msg.Type == tea.KeyRunes {
			m.credEntry.appendText(string(msg.Runes))
		}
	}
	return m, nil
}

// dispatchCredentialSavePrompt handles the post-submit "save credentials to
// disk?" overlay. Driven by credEntry.savePending; not a screen handler so
// the AST keymap audit ignores it.
func (m DashboardModel) dispatchCredentialSavePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m.commitCredentialSave()
	case "n", "N", "esc":
		return m.finishCredentialEntry()
	}
	return m, nil
}

func (m DashboardModel) commitCredentialSave() (tea.Model, tea.Cmd) {
	if m.credEntry == nil {
		return m, nil
	}
	var home string
	if m.manager != nil {
		home = m.manager.HomeDir()
	}
	if home == "" || home == "." {
		m.credEntry.saveErr = fmt.Errorf("home directory is unknown")
		return m, nil
	}
	path, err := SaveCredentialsProfiles(home, m.credEntry.pendingProfs, time.Now())
	if err != nil {
		m.credEntry.saveErr = err
		return m, nil
	}
	m.credEntry.saveErr = nil
	m.credEntry.saveSucceeded = true
	m.credEntry.savedPath = path
	return m.finishCredentialEntry()
}

func (m DashboardModel) finishCredentialEntry() (tea.Model, tea.Cmd) {
	m.credEntry = nil
	m.screen = m.credReturnTo
	return m, nil
}

func (m DashboardModel) submitCredentialEntry() (tea.Model, tea.Cmd) {
	if m.credEntry == nil {
		return m, nil
	}
	prov, ok := provider.DefaultRegistry().Get(m.credEntry.providerID)
	if !ok {
		m.credEntry.submitErr = fmt.Errorf("provider %q is unavailable", m.credEntry.providerID)
		return m, nil
	}

	values := make(map[string]string, len(m.credEntry.fields))
	for _, field := range m.credEntry.fields {
		values[field.Spec.Key] = strings.TrimSpace(field.Value)
	}
	if err := validate.MissingRequiredCredentials(prov, values); err != nil {
		m.credEntry.submitErr = err
		return m, nil
	}

	profiles, err := credentialProfilesFromValues(prov, values)
	if err != nil {
		m.credEntry.submitErr = err
		return m, nil
	}
	if len(profiles) == 0 {
		m.credEntry.submitErr = fmt.Errorf("at least one credential profile is required")
		return m, nil
	}

	m.profiles = append(m.profiles, profiles...)
	m.readiness = ComputeReadiness(manifest.AllProviders(), m.report, m.profiles)
	m.credEntry.pendingProfs = profiles
	m.credEntry.savePending = true
	m.credEntry.submitErr = nil
	return m, nil
}

func credentialProfilesFromValues(prov provider.MCPProvider, values map[string]string) ([]provider.CredentialProfile, error) {
	if parser, ok := prov.(provider.MultiValueParser); ok {
		var profiles []provider.CredentialProfile
		for _, spec := range prov.RequiredCredentials() {
			if !spec.MultiValue {
				continue
			}
			parsed, err := parser.ParseMultiValue(spec.Key, values[spec.Key])
			if err != nil {
				return nil, err
			}
			profiles = append(profiles, parsed...)
		}
		if len(profiles) > 0 {
			return profiles, nil
		}
	}

	label := "interactive"
	for _, spec := range prov.RequiredCredentials() {
		if spec.Secret && strings.TrimSpace(values[spec.Key]) != "" {
			label = validate.RedactedCredentialLabel(prov.ID(), spec.Key, values[spec.Key])
			break
		}
	}
	return []provider.CredentialProfile{{
		ProviderID: prov.ID(),
		Values:     cloneCredentialValues(values),
		Label:      label,
	}}, nil
}

func cloneCredentialValues(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
