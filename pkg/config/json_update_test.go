package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func TestUpdateMCPServersJSONPreservesUnrelatedFields(t *testing.T) {
	data := mustReadFixture(t, "claude_desktop.json")
	exaURL := "https://mcp.exa.ai/mcp?exaApiKey=11111111-1111-1111-1111-111111111111&tools=web_search_exa"
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: exaURL}

	updated, err := UpdateMCPServersJSON(data, "exa", "mcpServers", "url", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateMCPServersJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	if root["theme"] != "dark" {
		t.Fatalf("expected unrelated theme field to survive, got %#v", root["theme"])
	}

	servers := root["mcpServers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Fatal("expected existing context7 server to remain")
	}
	exa := servers["exa"].(map[string]any)
	if exa["url"] != exaURL {
		t.Fatalf("expected Exa URL to be updated, got %#v", exa["url"])
	}
}

func TestUpdateMCPServersJSONSupportsStdioServers(t *testing.T) {
	data := mustReadFixture(t, "claude_desktop.json")
	cfg := provider.MCPConfig{
		Type:    provider.TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "mcp-remote", "https://mcp.exa.ai/mcp?exaApiKey=11111111-1111-1111-1111-111111111111&tools=web_search_exa,web_search_advanced_exa,web_fetch_exa"},
	}

	updated, err := UpdateMCPServersJSON(data, "exa", "mcpServers", "url", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateMCPServersJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	servers := root["mcpServers"].(map[string]any)
	exa := servers["exa"].(map[string]any)
	if exa["command"] != "npx" {
		t.Fatalf("expected stdio command to be set, got %#v", exa["command"])
	}
	args := exa["args"].([]any)
	if len(args) != 3 || args[1] != "mcp-remote" {
		t.Fatalf("expected mcp-remote args, got %#v", args)
	}
	if _, ok := exa["url"]; ok {
		t.Fatalf("did not expect url field for stdio config, got %#v", exa["url"])
	}
}

func TestUpdateGeminiSettingsPreservesUISecurity(t *testing.T) {
	data := mustReadFixture(t, "gemini_settings.json")
	exaURL := "https://mcp.exa.ai/mcp?exaApiKey=11111111-1111-1111-1111-111111111111&tools=web_search_exa"
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: exaURL}

	updated, err := UpdateMCPServersJSON(data, "exa", "mcpServers", "httpUrl", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateMCPServersJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	if _, ok := root["ui"].(map[string]any); !ok {
		t.Fatal("expected ui field to remain")
	}
	if _, ok := root["security"].(map[string]any); !ok {
		t.Fatal("expected security field to remain")
	}
	servers := root["mcpServers"].(map[string]any)
	exa := servers["exa"].(map[string]any)
	if exa["httpUrl"] != exaURL {
		t.Fatalf("expected httpUrl to be set, got %#v", exa["httpUrl"])
	}
}

func TestUpdateBareMCPServersJSON(t *testing.T) {
	data := []byte("{\n  \"other\": {\n    \"url\": \"https://example.com\"\n  }\n}\n")
	exaURL := "https://mcp.exa.ai/mcp?exaApiKey=11111111-1111-1111-1111-111111111111&tools=web_search_exa"
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: exaURL}

	updated, err := UpdateBareMCPServersJSON(data, "exa", "httpUrl", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateBareMCPServersJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	if _, ok := root["mcpServers"]; ok {
		t.Fatal("did not expect mcpServers root key")
	}
	exa := root["exa"].(map[string]any)
	if exa["httpUrl"] != exaURL {
		t.Fatalf("expected Exa URL to be updated, got %#v", exa["httpUrl"])
	}
	if _, ok := root["other"].(map[string]any); !ok {
		t.Fatal("expected unrelated server entries to remain")
	}
}

