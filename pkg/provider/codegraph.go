package provider

// CodeGraphProvider installs the CodeGraph MCP server via npx.
// CodeGraph provides semantic code intelligence — symbol maps, call graphs,
// and cross-file dependency trees — enabling agents to answer architecture
// questions with surgical context in a single tool call instead of dozens of
// file-by-file reads.
//
// Reference: https://github.com/colbymchenry/codegraph
// Install:   curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh
// Per-project init: cd your-project && codegraph init
type CodeGraphProvider struct{}

func NewCodeGraphProvider() *CodeGraphProvider { return &CodeGraphProvider{} }

func (p *CodeGraphProvider) ID() string   { return "codegraph" }
func (p *CodeGraphProvider) Name() string { return "CodeGraph" }
func (p *CodeGraphProvider) Description() string {
	return "Semantic code intelligence — call graphs, symbol maps, and cross-file dependency trees for AI agents. 100% local, no data leaves the machine."
}

// RequiredCredentials returns nil because CodeGraph is fully local and
// requires no API keys or tokens.
func (p *CodeGraphProvider) RequiredCredentials() []CredentialSpec {
	return nil
}

// GenerateConfig returns a stdio transport config that launches the CodeGraph
// MCP server via npx. The server connects to the local .codegraph/ index
// produced by `codegraph init` in the project root.
func (p *CodeGraphProvider) GenerateConfig(_ map[string]string) (MCPConfig, error) {
	return MCPConfig{
		Type:    TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "@colbymchenry/codegraph", "mcp"},
		Runtime: &PackageRuntime{Type: "npm"},
	}, nil
}
