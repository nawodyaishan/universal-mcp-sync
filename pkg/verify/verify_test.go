package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/exa"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func TestVerifyBareMCPServersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	content := fmt.Sprintf(`{
  "exa": {
    "httpUrl": %q
  }
}`, exaURL)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := VerifyFile(path, config.FileKindBareMCPServers, len(exa.DefaultTools))
	if result.Status != StatusOK {
		t.Fatalf("expected status OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyMCPServersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	content := fmt.Sprintf(`{
  "mcpServers": {
    "exa": {
      "url": %q
    }
  }
}`, exaURL)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := VerifyFile(path, config.FileKindMCPServers, len(exa.DefaultTools))
	if result.Status != StatusOK {
		t.Fatalf("expected status OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFileSupportsClaudeDesktopStdioBridge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude_desktop_config.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	content := fmt.Sprintf(`{
  "mcpServers": {
    "exa": {
      "command": "npx",
      "args": ["-y", "mcp-remote", %q]
    }
  }
}`, exaURL)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := VerifyProviderFile(path, config.FileKindMCPServers, "exa", provider.MCPConfig{
		Type:    provider.TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "mcp-remote", exaURL},
	})
	if result.Status != StatusOK {
		t.Fatalf("expected status OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFile_GenericStdio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"mcpServers":{"github":{"command":"npx","args":["-y","@modelcontextprotocol/server-github"]}}}`
	_ = os.WriteFile(path, []byte(content), 0o600)

	cfg := provider.MCPConfig{Type: provider.TransportStdio, Command: "npx"}
	result := VerifyProviderFile(path, config.FileKindMCPServers, "github", cfg)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFile_GenericHTTP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"mcpServers":{"brave":{"url":"https://api.brave.com/mcp"}}}`
	_ = os.WriteFile(path, []byte(content), 0o600)

	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: "https://api.brave.com/mcp"}
	result := VerifyProviderFile(path, config.FileKindMCPServers, "brave", cfg)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFile_GenericCodexStdio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte(`[mcp_servers.playwright]
command = "npx"
args = ["@playwright/mcp@latest"]
`), 0o600)

	cfg := provider.MCPConfig{
		Type:    provider.TransportStdio,
		Command: "npx",
		Args:    []string{"@playwright/mcp@latest"},
	}
	result := VerifyProviderFile(path, config.FileKindCodexTOML, "playwright", cfg)
	if result.Status != StatusOK {
		t.Fatalf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFile_GenericCodexStdioMissingCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte(`[mcp_servers.playwright]
args = ["@playwright/mcp@latest"]
`), 0o600)

	cfg := provider.MCPConfig{Type: provider.TransportStdio, Command: "npx"}
	result := VerifyProviderFile(path, config.FileKindCodexTOML, "playwright", cfg)
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyProviderFile_GenericMissingEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o600)

	cfg := provider.MCPConfig{Type: provider.TransportStdio, Command: "npx"}
	result := VerifyProviderFile(path, config.FileKindMCPServers, "github", cfg)
	if result.Status != StatusFailed {
		t.Errorf("expected Failed for missing entry, got %s", result.Status)
	}
}

func TestVerifyContext7File_HeadersPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"mcpServers":{"context7":{"url":"https://mcp.context7.com/mcp","headers":{"CONTEXT7_API_KEY":"ctx7sk_123"}}}}`), 0o600)
	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP}
	result := VerifyProviderFile(path, config.FileKindMCPServers, "context7", cfg)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s", result.Status)
	}
}

