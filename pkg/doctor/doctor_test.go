package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanFindsProvidersAndDoesNotWrite(t *testing.T) {
	homeDir := t.TempDir()
	claudePath := filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("{\n  \"mcpServers\": {\n    \"context7\": {\"url\": \"https://context7.example/mcp\"}\n  }\n}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	codexPath := filepath.Join(homeDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(codexPath, []byte("model = \"gpt-5\"\n\n[mcp_servers.exa]\nurl = \"https://mcp.exa.ai/mcp?exaApiKey=old\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	before, err := fileSet(homeDir)
	if err != nil {
		t.Fatalf("fileSet before returned error: %v", err)
	}

	doctor, err := New(Options{
		HomeDir:       homeDir,
		GOOS:          "darwin",
		CheckRuntimes: false,
		Now: func() time.Time {
			return time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	doctor.lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	report, err := doctor.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if report.Platform != "darwin" {
		t.Fatalf("unexpected platform %q", report.Platform)
	}

	after, err := fileSet(homeDir)
	if err != nil {
		t.Fatalf("fileSet after returned error: %v", err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("scan wrote files:\nbefore=%v\nafter=%v", before, after)
	}

	claude := findClientFinding(t, report, "claude-desktop")
	if len(claude.ConfiguredProviders) != 1 || claude.ConfiguredProviders[0] != "context7" {
		t.Fatalf("unexpected Claude providers: %#v", claude.ConfiguredProviders)
	}

	codex := findClientFinding(t, report, "codex-cli")
	if len(codex.ConfiguredProviders) != 1 || codex.ConfiguredProviders[0] != "exa" {
		t.Fatalf("unexpected Codex providers: %#v", codex.ConfiguredProviders)
	}
}

func TestScanEmptyHome(t *testing.T) {
	homeDir := t.TempDir()

	doctor, err := New(Options{
		HomeDir:       homeDir,
		GOOS:          "darwin",
		CheckRuntimes: false,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	doctor.lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	report, err := doctor.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if report.HasFindings() {
		t.Fatalf("expected empty home to be clean, got %#v", report)
	}
}

func TestScanDetectsConflictAndSymlink(t *testing.T) {
	homeDir := t.TempDir()

	repoCurrent := filepath.Join(homeDir, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(repoCurrent), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(repoCurrent, []byte("{\"mcpServers\":{\"exa\":{\"url\":\"https://mcp.exa.ai/mcp\"}}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	alternateTarget := filepath.Join(homeDir, ".gemini", "antigravity-data", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(alternateTarget), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(alternateTarget, []byte("{\"mcpServers\":{\"context7\":{\"url\":\"https://context7.example/mcp\"}}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	alternateLink := filepath.Join(homeDir, ".gemini", "antigravity", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(alternateLink), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.Symlink(alternateTarget, alternateLink); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	doctor, err := New(Options{
		HomeDir:       homeDir,
		GOOS:          "darwin",
		CheckRuntimes: false,
		Now: func() time.Time {
			return time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	doctor.lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	report, err := doctor.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	antigravity := findClientFinding(t, report, "antigravity")
	if antigravity.Confidence != ConfidenceConflict {
		t.Fatalf("expected conflict confidence, got %q", antigravity.Confidence)
	}
	resolvedTarget, err := filepath.EvalSymlinks(alternateTarget)
	if err != nil {
		t.Fatalf("EvalSymlinks returned error: %v", err)
	}
	foundSymlink := false
	for _, candidate := range antigravity.Candidates {
		if candidate.Label == "alternate-symlink" && candidate.IsSymlink && candidate.Resolved == resolvedTarget {
			foundSymlink = true
		}
	}
	if !foundSymlink {
		t.Fatalf("expected symlink candidate in %#v", antigravity.Candidates)
	}

}

func TestScanMalformedCodexConfig(t *testing.T) {
	homeDir := t.TempDir()
	codexPath := filepath.Join(homeDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(codexPath, []byte("[mcp_servers.exa\nurl = \"broken\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	doctor, err := New(Options{
		HomeDir:       homeDir,
		GOOS:          "darwin",
		CheckRuntimes: false,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	doctor.lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	report, err := doctor.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	codex := findClientFinding(t, report, "codex-cli")
	if len(codex.Issues) == 0 {
		t.Fatalf("expected parse issues, got %#v", codex)
	}
}

func TestFormatAndMarshalReport(t *testing.T) {
	report := Report{
		Platform: "darwin",
		Clients: []ClientFinding{{
			ID:         "antigravity-cli",
			Name:       "Antigravity CLI",
			Confidence: ConfidenceMedium,
		}},
	}

	formatted := FormatReport(report)
	if !strings.Contains(formatted, "Antigravity CLI") {
		t.Fatalf("formatted report missing client: %s", formatted)
	}

	data, err := MarshalReportJSON(report)
	if err != nil {
		t.Fatalf("MarshalReportJSON returned error: %v", err)
	}
	if !strings.Contains(string(data), "\"platform\": \"darwin\"") {
		t.Fatalf("unexpected json output: %s", string(data))
	}
}

func fileSet(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

func findClientFinding(t *testing.T, report Report, id string) ClientFinding {
	t.Helper()
	for _, client := range report.Clients {
		if string(client.ID) == id {
			return client
		}
	}
	t.Fatalf("missing client %s in %#v", id, report.Clients)
	return ClientFinding{}
}

func TestScanDetectsManagedSettings(t *testing.T) {
	homeDir := t.TempDir()
	fakeManaged := filepath.Join(homeDir, "managed-settings.json")
	if err := os.WriteFile(fakeManaged, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	doc, err := New(Options{
		HomeDir:              homeDir,
		CheckRuntimes:        false,
		ManagedSettingsPaths: []string{fakeManaged},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc.lookPath = func(string) (string, error) { return "", os.ErrNotExist }

	report, err := doc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	cc := findClientFinding(t, report, "claude-code")
	if len(cc.Warnings) == 0 {
		t.Errorf("expected managed-settings warning for Claude Code, got none")
	}
	found := false
	for _, w := range cc.Warnings {
		if strings.Contains(w, "managed configuration") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'managed configuration' in warnings, got %v", cc.Warnings)
	}
}
