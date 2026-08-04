package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

func newDarwinQAManager(t *testing.T, homeDir string, runner CommandRunner) *Manager {
	t.Helper()

	manager, err := NewManager(homeDir, fixedNow(), runner)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	manager.Apps, err = config.DetectAppConfigsForOS(homeDir, "darwin")
	if err != nil {
		t.Fatalf("DetectAppConfigsForOS: %v", err)
	}

	return manager
}

func TestQAExaReadmeScenarios(t *testing.T) {
	homeDir := t.TempDir()

	// Initialize dummy files
	claudeDesktopPath := filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	antigravityPath := filepath.Join(homeDir, ".gemini", "config", "mcp_config.json")
	codexPath := filepath.Join(homeDir, ".codex", "config.toml")
	cursorPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	vscodePath := filepath.Join(homeDir, ".vscode", "mcp.json")
	windsurfPath := filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json")
	zedPath := filepath.Join(homeDir, ".config", "zed", "settings.json")
	roocodePath := filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json")
	opencodePath := filepath.Join(homeDir, ".opencode.json")
	kiroPath := filepath.Join(homeDir, ".kiro", "settings", "mcp.json")

	mustWriteFile(t, claudeDesktopPath, []byte("{}"))
	mustWriteFile(t, antigravityPath, []byte("{}"))
	mustWriteFile(t, codexPath, []byte(""))
	mustWriteFile(t, cursorPath, []byte("{}"))
	mustWriteFile(t, vscodePath, []byte("{}"))
	mustWriteFile(t, windsurfPath, []byte("{}"))
	mustWriteFile(t, zedPath, []byte("{}"))
	mustWriteFile(t, roocodePath, []byte("{}"))
	mustWriteFile(t, opencodePath, []byte("{}"))
	mustWriteFile(t, kiroPath, []byte("{}"))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{})

	key := "11111111-1111-1111-1111-111111111111"
	selected := make(map[config.AppID]bool)
	for _, id := range config.AppOrder {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.Prepare([]string{key}, selected, assignments)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 1. Verify Claude Desktop (Manual config / Bridge path)
	data, _ := os.ReadFile(claudeDesktopPath)
	if !bytes.Contains(data, []byte(`"command": "npx"`)) || !bytes.Contains(data, []byte(`"mcp-remote"`)) {
		t.Errorf("Claude Desktop mismatch")
	}

	// 2. Verify Windsurf (serverUrl)
	data, _ = os.ReadFile(windsurfPath)
	if !bytes.Contains(data, []byte(`"serverUrl":`)) {
		t.Errorf("Windsurf mismatch")
	}

	// 4. Verify VS Code (servers root + type: http)
	data, _ = os.ReadFile(vscodePath)
	if !bytes.Contains(data, []byte(`"servers":`)) || !bytes.Contains(data, []byte(`"type": "http"`)) {
		t.Errorf("VS Code mismatch:\n%s", string(data))
	}

	// 5. Verify Zed (context_servers root)
	data, _ = os.ReadFile(zedPath)
	if !bytes.Contains(data, []byte(`"context_servers":`)) {
		t.Errorf("Zed mismatch")
	}

	// 6. Verify Roo Code (type: streamable-http)
	data, _ = os.ReadFile(roocodePath)
	if !bytes.Contains(data, []byte(`"type": "streamable-http"`)) {
		t.Errorf("Roo Code mismatch")
	}

	// 7. Verify OpenCode (mcp root + enabled: true)
	data, _ = os.ReadFile(opencodePath)
	if !bytes.Contains(data, []byte(`"mcp":`)) || !bytes.Contains(data, []byte(`"enabled": true`)) {
		t.Errorf("OpenCode mismatch")
	}

	// 8. Verify Cursor (Standard url)
	data, _ = os.ReadFile(cursorPath)
	if !bytes.Contains(data, []byte(`"url":`)) {
		t.Errorf("Cursor mismatch")
	}

	// 9. Verify Kiro (Standard url)
	data, _ = os.ReadFile(kiroPath)
	if !bytes.Contains(data, []byte(`"url":`)) {
		t.Errorf("Kiro mismatch")
	}

	// 10. Verify Codex (TOML url)
	data, _ = os.ReadFile(codexPath)
	if !bytes.Contains(data, []byte(`url = "https://mcp.exa.ai/mcp`)) {
		t.Errorf("Codex mismatch")
	}
}