func TestUpdateNamedServerJSONReplacesMalformedAntigravityURL(t *testing.T) {
	data := mustReadFixture(t, "antigravity.json")
	exaURL := "https://mcp.exa.ai/mcp?exaApiKey=11111111-1111-1111-1111-111111111111&tools=web_search_exa"
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: exaURL}

	updated, err := UpdateNamedServerJSON(data, "exa", "", "serverUrl", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateNamedServerJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	exa := root["exa"].(map[string]any)
	if exa["serverUrl"] != exaURL {
		t.Fatalf("expected Exa serverUrl to be replaced, got %#v", exa["serverUrl"])
	}
	if _, ok := root["other"].(map[string]any); !ok {
		t.Fatal("expected unrelated server entries to remain")
	}
}

func TestUpdateMCPServersJSONWithExtraFields(t *testing.T) {
	exaURL := "https://mcp.exa.ai/mcp"
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: exaURL}
	extra := map[string]any{"type": "streamable-http"}

	updated, err := UpdateMCPServersJSON(nil, "exa", "mcpServers", "url", cfg, extra)
	if err != nil {
		t.Fatalf("UpdateMCPServersJSON: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	exa := root["mcpServers"].(map[string]any)["exa"].(map[string]any)
	if exa["type"] != "streamable-http" {
		t.Errorf("expected extra field 'type', got %#v", exa["type"])
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func decodeJSONForTest(t *testing.T, data []byte) map[string]any {
	t.Helper()
	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	return root
}

func TestBuildConfigMap_EmitsHeadersWhenPresent(t *testing.T) {
	cfg := provider.MCPConfig{
		Type:    provider.TransportStreamableHTTP,
		URL:     "https://mcp.context7.com/mcp",
		Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"},
	}
	result, _ := UpdateMCPServersJSON([]byte("{}"), "context7", "mcpServers", "url", cfg, nil)
	if !bytes.Contains(result, []byte(`"headers"`)) {
		t.Errorf("expected headers in output:\n%s", result)
	}
	if !bytes.Contains(result, []byte(`"CONTEXT7_API_KEY"`)) {
		t.Errorf("expected header key in output:\n%s", result)
	}
}

func TestUpdateNamedServerJSON_Stdio(t *testing.T) {
	data := []byte(`{"mcp":{}}`)
	cfg := provider.MCPConfig{
		Type:    provider.TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "mcp-remote", "url"},
	}
	updated, err := UpdateNamedServerJSON(data, "exa", "mcp", "url", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeJSONForTest(t, updated)
	mcp := root["mcp"].(map[string]any)
	exa := mcp["exa"].(map[string]any)
	if exa["command"] != "npx" {
		t.Errorf("expected command npx, got %v", exa["command"])
	}
}

func TestUpdateOpenCodeJSONRemoteProvider(t *testing.T) {
	data := []byte(`{"model":"anthropic/claude","mcp":{"manual":{"type":"remote","url":"https://example.com/mcp"}}}`)
	cfg := provider.MCPConfig{
		Type:    provider.TransportStreamableHTTP,
		URL:     "https://mcp.context7.com/mcp",
		Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"},
	}

	updated, err := UpdateOpenCodeJSON(data, "context7", cfg)
	if err != nil {
		t.Fatalf("UpdateOpenCodeJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	if root["model"] != "anthropic/claude" {
		t.Fatalf("expected unrelated model field to survive, got %#v", root["model"])
	}
	mcp := root["mcp"].(map[string]any)
	if _, ok := mcp["manual"].(map[string]any); !ok {
		t.Fatal("expected unrelated MCP entry to survive")
	}
	entry := mcp["context7"].(map[string]any)
	if entry["type"] != "remote" {
		t.Fatalf("expected remote type, got %#v", entry["type"])
	}
	if entry["url"] != cfg.URL {
		t.Fatalf("expected url %q, got %#v", cfg.URL, entry["url"])
	}
	if entry["enabled"] != true {
		t.Fatalf("expected enabled=true, got %#v", entry["enabled"])
	}
	headers := entry["headers"].(map[string]any)
	if headers["CONTEXT7_API_KEY"] != "ctx7sk_test" {
		t.Fatalf("expected header to be written, got %#v", headers)
	}
}

func TestUpdateOpenCodeJSONLocalProvider(t *testing.T) {
	cfg := provider.MCPConfig{
		Type:    provider.TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "@playwright/mcp@latest"},
		Env:     map[string]string{"DEBUG": "pw:mcp"},
	}

	updated, err := UpdateOpenCodeJSON([]byte("{}"), "playwright", cfg)
	if err != nil {
		t.Fatalf("UpdateOpenCodeJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	entry := root["mcp"].(map[string]any)["playwright"].(map[string]any)
	if entry["type"] != "local" {
		t.Fatalf("expected local type, got %#v", entry["type"])
	}
	command := entry["command"].([]any)
	if len(command) != 3 || command[0] != "npx" || command[2] != "@playwright/mcp@latest" {
		t.Fatalf("unexpected command array: %#v", command)
	}
	env := entry["environment"].(map[string]any)
	if env["DEBUG"] != "pw:mcp" {
		t.Fatalf("expected environment map, got %#v", env)
	}
	if entry["enabled"] != true {
		t.Fatalf("expected enabled=true, got %#v", entry["enabled"])
	}
}

func TestUpdateOpenCodeJSONReplacesProviderOnly(t *testing.T) {
	data := []byte(`{"mcp":{"playwright":{"type":"local","command":["old"]},"manual":{"type":"remote","url":"https://example.com"}}}`)
	cfg := provider.MCPConfig{Type: provider.TransportHTTP, URL: "https://new.example.com/mcp"}

	updated, err := UpdateOpenCodeJSON(data, "playwright", cfg)
	if err != nil {
		t.Fatalf("UpdateOpenCodeJSON returned error: %v", err)
	}

	root := decodeJSONForTest(t, updated)
	mcp := root["mcp"].(map[string]any)
	if _, ok := mcp["manual"].(map[string]any); !ok {
		t.Fatal("expected manual entry to remain")
	}
	entry := mcp["playwright"].(map[string]any)
	if entry["type"] != "remote" || entry["url"] != "https://new.example.com/mcp" {
		t.Fatalf("expected provider entry to be replaced, got %#v", entry)
	}
}

// --- MergeVSCodeInputs tests ---

func TestMergeVSCodeInputs_Empty(t *testing.T) {
	data := []byte(`{"servers":{"exa":{"url":"https://x.example/mcp"}}}` + "\n")
	out, err := MergeVSCodeInputs(data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("expected unchanged output for nil inputs, got %s", out)
	}
}

func TestMergeVSCodeInputs_Adds(t *testing.T) {
	data := []byte(`{"servers":{"exa":{"url":"https://x.example/mcp"}}}` + "\n")
	inp := []VSCodeInput{{Type: "promptString", ID: "exa-api-key", Description: "Exa API Key", Password: true}}
	out, err := MergeVSCodeInputs(data, inp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `"inputs"`) {
		t.Errorf("expected inputs key in output, got %s", out)
	}
	if !strings.Contains(string(out), `"exa-api-key"`) {
		t.Errorf("expected exa-api-key in output, got %s", out)
	}
}

func TestMergeVSCodeInputs_Deduplicates(t *testing.T) {
	data := []byte(`{"inputs":[{"type":"promptString","id":"exa-api-key","description":"Old","password":false}],"servers":{}}` + "\n")
	inp := []VSCodeInput{{Type: "promptString", ID: "exa-api-key", Description: "New", Password: true}}
	out, err := MergeVSCodeInputs(data, inp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(string(out), `"exa-api-key"`) != 1 {
		t.Errorf("expected exactly one exa-api-key in output, got %s", out)
	}
	if strings.Contains(string(out), `"Old"`) {
		t.Errorf("expected old description replaced, got %s", out)
	}
	if !strings.Contains(string(out), `"New"`) {
		t.Errorf("expected new description in output, got %s", out)
	}
}

func TestMergeVSCodeInputs_PreservesOthers(t *testing.T) {
	data := []byte(`{"inputs":[{"type":"promptString","id":"other-key","description":"Other","password":false}],"servers":{}}` + "\n")
	inp := []VSCodeInput{{Type: "promptString", ID: "exa-api-key", Description: "Exa API Key", Password: true}}
	out, err := MergeVSCodeInputs(data, inp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `"other-key"`) {
		t.Errorf("expected other-key preserved in output, got %s", out)
	}
	if !strings.Contains(string(out), `"exa-api-key"`) {
		t.Errorf("expected exa-api-key added in output, got %s", out)
	}
}

func TestMergeVSCodeInputs_MalformedJSON(t *testing.T) {
	_, err := MergeVSCodeInputs([]byte(`{broken`), []VSCodeInput{{ID: "x"}})
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestUpdateNamedServerJSON_PreservesSandboxFields(t *testing.T) {
	data := []byte(`{
  "servers": {
    "exa": {
      "type": "http",
      "url": "https://old.example/mcp",
      "sandboxEnabled": true,
      "sandbox": {"filesystem": "readonly", "network": "none"}
    }
  }
}`)
	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: "https://new.example/mcp"}
	updated, err := UpdateNamedServerJSON(data, "exa", "servers", "url", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(updated, []byte(`"sandboxEnabled": true`)) {
		t.Errorf("sandboxEnabled was stripped:\n%s", updated)
	}
	if !bytes.Contains(updated, []byte(`"sandbox"`)) {
		t.Errorf("sandbox object was stripped:\n%s", updated)
	}
	if !bytes.Contains(updated, []byte(`"readonly"`)) {
		t.Errorf("sandbox content was stripped:\n%s", updated)
	}
}

// TestStripJSONComments verifies comment stripping used to handle JSONC files such as Zed's settings.json.
func TestStripJSONComments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON unchanged",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "single-line comment removed",
			input: "{\n  // this is a comment\n  \"key\": \"value\"\n}",
			want:  "{\n  \n  \"key\": \"value\"\n}",
		},
		{
			name:  "block comment removed",
			input: `{"key": /* comment */ "value"}`,
			want:  `{"key":  "value"}`,
		},
		{
			name:  "comment inside string preserved",
			input: `{"url": "https://example.com/mcp?key=abc//def"}`,
			want:  `{"url": "https://example.com/mcp?key=abc//def"}`,
		},
		{
			name:  "slash-star inside string preserved",
			input: `{"note": "a /* not a comment */"}`,
			want:  `{"note": "a /* not a comment */"}`,
		},
		{
			name:  "escaped quote inside string",
			input: `{"key": "say \"// not a comment\""}`,
			want:  `{"key": "say \"// not a comment\""}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(stripJSONComments([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("stripJSONComments(%q)\n got  %q\n want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestStripTrailingCommas verifies trailing-comma removal for JSONC files.
func TestStripTrailingCommas(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON unchanged",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "trailing comma in object",
			input: "{\"a\": 1,\n}",
			want:  "{\"a\": 1\n}",
		},
		{
			name:  "trailing comma in array",
			input: "[1, 2,\n]",
			want:  "[1, 2\n]",
		},
		{
			name:  "comma inside string preserved",
			input: `{"key": "a,}"}`,
			want:  `{"key": "a,}"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(stripTrailingCommas([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("stripTrailingCommas(%q)\n got  %q\n want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestUpdateNamedServerJSON_ZedJSONCFile verifies that Zed's settings.json with
// JSONC-style comments (// ...) and trailing commas is parsed and updated without error.
func TestUpdateNamedServerJSON_ZedJSONCFile(t *testing.T) {
	data := []byte(`{
  // Zed settings — this is JSONC, not strict JSON
  "context_servers": {
    "other-server": {
      "url": "https://other.example/mcp",
    },
  },
  /* multi-line comment
     spanning two lines */
  "theme": "One Dark",
}`)
	cfg := provider.MCPConfig{Type: provider.TransportStreamableHTTP, URL: "https://mcp.exa.ai/mcp?exaApiKey=test"}
	updated, err := UpdateNamedServerJSON(data, "exa", "context_servers", "url", cfg, nil)
	if err != nil {
		t.Fatalf("UpdateNamedServerJSON on JSONC Zed file: %v", err)
	}
	if !bytes.Contains(updated, []byte(`"exa"`)) {
		t.Errorf("exa server not written:\n%s", updated)
	}
	if !bytes.Contains(updated, []byte(`mcp.exa.ai`)) {
		t.Errorf("exa URL not written:\n%s", updated)
	}
}
