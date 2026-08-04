package manifest

type ClientID string
type ProviderID string
type ConfigFormat string
type MutationKind string
type ScopeKind string
type ManagerKind string
type Confidence string

const (
	ClientClaudeDesktop  ClientID = "claude-desktop"
	ClientClaudeCode     ClientID = "claude-code"
	ClientCursor         ClientID = "cursor"
	ClientVSCode         ClientID = "vscode"
	ClientWindsurf       ClientID = "windsurf"
	ClientZed            ClientID = "zed"
	ClientRooCode        ClientID = "roocode"
	ClientOpenCode       ClientID = "opencode"
	ClientKiro           ClientID = "kiro"
	ClientAntigravityCLI ClientID = "antigravity-cli"
	ClientAntigravity    ClientID = "antigravity"
	ClientCodexCLI       ClientID = "codex-cli"
)

const (
	ProviderExa        ProviderID = "exa"
	ProviderGitHub     ProviderID = "github"
	ProviderContext7   ProviderID = "context7"
	ProviderTavily     ProviderID = "tavily"
	ProviderPlaywright ProviderID = "playwright"
	ProviderKubernetes ProviderID = "kubernetes"
	ProviderTerraform  ProviderID = "terraform"
	ProviderCodeGraph  ProviderID = "codegraph"
)

const (
	FormatJSON  ConfigFormat = "json"
	FormatJSONC ConfigFormat = "jsonc"
	FormatTOML  ConfigFormat = "toml"
)

const (
	MutationMCPServers     MutationKind = "mcpServers"
	MutationBareMCPServers MutationKind = "bareMCPServers"
	MutationNamedServer    MutationKind = "namedServer"
	MutationCodexTOML      MutationKind = "codexTOML"
	MutationClaudeCodeCLI  MutationKind = "claudeCodeCLI"
	MutationCodexCLI       MutationKind = "codexCLI" // user-scope codex mcp add CLI path
)

const (
	ScopeUser      ScopeKind = "user"
	ScopeProject   ScopeKind = "project"
	ScopeWorkspace ScopeKind = "workspace"
	ScopeGlobal    ScopeKind = "global"
	ScopeLegacy    ScopeKind = "legacy"
	ScopeManaged   ScopeKind = "managed"
)

const (
	ManagerFile ManagerKind = "file"
	ManagerCLI  ManagerKind = "cli"
)

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type ClientManifest struct {
	ID         ClientID
	Name       string
	Platforms  []string
	Candidates []ConfigCandidate
	Manager    ManagerKind
	CLIName    string
	DocsURL    string
	Warnings   []ClientWarning
	Sources    []SourceRef
}

type ConfigCandidate struct {
	Label           string
	PathTemplate    string
	Platforms       []string
	Scope           ScopeKind
	Format          ConfigFormat
	MutationKind    MutationKind
	RootKey         string
	URLField        string
	Creatable       bool
	Confidence      Confidence
	Precedence      int
	Deprecated      bool
	ReplacedBy      string
	DeprecationNote string
	SymlinkHint     bool
	GitWarning      bool
}

type ClientWarning struct {
	Code     string
	Message  string
	Deadline string
}

type SourceRef struct {
	URL        string
	Title      string
	VerifiedAt string
	Confidence string // "official" (primary docs/source) | "empirical" (community/issue-tracker observation)
}

type ProviderMeta struct {
	ID          ProviderID
	Name        string
	DocsURL     string
	Credentials []CredentialAcquisition
	RuntimeIDs  []string
	Sources     []SourceRef
}

type CredentialAcquisition struct {
	Key             string
	EnvVar          string
	Required        bool
	FormatHint      string
	OfflineRegex    string
	GetURL          string
	DocsURL         string
	LiveValidation  *LiveValidationSpec
	DeprecationNote string
}

type LiveValidationSpec struct {
	Method     string
	URL        string
	AuthHeader string
	QuotaSafe  bool
	QuotaNote  string
}

type RuntimeRequirement struct {
	ID          string
	Name        string
	Command     string
	Args        []string
	InstallURL  string
	RequiredFor []string
}

type PathVars struct {
	Home      string
	Workspace string
}