func TestQAIdempotency(t *testing.T) {
	homeDir := t.TempDir()
	claudeDesktopPath := filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	mustWriteFile(t, claudeDesktopPath, []byte("{}"))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{})

	key := "11111111-1111-1111-1111-111111111111"
	selected := map[config.AppID]bool{config.AppClaudeDesktop: true}
	assignments := DefaultAssignments(selected, 1)

	// First run
	plan1, _ := manager.Prepare([]string{key}, selected, assignments)
	_, _ = manager.Apply(plan1)
	data1, _ := os.ReadFile(claudeDesktopPath)

	// Second run with same key
	plan2, _ := manager.Prepare([]string{key}, selected, assignments)
	_, _ = manager.Apply(plan2)
	data2, _ := os.ReadFile(claudeDesktopPath)

	if !bytes.Equal(data1, data2) {
		t.Errorf("Idempotency failure: second run changed file content")
	}
}

func TestQAGitHubStdioSupportedClients(t *testing.T) {
	homeDir := t.TempDir()

	// Write empty config files for all clients that support stdio
	paths := map[config.AppID]string{
		config.AppClaudeDesktop: filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:        filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:        filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:      filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:           filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:       filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:      filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:          filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewGitHubProvider()
	pat := "ghp_" + strings.Repeat("a", 36)
	profiles := []provider.CredentialProfile{{
		ProviderID: "github",
		Values:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": pat},
		Label:      "ghp_...aaaa",
	}}

	selected := make(map[config.AppID]bool)
	for id := range paths {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	// No operations should have SkipReason for stdio-capable clients
	for _, op := range plan.Operations {
		if op.SkipReason != "" {
			t.Errorf("unexpected skip for %s: %s", op.AppName, op.SkipReason)
		}
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify Claude Desktop got stdio command written (no bridge needed for stdio provider)
	data, _ := os.ReadFile(paths[config.AppClaudeDesktop])
	if !bytes.Contains(data, []byte(`"command": "npx"`)) {
		t.Errorf("Claude Desktop: expected stdio command\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"@modelcontextprotocol/server-github"`)) {
		t.Errorf("Claude Desktop: expected GitHub server arg\n%s", data)
	}
	// PAT must appear in env block
	if !bytes.Contains(data, []byte(`"GITHUB_PERSONAL_ACCESS_TOKEN"`)) {
		t.Errorf("Claude Desktop: expected env key in config\n%s", data)
	}

	// Verify Cursor got stdio command written
	data, _ = os.ReadFile(paths[config.AppCursor])
	if !bytes.Contains(data, []byte(`"command": "npx"`)) {
		t.Errorf("Cursor: expected stdio command\n%s", data)
	}
}

func TestQAGitHubSkippedOnHTTPOnlyClients(t *testing.T) {
	homeDir := t.TempDir()

	antigravityPath := filepath.Join(homeDir, ".gemini", "config", "mcp_config.json")
	antigravityCLIPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json")
	mustWriteFile(t, antigravityPath, []byte("{}"))
	mustWriteFile(t, antigravityCLIPath, []byte("{}"))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{})

	prov := provider.NewGitHubProvider()
	pat := "ghp_" + strings.Repeat("a", 36)
	profiles := []provider.CredentialProfile{{
		ProviderID: "github",
		Values:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": pat},
		Label:      "ghp_...aaaa",
	}}
	selected := map[config.AppID]bool{
		config.AppAntigravityCLI: true,
		config.AppAntigravity:    true,
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	// All operations should be skipped for HTTP-only clients with a stdio provider
	skipped := 0
	for _, op := range plan.Operations {
		if op.SkipReason != "" {
			skipped++
		}
	}
	if skipped == 0 && len(plan.Warnings) == 0 {
		t.Error("expected AntigravityCLI and Antigravity to be skipped for stdio-only provider")
	}

	// Files should not be modified
	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, _ := os.ReadFile(antigravityCLIPath)
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity CLI settings should not be modified for GitHub stdio provider\n%s", data)
	}
	data, _ = os.ReadFile(antigravityPath)
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity config should not be modified for GitHub stdio provider\n%s", data)
	}
}

func TestQAExaAndGitHubCoexist(t *testing.T) {
	homeDir := t.TempDir()
	cursorPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	mustWriteFile(t, cursorPath, []byte("{}"))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{})
	selected := map[config.AppID]bool{config.AppCursor: true}
	assignments := DefaultAssignments(selected, 1)

	// Apply Exa first
	exaKey := "11111111-1111-1111-1111-111111111111"
	exaPlan, err := manager.Prepare([]string{exaKey}, selected, assignments)
	if err != nil {
		t.Fatalf("Prepare Exa: %v", err)
	}
	if _, err := manager.Apply(exaPlan); err != nil {
		t.Fatalf("Apply Exa: %v", err)
	}

	// Apply GitHub second
	pat := "ghp_" + strings.Repeat("a", 36)
	githubProv := provider.NewGitHubProvider()
	githubProfiles := []provider.CredentialProfile{{
		ProviderID: "github",
		Values:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": pat},
		Label:      "ghp_...aaaa",
	}}
	githubPlan, err := manager.PrepareProvider(githubProv, githubProfiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider GitHub: %v", err)
	}
	if _, err := manager.Apply(githubPlan); err != nil {
		t.Fatalf("Apply GitHub: %v", err)
	}

	data, _ := os.ReadFile(cursorPath)
	// Both providers must be present
	if !bytes.Contains(data, []byte(`"exa"`)) {
		t.Errorf("Cursor: Exa entry should survive GitHub sync\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"github"`)) {
		t.Errorf("Cursor: GitHub entry should be present\n%s", data)
	}
}

func TestQAContext7AllClients(t *testing.T) {
	homeDir := t.TempDir()

	// Write empty config files for all clients
	paths := map[config.AppID]string{
		config.AppClaudeDesktop:  filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:         filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:         filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:       filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:            filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:        filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:       filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:           filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
		config.AppAntigravityCLI: filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json"),
		config.AppAntigravity:    filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
		config.AppCodexCLI:       filepath.Join(homeDir, ".codex", "config.toml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}
	mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewContext7Provider()
	key := "ctx7sk-" + strings.Repeat("a", 20)
	profiles := []provider.CredentialProfile{{
		ProviderID: "context7",
		Values:     map[string]string{"CONTEXT7_API_KEY": key},
		Label:      "ctx7sk-aaaa...aaaa",
	}}
	selected := make(map[config.AppID]bool)
	for id := range paths {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	// Raw key must never appear in plan
	planText := FormatPlan(plan)
	if strings.Contains(planText, key) {
		t.Errorf("plan output must not contain raw API key")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Claude Desktop: stdio shape
	data, _ := os.ReadFile(paths[config.AppClaudeDesktop])
	if !bytes.Contains(data, []byte(`"@upstash/context7-mcp"`)) {
		t.Errorf("Claude Desktop: expected direct npx invocation\n%s", data)
	}

	// Cursor: url + headers
	data, _ = os.ReadFile(paths[config.AppCursor])
	if !bytes.Contains(data, []byte(`"https://mcp.context7.com/mcp"`)) {
		t.Errorf("Cursor: expected Context7 endpoint\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"CONTEXT7_API_KEY"`)) {
		t.Errorf("Cursor: expected headers field\n%s", data)
	}

	// Antigravity CLI: also has Accept header
	data, _ = os.ReadFile(paths[config.AppAntigravityCLI])
	if !bytes.Contains(data, []byte(`"Accept"`)) {
		t.Errorf("Antigravity CLI: expected Accept header\n%s", data)
	}

	// Codex: http_headers in TOML
	data, _ = os.ReadFile(paths[config.AppCodexCLI])
	if !bytes.Contains(data, []byte(`http_headers`)) {
		t.Errorf("Codex: expected http_headers in TOML\n%s", data)
	}

	// Idempotency check (T-D3)
	cursorData1, _ := os.ReadFile(paths[config.AppCursor])
	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply 2: %v", err)
	}

	data2, _ := os.ReadFile(paths[config.AppCursor])
	if !bytes.Equal(cursorData1, data2) {
		t.Errorf("Cursor file changed on second apply\nRun 1: %s\nRun 2: %s", string(cursorData1), string(data2))
	}
}

func TestQAExaAndContext7Coexist(t *testing.T) {
	homeDir := t.TempDir()
	cursorPath := filepath.Join(homeDir, ".cursor", "mcp.json")
	mustWriteFile(t, cursorPath, []byte("{}"))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{})
	selected := map[config.AppID]bool{config.AppCursor: true}
	assignments := DefaultAssignments(selected, 1)

	// 1. Sync Exa
	exaProv := provider.NewExaProvider()
	exaProfiles := []provider.CredentialProfile{{
		ProviderID: "exa",
		Values:     map[string]string{"EXA_API_KEY": "11111111-1111-1111-1111-111111111111"},
		Label:      "1111...1111",
	}}
	plan1, err := manager.PrepareProvider(exaProv, exaProfiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider Exa: %v", err)
	}
	_, err = manager.Apply(plan1)
	if err != nil {
		t.Fatalf("Apply Exa: %v", err)
	}

	exaData1, _ := os.ReadFile(cursorPath)

	// 2. Sync Context7
	ctx7Prov := provider.NewContext7Provider()
	ctx7Profiles := []provider.CredentialProfile{{
		ProviderID: "context7",
		Values:     map[string]string{"CONTEXT7_API_KEY": "ctx7sk-abcdef1234567890wxyz"},
		Label:      "ctx7sk-abcd...wxyz",
	}}
	plan2, err := manager.PrepareProvider(ctx7Prov, ctx7Profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider Context7: %v", err)
	}
	_, err = manager.Apply(plan2)
	if err != nil {
		t.Fatalf("Apply Context7: %v", err)
	}

	data, _ := os.ReadFile(cursorPath)
	// Both providers must be present
	if !bytes.Contains(data, []byte(`"exa"`)) {
		t.Errorf("Cursor: Exa entry should survive Context7 sync\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"context7"`)) {
		t.Errorf("Cursor: Context7 entry should be present\n%s", data)
	}
	if bytes.Contains(exaData1, []byte(`"headers"`)) {
		t.Errorf("Exa should not have headers")
	}
}

func TestQATavilyAllClients(t *testing.T) {
	homeDir := t.TempDir()

	paths := map[config.AppID]string{
		config.AppClaudeDesktop: filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:        filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:        filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:      filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:           filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:       filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:      filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:          filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewTavilyProvider()
	key := "tvly-" + strings.Repeat("a", 20)
	profiles := []provider.CredentialProfile{{
		ProviderID: "tavily",
		Values:     map[string]string{"TAVILY_API_KEY": key},
		Label:      "tvly-aaaa...aaaa",
	}}
	selected := make(map[config.AppID]bool)
	for id := range paths {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	// Raw key must never appear in plan
	planText := FormatPlan(plan)
	if strings.Contains(planText, key) {
		t.Errorf("plan output must not contain raw API key")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Claude Desktop: stdio shape
	data, _ := os.ReadFile(paths[config.AppClaudeDesktop])
	if !bytes.Contains(data, []byte(`"tavily-mcp@latest"`)) {
		t.Errorf("Claude Desktop: expected direct npx invocation for tavily-mcp@latest\n%s", data)
	}

	// Cursor: stdio shape
	data, _ = os.ReadFile(paths[config.AppCursor])
	if !bytes.Contains(data, []byte(`"tavily-mcp@latest"`)) {
		t.Errorf("Cursor: expected direct npx invocation for tavily-mcp@latest\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"TAVILY_API_KEY"`)) {
		t.Errorf("Cursor: expected TAVILY_API_KEY in env\n%s", data)
	}
}

func TestQAPlaywrightAllClients(t *testing.T) {
	homeDir := t.TempDir()

	paths := map[config.AppID]string{
		config.AppClaudeDesktop:  filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:         filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:         filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:       filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:            filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:        filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:       filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:           filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
		config.AppAntigravityCLI: filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json"),
		config.AppAntigravity:    filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
		config.AppCodexCLI:       filepath.Join(homeDir, ".codex", "config.toml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}
	mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewPlaywrightProvider()
	profiles := []provider.CredentialProfile{{
		ProviderID: "playwright",
		Values:     map[string]string{},
		Label:      "Default",
	}}
	selected := make(map[config.AppID]bool)
	for _, id := range config.AppOrder {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	warnings := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Gemini CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Antigravity CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity IDE does not support stdio transport") {
		t.Errorf("expected Antigravity IDE skip warning, got:\n%s", warnings)
	}

	foundClaudeCode := false
	for _, op := range plan.Operations {
		if op.AppID == config.AppClaudeCode {
			foundClaudeCode = true
			got := strings.Join(op.CLIAddArgs, " ")
			want := "mcp add -s user playwright -- npx @playwright/mcp@latest"
			if got != want {
				t.Fatalf("Claude Code args mismatch:\ngot:  %s\nwant: %s", got, want)
			}
		}
	}
	if !foundClaudeCode {
		t.Fatal("expected Claude Code CLI operation")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, _ := os.ReadFile(paths[config.AppClaudeDesktop])
	if !bytes.Contains(data, []byte(`"@playwright/mcp@latest"`)) {
		t.Errorf("Claude Desktop: expected Playwright MCP arg\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppCursor])
	if !bytes.Contains(data, []byte(`"command": "npx"`)) || !bytes.Contains(data, []byte(`"@playwright/mcp@latest"`)) {
		t.Errorf("Cursor: expected stdio Playwright config\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppVSCode])
	if bytes.Contains(data, []byte(`"type": "http"`)) {
		t.Errorf("VS Code: stdio provider must not be written as HTTP\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppRooCode])
	if !bytes.Contains(data, []byte(`"type": "stdio"`)) {
		t.Errorf("Roo Code: expected stdio type\n%s", data)
	}
	if bytes.Contains(data, []byte(`"streamable-http"`)) {
		t.Errorf("Roo Code: stdio provider must not be written as streamable-http\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppOpenCode])
	if !bytes.Contains(data, []byte(`"type": "local"`)) {
		t.Errorf("OpenCode: expected local type for stdio\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppCodexCLI])
	if !bytes.Contains(data, []byte(`[mcp_servers.playwright]`)) ||
		!bytes.Contains(data, []byte(`command = "npx"`)) ||
		!bytes.Contains(data, []byte(`args = ["@playwright/mcp@latest"]`)) {
		t.Errorf("Codex: expected stdio TOML\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppAntigravityCLI])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity CLI should not be modified for Playwright stdio provider\n%s", data)
	}
	data, _ = os.ReadFile(paths[config.AppAntigravity])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity IDE should not be modified for Playwright stdio provider\n%s", data)
	}
}

func TestQAKubernetesReadOnlyAllClients(t *testing.T) {
	homeDir := t.TempDir()

	paths := map[config.AppID]string{
		config.AppClaudeDesktop:  filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:         filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:         filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:       filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:            filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:        filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:       filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:           filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
		config.AppAntigravityCLI: filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json"),
		config.AppAntigravity:    filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
		config.AppCodexCLI:       filepath.Join(homeDir, ".codex", "config.toml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}
	mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewKubernetesProvider()
	profiles := []provider.CredentialProfile{{
		ProviderID: "kubernetes",
		Values:     map[string]string{},
		Label:      "Default",
	}}
	selected := make(map[config.AppID]bool)
	for _, id := range config.AppOrder {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	warnings := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Gemini CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Antigravity CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity IDE does not support stdio transport") {
		t.Errorf("expected Antigravity IDE skip warning, got:\n%s", warnings)
	}

	foundClaudeCode := false
	for _, op := range plan.Operations {
		if op.AppID == config.AppClaudeCode {
			foundClaudeCode = true
			got := strings.Join(op.CLIAddArgs, " ")
			want := "mcp add -s user kubernetes -- npx -y kubernetes-mcp-server@latest --read-only"
			if got != want {
				t.Fatalf("Claude Code args mismatch:\ngot:  %s\nwant: %s", got, want)
			}
		}
	}
	if !foundClaudeCode {
		t.Fatal("expected Claude Code CLI operation")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, id := range []config.AppID{
		config.AppClaudeDesktop,
		config.AppCursor,
		config.AppVSCode,
		config.AppWindsurf,
		config.AppZed,
		config.AppRooCode,
		config.AppOpenCode,
		config.AppKiro,
	} {
		data, _ := os.ReadFile(paths[id])
		if !bytes.Contains(data, []byte(`kubernetes-mcp-server@latest`)) {
			t.Errorf("%s: expected Kubernetes MCP server package\n%s", id, data)
		}
		if !bytes.Contains(data, []byte(`--read-only`)) {
			t.Errorf("%s: expected read-only flag\n%s", id, data)
		}
	}

	data, _ := os.ReadFile(paths[config.AppRooCode])
	if !bytes.Contains(data, []byte(`"type": "stdio"`)) {
		t.Errorf("Roo Code: expected stdio type\n%s", data)
	}
	if bytes.Contains(data, []byte(`"streamable-http"`)) {
		t.Errorf("Roo Code: stdio provider must not be written as streamable-http\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppOpenCode])
	if !bytes.Contains(data, []byte(`"type": "local"`)) {
		t.Errorf("OpenCode: expected local type for stdio\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppCodexCLI])
	if !bytes.Contains(data, []byte(`[mcp_servers.kubernetes]`)) ||
		!bytes.Contains(data, []byte(`command = "npx"`)) ||
		!bytes.Contains(data, []byte(`"kubernetes-mcp-server@latest"`)) ||
		!bytes.Contains(data, []byte(`"--read-only"`)) {
		t.Errorf("Codex: expected read-only stdio TOML\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppAntigravityCLI])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity CLI should not be modified for Kubernetes stdio provider\n%s", data)
	}
	data, _ = os.ReadFile(paths[config.AppAntigravity])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity IDE should not be modified for Kubernetes stdio provider\n%s", data)
	}
}

func TestQATerraformDockerAllClients(t *testing.T) {
	homeDir := t.TempDir()

	paths := map[config.AppID]string{
		config.AppClaudeDesktop:  filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:         filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:         filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:       filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:            filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:        filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:       filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:           filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
		config.AppAntigravityCLI: filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json"),
		config.AppAntigravity:    filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
		config.AppCodexCLI:       filepath.Join(homeDir, ".codex", "config.toml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}
	mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

	runner := fakeRunner{
		available: map[string]bool{"claude": true, "docker": true},
		outputs: map[string]string{
			"docker info --format {{.ServerVersion}}": "27.0.0",
		},
	}
	manager := newDarwinQAManager(t, homeDir, runner)

	prov := provider.NewTerraformProvider()
	profiles := []provider.CredentialProfile{{
		ProviderID: "terraform",
		Values:     map[string]string{},
		Label:      "Default",
	}}
	selected := make(map[config.AppID]bool)
	for _, id := range config.AppOrder {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	warnings := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Gemini CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Antigravity CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity IDE does not support stdio transport") {
		t.Errorf("expected Antigravity IDE skip warning, got:\n%s", warnings)
	}
	if strings.Contains(warnings, "Docker") {
		t.Errorf("did not expect Docker prerequisite warning, got:\n%s", warnings)
	}

	foundClaudeCode := false
	for _, op := range plan.Operations {
		if op.AppID == config.AppClaudeCode {
			foundClaudeCode = true
			got := strings.Join(op.CLIAddArgs, " ")
			want := "mcp add -s user terraform -- docker run -i --rm -e ENABLE_TF_OPERATIONS=false hashicorp/terraform-mcp-server:0.5.2"
			if got != want {
				t.Fatalf("Claude Code args mismatch:\ngot:  %s\nwant: %s", got, want)
			}
		}
	}
	if !foundClaudeCode {
		t.Fatal("expected Claude Code CLI operation")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, id := range []config.AppID{
		config.AppClaudeDesktop,
		config.AppCursor,
		config.AppVSCode,
		config.AppWindsurf,
		config.AppZed,
		config.AppRooCode,
		config.AppOpenCode,
		config.AppKiro,
	} {
		data, _ := os.ReadFile(paths[id])
		if id == config.AppOpenCode {
			if !bytes.Contains(data, []byte(`"command": [`)) || !bytes.Contains(data, []byte(`"docker"`)) {
				t.Errorf("%s: expected Docker command array\n%s", id, data)
			}
		} else if !bytes.Contains(data, []byte(`"command": "docker"`)) {
			t.Errorf("%s: expected Docker command string\n%s", id, data)
		}
		if !bytes.Contains(data, []byte(`hashicorp/terraform-mcp-server:0.5.2`)) {
			t.Errorf("%s: expected Terraform MCP image\n%s", id, data)
		}
		if !bytes.Contains(data, []byte(`ENABLE_TF_OPERATIONS=false`)) {
			t.Errorf("%s: expected operations-disabled env flag\n%s", id, data)
		}
	}

	data, _ := os.ReadFile(paths[config.AppRooCode])
	if !bytes.Contains(data, []byte(`"type": "stdio"`)) {
		t.Errorf("Roo Code: expected stdio type\n%s", data)
	}
	if bytes.Contains(data, []byte(`"streamable-http"`)) {
		t.Errorf("Roo Code: stdio provider must not be written as streamable-http\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppOpenCode])
	if !bytes.Contains(data, []byte(`"type": "local"`)) {
		t.Errorf("OpenCode: expected local type for stdio\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppCodexCLI])
	if !bytes.Contains(data, []byte(`[mcp_servers.terraform]`)) ||
		!bytes.Contains(data, []byte(`command = "docker"`)) ||
		!bytes.Contains(data, []byte(`"hashicorp/terraform-mcp-server:0.5.2"`)) ||
		!bytes.Contains(data, []byte(`"ENABLE_TF_OPERATIONS=false"`)) {
		t.Errorf("Codex: expected Docker stdio TOML\n%s", data)
	}

	data, _ = os.ReadFile(paths[config.AppAntigravityCLI])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity CLI should not be modified for Terraform stdio provider\n%s", data)
	}
	data, _ = os.ReadFile(paths[config.AppAntigravity])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity IDE should not be modified for Terraform stdio provider\n%s", data)
	}
}

// TestQACodeGraphAllClients validates the CodeGraph provider across all supported clients.
// CodeGraph is a 100% local stdio MCP server that requires no API keys or credentials.
// It ships via npm (@colbymchenry/codegraph) and is launched with:
//
//	npx -y @colbymchenry/codegraph mcp
//
// Expected behaviour:
//   - Every stdio-capable client receives an npx stdio config.
//   - Antigravity CLI and Antigravity IDE are skipped (no stdio support).
//   - No credential values appear anywhere in the output.
func TestQACodeGraphAllClients(t *testing.T) {
	homeDir := t.TempDir()

	paths := map[config.AppID]string{
		config.AppClaudeDesktop:  filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		config.AppCursor:         filepath.Join(homeDir, ".cursor", "mcp.json"),
		config.AppVSCode:         filepath.Join(homeDir, ".vscode", "mcp.json"),
		config.AppWindsurf:       filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
		config.AppZed:            filepath.Join(homeDir, ".config", "zed", "settings.json"),
		config.AppRooCode:        filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
		config.AppOpenCode:       filepath.Join(homeDir, ".opencode.json"),
		config.AppKiro:           filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
		config.AppAntigravityCLI: filepath.Join(homeDir, ".gemini", "antigravity-cli", "mcp_config.json"),
		config.AppAntigravity:    filepath.Join(homeDir, ".gemini", "config", "mcp_config.json"),
		config.AppCodexCLI:       filepath.Join(homeDir, ".codex", "config.toml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("{}"))
	}
	mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

	manager := newDarwinQAManager(t, homeDir, fakeRunner{available: map[string]bool{"claude": true}})

	prov := provider.NewCodeGraphProvider()
	profiles := []provider.CredentialProfile{{
		ProviderID: "codegraph",
		Values:     map[string]string{},
		Label:      "Default",
	}}
	selected := make(map[config.AppID]bool)
	for _, id := range config.AppOrder {
		selected[id] = true
	}
	assignments := DefaultAssignments(selected, 1)

	plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
	if err != nil {
		t.Fatalf("PrepareProvider: %v", err)
	}

	// CodeGraph is stdio — Antigravity CLI and IDE must be skipped.
	warnings := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warnings, "Antigravity CLI does not support stdio transport") {
		t.Errorf("expected Antigravity CLI skip warning, got:\n%s", warnings)
	}
	if !strings.Contains(warnings, "Antigravity IDE does not support stdio transport") {
		t.Errorf("expected Antigravity IDE skip warning, got:\n%s", warnings)
	}

	// Claude Code CLI path must use the correct npx invocation.
	foundClaudeCode := false
	for _, op := range plan.Operations {
		if op.AppID == config.AppClaudeCode {
			foundClaudeCode = true
			got := strings.Join(op.CLIAddArgs, " ")
			want := "mcp add -s user codegraph -- npx -y @colbymchenry/codegraph mcp"
			if got != want {
				t.Fatalf("Claude Code args mismatch:\ngot:  %s\nwant: %s", got, want)
			}
		}
	}
	if !foundClaudeCode {
		t.Fatal("expected Claude Code CLI operation")
	}

	_, err = manager.Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Claude Desktop: must receive the npx stdio command.
	data, _ := os.ReadFile(paths[config.AppClaudeDesktop])
	if !bytes.Contains(data, []byte(`"@colbymchenry/codegraph"`)) {
		t.Errorf("Claude Desktop: expected CodeGraph package arg\n%s", data)
	}

	// Cursor: standard stdio JSON shape.
	data, _ = os.ReadFile(paths[config.AppCursor])
	if !bytes.Contains(data, []byte(`"command": "npx"`)) || !bytes.Contains(data, []byte(`"@colbymchenry/codegraph"`)) {
		t.Errorf("Cursor: expected stdio CodeGraph config\n%s", data)
	}

	// VS Code: stdio provider must not be written as HTTP type.
	data, _ = os.ReadFile(paths[config.AppVSCode])
	if bytes.Contains(data, []byte(`"type": "http"`)) {
		t.Errorf("VS Code: stdio provider must not be written as HTTP\n%s", data)
	}

	// Roo Code: must carry explicit stdio type.
	data, _ = os.ReadFile(paths[config.AppRooCode])
	if !bytes.Contains(data, []byte(`"type": "stdio"`)) {
		t.Errorf("Roo Code: expected stdio type\n%s", data)
	}
	if bytes.Contains(data, []byte(`"streamable-http"`)) {
		t.Errorf("Roo Code: stdio provider must not be written as streamable-http\n%s", data)
	}

	// OpenCode: local type for stdio.
	data, _ = os.ReadFile(paths[config.AppOpenCode])
	if !bytes.Contains(data, []byte(`"type": "local"`)) {
		t.Errorf("OpenCode: expected local type for stdio\n%s", data)
	}

	// Codex CLI: TOML with correct npx command and args.
	data, _ = os.ReadFile(paths[config.AppCodexCLI])
	if !bytes.Contains(data, []byte(`[mcp_servers.codegraph]`)) ||
		!bytes.Contains(data, []byte(`command = "npx"`)) ||
		!bytes.Contains(data, []byte(`"@colbymchenry/codegraph"`)) {
		t.Errorf("Codex: expected stdio TOML\n%s", data)
	}

	// Antigravity CLI and IDE: must remain untouched (stdio not supported).
	data, _ = os.ReadFile(paths[config.AppAntigravityCLI])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity CLI should not be modified for CodeGraph stdio provider\n%s", data)
	}
	data, _ = os.ReadFile(paths[config.AppAntigravity])
	if !bytes.Equal(data, []byte("{}")) {
		t.Errorf("Antigravity IDE should not be modified for CodeGraph stdio provider\n%s", data)
	}
}