func TestVerifyContext7File_HeadersMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"mcpServers":{"context7":{"url":"https://mcp.context7.com/mcp"}}}`), 0o600)
	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP}
	result := VerifyProviderFile(path, config.FileKindMCPServers, "context7", cfg)
	if result.Status != StatusFailed {
		t.Errorf("expected Failed, got %s", result.Status)
	}
}

func TestVerifyContext7CodexFile_HttpHeadersPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte("[mcp_servers.context7]\nurl = \"https://mcp.context7.com/mcp\"\nhttp_headers = { \"CONTEXT7_API_KEY\" = \"ctx7sk_123\" }"), 0o600)
	result := verifyContext7CodexFile(path)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s", result.Status)
	}
}

type mockRunner struct {
	lookPathErr error
	runOutput   string
	runErr      error
}

func (m mockRunner) LookPath(name string) (string, error) {
	return name, m.lookPathErr
}

func (m mockRunner) Run(name string, args ...string) (string, error) {
	return m.runOutput, m.runErr
}

func TestVerifyOptionalCLI(t *testing.T) {
	runner := mockRunner{runOutput: "v1.0.0"}
	result := VerifyOptionalCLI(runner, "test-cli", "--version")
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s", result.Status)
	}

	runnerMissing := mockRunner{lookPathErr: fmt.Errorf("not found")}
	resultMissing := VerifyOptionalCLI(runnerMissing, "missing-cli")
	if resultMissing.Status != StatusSkipped {
		t.Errorf("expected Skipped, got %s", resultMissing.Status)
	}

	runnerErr := mockRunner{runErr: fmt.Errorf("error")}
	resultErr := VerifyOptionalCLI(runnerErr, "fail-cli")
	if resultErr.Status != StatusWarning {
		t.Errorf("expected Warning, got %s", resultErr.Status)
	}
}

func TestVerifyExaBareMCPServersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	_ = os.WriteFile(path, []byte(fmt.Sprintf(`{"exa":{"url":%q}}`, exaURL)), 0o600)
	cfg := provider.MCPConfig{Type: provider.TransportHTTP}
	result := verifyExaBareMCPServersFile(path, cfg)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyExaNamedServerFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "named.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	_ = os.WriteFile(path, []byte(fmt.Sprintf(`{"exa":{"url":%q}}`, exaURL)), 0o600)
	cfg := provider.MCPConfig{Type: provider.TransportHTTP}
	result := verifyExaNamedServerFile(path, cfg)
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyNamedServerFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "named.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	_ = os.WriteFile(path, []byte(fmt.Sprintf(`{"exa":{"url":%q}}`, exaURL)), 0o600)
	result := verifyNamedServerFile(path, len(exa.DefaultTools))
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}
}

func TestVerifyCodexFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=123&tools=%s", strings.Join(exa.DefaultTools, ","))
	_ = os.WriteFile(path, []byte(fmt.Sprintf("[mcp_servers.exa]\nurl = %q", exaURL)), 0o600)
	result := verifyCodexFile(path, len(exa.DefaultTools))
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %s: %v", result.Status, result.Details)
	}

	_ = os.WriteFile(path, []byte(""), 0o600)
	resultMissing := verifyCodexFile(path, len(exa.DefaultTools))
	if resultMissing.Status != StatusFailed {
		t.Errorf("expected Failed, got %s", resultMissing.Status)
	}
}

func TestVerifyContext7BareAndNamed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"context7":{"url":"https://mcp.context7.com/mcp","headers":{"CONTEXT7_API_KEY":"ctx7sk_123"}}}`
	_ = os.WriteFile(path, []byte(content), 0o600)

	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP}
	res1 := verifyContext7BareMCPServersFile(path, cfg)
	if res1.Status != StatusOK {
		t.Errorf("Bare: expected OK, got %s: %v", res1.Status, res1.Details)
	}

	res2 := verifyContext7NamedServerFile(path, cfg)
	if res2.Status != StatusOK {
		t.Errorf("Named: expected OK, got %s: %v", res2.Status, res2.Details)
	}
}

func TestVerifyFile_Error(t *testing.T) {
	if res := VerifyFile("nonexistent", config.FileKindMCPServers, 0); res.Status != StatusFailed {
		t.Errorf("expected failed for nonexistent file, got %s", res.Status)
	}
	if res := VerifyFile("invalid.json", "invalid", 0); res.Status != StatusFailed {
		t.Errorf("expected failed for invalid kind, got %s", res.Status)
	}
}

func TestResultFrom(t *testing.T) {
	res := resultFrom("target", []string{"detail"}, true)
	if res.Status != StatusOK {
		t.Errorf("expected OK, got %s", res.Status)
	}
	res2 := resultFrom("target", []string{"detail"}, false)
	if res2.Status != StatusFailed {
		t.Errorf("expected Failed, got %s", res2.Status)
	}
}

// TestVerifyProviderFile_ZedContextServers is a regression test for the bug where
// verifyExaNamedServerFile used readRootServerEntry, which looks for root["exa"]
// and fails on Zed's settings.json where the entry lives under root["context_servers"]["exa"].
func TestVerifyProviderFile_ZedContextServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	exaURL := fmt.Sprintf("https://mcp.exa.ai/mcp?exaApiKey=test-key&tools=%s", strings.Join(exa.DefaultTools, ","))
	content := fmt.Sprintf(`{
  "context_servers": {
    "exa": {
      "url": %q
    }
  },
  "theme": "One Dark"
}`, exaURL)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: exaURL}
	result := VerifyProviderFile(path, config.FileKindNamedServer, "exa", cfg)
	if result.Status == StatusFailed {
		t.Errorf("expected OK or warning for Zed context_servers layout, got %s: %v", result.Status, result.Details)
	}
}

// TestVerifyProviderFile_Context7ZedContextServers is a regression test for the bug where
// verifyContext7NamedServerFile used readRootServerEntry and failed on Zed's settings.json
// where the entry lives under root["context_servers"]["context7"].
func TestVerifyProviderFile_Context7ZedContextServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := `{
  "context_servers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": {"CONTEXT7_API_KEY": "ctx7sk-testkey"}
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: "https://mcp.context7.com/mcp"}
	result := VerifyProviderFile(path, config.FileKindNamedServer, "context7", cfg)
	if result.Status == StatusFailed {
		t.Errorf("expected OK or warning for Zed context_servers layout, got %s: %v", result.Status, result.Details)
	}
}
