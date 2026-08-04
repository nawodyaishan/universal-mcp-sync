package manifest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAllClientsIncludesRepoAppIDsInOrder(t *testing.T) {
	got := make([]ClientID, 0, len(AllClients()))
	for _, client := range AllClients() {
		got = append(got, client.ID)
	}

	want := []ClientID{
		ClientClaudeDesktop,
		ClientClaudeCode,
		ClientCursor,
		ClientVSCode,
		ClientWindsurf,
		ClientZed,
		ClientRooCode,
		ClientOpenCode,
		ClientKiro,
		ClientAntigravityCLI,
		ClientAntigravity,
		ClientCodexCLI,
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unexpected client IDs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAllProvidersIncludesSupportedProviderIDsInOrder(t *testing.T) {
	got := make([]ProviderID, 0, len(AllProviders()))
	for _, provider := range AllProviders() {
		got = append(got, provider.ID)
	}

	want := []ProviderID{
		ProviderExa,
		ProviderGitHub,
		ProviderContext7,
		ProviderTavily,
		ProviderPlaywright,
		ProviderKubernetes,
		ProviderTerraform,
		ProviderCodeGraph,
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unexpected provider IDs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClientByIDReturnsCopy(t *testing.T) {
	client, ok := ClientByID(ClientCursor)
	if !ok {
		t.Fatal("expected cursor manifest")
	}
	client.Candidates[0].Label = "changed"

	again, ok := ClientByID(ClientCursor)
	if !ok {
		t.Fatal("expected cursor manifest on second lookup")
	}
	if again.Candidates[0].Label == "changed" {
		t.Fatal("ClientByID returned shared backing data")
	}
}

func TestProviderByIDReturnsCopy(t *testing.T) {
	provider, ok := ProviderByID(ProviderGitHub)
	if !ok {
		t.Fatal("expected github provider")
	}
	provider.RuntimeIDs[0] = "changed"

	again, ok := ProviderByID(ProviderGitHub)
	if !ok {
		t.Fatal("expected github provider on second lookup")
	}
	if again.RuntimeIDs[0] == "changed" {
		t.Fatal("ProviderByID returned shared backing data")
	}
}

func TestForPlatformFiltersCandidates(t *testing.T) {
	clients := ForPlatform(AllClients(), "linux")
	vscode, ok := findClient(clients, ClientVSCode)
	if !ok {
		t.Fatal("expected vscode manifest")
	}

	labels := make([]string, 0, len(vscode.Candidates))
	for _, candidate := range vscode.Candidates {
		labels = append(labels, candidate.Label)
	}

	if !slices.Contains(labels, "user-linux") {
		t.Fatalf("expected linux user candidate, got %v", labels)
	}
	if slices.Contains(labels, "user-darwin") {
		t.Fatalf("unexpected darwin candidate in linux filter: %v", labels)
	}
}

func TestExpandPathHandlesHomeWorkspaceAndSpaces(t *testing.T) {
	got, err := ExpandPath(
		"{{.Home}}/Library/Application Support/Claude/claude_desktop_config.json",
		PathVars{Home: "/Users/Test User"},
	)
	if err != nil {
		t.Fatalf("ExpandPath returned error: %v", err)
	}

	want := filepath.Clean("/Users/Test User/Library/Application Support/Claude/claude_desktop_config.json")
	if got != want {
		t.Fatalf("unexpected expanded path: got %q want %q", got, want)
	}

	projectPath, err := ExpandPath("{{.Workspace}}/.cursor/mcp.json", PathVars{Workspace: "/tmp/project"})
	if err != nil {
		t.Fatalf("ExpandPath workspace returned error: %v", err)
	}
	if projectPath != filepath.Clean("/tmp/project/.cursor/mcp.json") {
		t.Fatalf("unexpected workspace expansion: %q", projectPath)
	}
}

func TestExpandPathErrorsOnMissingVariables(t *testing.T) {
	if _, err := ExpandPath("{{.Home}}/.codex/config.toml", PathVars{}); err == nil {
		t.Fatal("expected missing home error")
	}
	if _, err := ExpandPath("{{.Workspace}}/.mcp.json", PathVars{Home: "/tmp/home"}); err == nil {
		t.Fatal("expected missing workspace error")
	}
}

func TestDeprecatedCandidatesHaveReplacementOrNote(t *testing.T) {
	for _, client := range AllClients() {
		for _, candidate := range client.Candidates {
			if !candidate.Deprecated {
				continue
			}
			if candidate.ReplacedBy == "" && candidate.DeprecationNote == "" {
				t.Fatalf("%s candidate %s is deprecated without replacement or note", client.ID, candidate.Label)
			}
		}
	}
}

func TestCandidateLabelsAreUniquePerClient(t *testing.T) {
	for _, client := range AllClients() {
		seen := make(map[string]bool)
		for _, candidate := range client.Candidates {
			if seen[candidate.Label] {
				t.Fatalf("%s has duplicate candidate label %q", client.ID, candidate.Label)
			}
			seen[candidate.Label] = true
		}
	}
}

func TestExpandedPathsAreUniquePerPlatform(t *testing.T) {
	vars := PathVars{
		Home:      "/Users/Test User",
		Workspace: "/Users/Test User/project",
	}

	for _, platform := range []string{"darwin", "linux"} {
		for _, client := range ForPlatform(AllClients(), platform) {
			seen := make(map[string]string)
			for _, candidate := range client.Candidates {
				path, err := ExpandPath(candidate.PathTemplate, vars)
				if err != nil {
					t.Fatalf("%s/%s ExpandPath returned error: %v", client.ID, candidate.Label, err)
				}
				if prior, ok := seen[path]; ok {
					t.Fatalf("%s on %s has duplicate expanded path %q for %s and %s", client.ID, platform, path, prior, candidate.Label)
				}
				seen[path] = candidate.Label
			}
		}
	}
}

func TestRuntimeRequirementsCoverExpectedIDs(t *testing.T) {
	runtimeIDs := make([]string, 0, len(AllRuntimeRequirements()))
	for _, runtime := range AllRuntimeRequirements() {
		runtimeIDs = append(runtimeIDs, runtime.ID)
	}

	for _, requiredID := range []string{"node", "npx", "docker", "claude", "codex", "antigravity"} {
		if !slices.Contains(runtimeIDs, requiredID) {
			t.Fatalf("missing runtime requirement %q", requiredID)
		}
	}
}

func TestManifestPackageHasNoInternalImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s) returned error: %v", entry.Name(), err)
		}
		if strings.Contains(string(data), "github.com/nawodyaishan/universal-mcp-sync/pkg/") {
			t.Fatalf("%s contains an internal package import", entry.Name())
		}
	}
}

func findClient(clients []ClientManifest, id ClientID) (ClientManifest, bool) {
	for _, client := range clients {
		if client.ID == id {
			return client, true
		}
	}
	return ClientManifest{}, false
}

func TestAllSourceRefsHaveConfidence(t *testing.T) {
	for _, client := range AllClients() {
		for i, src := range client.Sources {
			if src.Confidence == "" {
				t.Errorf("client %s Sources[%d] URL=%q missing Confidence", client.ID, i, src.URL)
			}
		}
	}
	for _, prov := range AllProviders() {
		for i, src := range prov.Sources {
			if src.Confidence == "" {
				t.Errorf("provider %s Sources[%d] URL=%q missing Confidence", prov.ID, i, src.URL)
			}
		}
	}
}
