package tui

import (
	"fmt"
	"strings"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

func (m DashboardModel) renderCredentialEntry() string {
	var b strings.Builder
	entry := m.credEntry
	if entry == nil {
		b.WriteString("Add Credentials\n\nNo credential entry is active.\n")
		b.WriteString("[Esc] back  [?] help  [q] quit")
		return b.String()
	}

	if entry.savePending {
		return renderCredentialSavePrompt(entry)
	}

	fmt.Fprintf(&b, "Add Credentials - %s\n", entry.providerName)
	b.WriteString(strings.Repeat("=", 20+len(entry.providerName)) + "\n\n")
	fmt.Fprintf(&b, "%s requires the following credential", entry.providerName)
	if len(entry.fields) == 1 {
		b.WriteString(":\n\n")
	} else {
		b.WriteString("s:\n\n")
	}

	for i, field := range entry.fields {
		cursor := "  "
		if i == entry.cursor {
			cursor = "> "
		}
		label := field.Spec.Label
		if label == "" {
			label = field.Spec.Key
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, label)
		if field.HelpURL != "" {
			fmt.Fprintf(&b, "  Get key: %s\n", field.HelpURL)
		}
		if field.Value == "" {
			b.WriteString("  [                                              ]\n")
		} else {
			b.WriteString("  [**** supplied]\n")
		}
		b.WriteString("\n")
	}

	if entry.submitErr != nil {
		b.WriteString("Error: " + redact.Text(entry.submitErr.Error()) + "\n\n")
	}

	b.WriteString("[Enter] submit  [Tab] next field  [Esc] cancel  [?] help  [q] quit")
	return b.String()
}

func renderCredentialSavePrompt(entry *credentialEntryState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Add Credentials - %s\n", entry.providerName)
	b.WriteString(strings.Repeat("=", 20+len(entry.providerName)) + "\n\n")
	b.WriteString("Credentials accepted. Save them to disk for future runs?\n\n")
	b.WriteString("  Path: ~/.config/usync/credentials.toml\n")
	b.WriteString("  Permissions: 0600 (private)\n\n")
	if entry.saveErr != nil {
		fmt.Fprintf(&b, "Save failed: %s\n\n", redact.Text(entry.saveErr.Error()))
	}
	b.WriteString("[y] save  [n] skip  [Esc] skip  [?] help  [q] quit")
	return b.String()
}
