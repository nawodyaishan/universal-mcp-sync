# Add Context7 Provider + Contributor Docs + In-Repo AI Skill

**Last updated:** 2026-05-10
**Status:** Approved
**Audience:** Implementation engineers and AI agents

---

## How to use this document

Each task (T-A1, T-B1...) is independently executable — pick one up without reading the whole doc. Work phases in order (A→B→C→D); Phase E and Phase F can run in parallel after Phase D merges. Run `make test && make lint` after every task. See the file change index at the bottom for the full picture.

---

## Context

Why this change: usync currently supports a single MCP provider (Exa). Context7 is the highest-value second provider for contributors working on this repo — it gives Claude Code, Cursor, and the other 10 supported AI clients live access to library documentation (Bubbletea, Huh, MCP spec). Shipping Context7 alongside Exa turns usync into a tool contributors can use on themselves.

Intended outcome:
- A working Context7 provider with full client-fleet support
- Docs that reference Context7 as the "header-auth" reference and Exa as the "URL-auth + multi-key" reference
- A `.claude/skills/add-provider` skill that walks any future contributor (human or agent) through the exact sequence used to add Context7

**Critical discovery:** Context7 authenticates via the HTTP header `CONTEXT7_API_KEY` — not a URL query parameter. The current `provider.MCPConfig` struct has **no `Headers` field**. Neither JSON writer nor TOML writer emits headers. This is a real architectural gap that must be closed before Context7 can ship.

**Existing infrastructure to reuse (already merged):**
- `pkg/client/adapter.go` — `Adapt()` and `CanHandle()` already wired into `pkg/app/app.go:157, 165`. Add `HeadersFor()` here.
- `pkg/client/capabilities.go` — capability Matrix already covers all 12 clients. No schema changes needed for v1.
- `pkg/redact/redact.go` — already redacts UUID-shaped strings. Extend with `ctx7sk_*` regex.
- `pkg/exa/keys.go`, `pkg/exa/url.go` — pattern to mirror for `pkg/context7/`.
- `pkg/provider/exa.go` — pattern to mirror for `pkg/provider/context7.go`.

---

## Architecture decision

### Decision: `MCPConfig.Headers` + per-client augmentation via `client.HeadersFor`

**Why:** Context7 uses HTTP header auth. Exa uses URL query string auth. Both are valid MCP patterns. Rather than making the provider responsible for per-client header variations (e.g., Gemini CLI needs an extra `Accept` header), we add `Headers` to `MCPConfig` as the provider's intent and let `client.HeadersFor()` apply per-client augmentation — consistent with how `client.Adapt()` already handles per-client transport transforms.

**What it replaces:** No existing code handles headers at all. This is purely additive.

**Claude Desktop special case:** `mcp-remote` (the existing bridge) does not reliably forward custom HTTP headers. Context7 ships its own stdio binary (`@upstash/context7-mcp`). For Claude Desktop, we extend the bridge mechanism in `pkg/client` to support `{header:KEY}` placeholder substitution, allowing a provider-specific bridge override to emit `npx -y @upstash/context7-mcp --api-key <key>` instead of the default `npx -y mcp-remote <url>`.

**Trade-offs accepted:**
- Pro: Zero Exa regressions — `len(cfg.Headers) > 0` guard ensures empty headers never serialized
- Pro: Gemini's extra `Accept` header is in `pkg/client` (right layer), not in the provider
- Con: T-B3 introduces a `{header:KEY}` bridge substitution pattern that is new — needs to be documented clearly
- Rejected alternative: provider-ID-keyed `ProviderBridges` map in capabilities.go — too much complexity for v1; the `BridgeOverride` field on `MCPConfig` achieves the same with less abstraction

---

## Dependency graph

```
Phase A (Headers capability)
 ├─ T-A1  MCPConfig.Headers field
 ├─ T-A2  buildConfigMap emits "headers"
 ├─ T-A3  UpdateCodexTOML emits http_headers
 └─ T-A4  verify checks headers
     └─ Phase B (provider)
         ├─ T-B1  pkg/context7/ helpers
         ├─ T-B2  pkg/provider/context7.go
         ├─ T-B3  Claude Desktop bridge override
         ├─ T-B4  register in registry
         └─ T-B5  redact ctx7sk_*
             └─ Phase C (per-client adaptation)
                 ├─ T-C1  client.HeadersFor()
                 └─ T-C2  wire into prepareFileOperation
                     └─ Phase D (QA)
                         ├─ T-D1  TestQAContext7AllClients
                         ├─ T-D2  TestQAExaAndContext7Coexist
                         └─ T-D3  Context7 idempotency
                             ├─ Phase E (docs) — parallel
                             └─ Phase F (skill) — parallel
```

---

## Phase A — Close the `Headers` capability gap

No behavior change for Exa. All three writers are guarded by `len(cfg.Headers) > 0`.

---

### T-A1 — Add `Headers` field to `MCPConfig`
<!-- execution: haiku -->

**Phase:** A
**Depends on:** none
**Blocks:** T-A2, T-A3, T-A4, T-B2
**Files:** `pkg/provider/types.go` — modified

#### What to change

Read `pkg/provider/types.go` first to confirm current line numbers. The `MCPConfig` struct is near line 12.

```go
// BEFORE
type MCPConfig struct {
    Type    TransportType
    URL     string
    Command string
    Args    []string
    Env     map[string]string
    Runtime *PackageRuntime
}

// AFTER
type MCPConfig struct {
    Type    TransportType
    URL     string
    Command string
    Args    []string
    Env     map[string]string
    Headers map[string]string // Per-server HTTP headers for remote transports. Nil for stdio.
    Runtime *PackageRuntime
}
```

#### Acceptance

- [ ] `go build ./...` passes
- [ ] `make test` passes (all Exa tests still green)
- [ ] `grep -n "Headers" pkg/provider/types.go` shows one field declaration

---

### T-A2 — Extend `buildConfigMap` to emit `"headers"` only when non-empty
<!-- execution: sonnet -->

**Phase:** A
**Depends on:** T-A1
**Blocks:** T-D1
**Files:** `pkg/config/json_update.go` — modified

#### What to change

**`pkg/config/json_update.go:11-34` (`buildConfigMap`)** — after the URL assignment, before the `extra` merge:

```go
// BEFORE (lines 11–34 approximately)
func buildConfigMap(cfg provider.MCPConfig, urlFieldName string, extra map[string]any) map[string]any {
    result := make(map[string]any)
    if cfg.Type == provider.TransportStdio {
        result["command"] = cfg.Command
        if len(cfg.Args) > 0 {
            result["args"] = cfg.Args
        }
        if len(cfg.Env) > 0 {
            result["env"] = cfg.Env
        }
    } else {
        if urlFieldName != "" {
            result[urlFieldName] = cfg.URL
        } else {
            result["url"] = cfg.URL
        }
    }

    for k, v := range extra {
        result[k] = v
    }
    return result
}

// AFTER — add headers block inside the else branch, before extra merge
func buildConfigMap(cfg provider.MCPConfig, urlFieldName string, extra map[string]any) map[string]any {
    result := make(map[string]any)
    if cfg.Type == provider.TransportStdio {
        result["command"] = cfg.Command
        if len(cfg.Args) > 0 {
            result["args"] = cfg.Args
        }
        if len(cfg.Env) > 0 {
            result["env"] = cfg.Env
        }
    } else {
        if urlFieldName != "" {
            result[urlFieldName] = cfg.URL
        } else {
            result["url"] = cfg.URL
        }
        if len(cfg.Headers) > 0 {
            headers := make(map[string]string, len(cfg.Headers))
            for k, v := range cfg.Headers {
                headers[k] = v
            }
            result["headers"] = headers
        }
    }

    for k, v := range extra {
        result[k] = v
    }
    return result
}
```

Also apply the same `len(cfg.Headers) > 0` guard in `UpdateNamedServerJSON` (lines 63–95) for the non-stdio branch: after updating `server[urlFieldName]`, merge `cfg.Headers` into the server map when non-empty.

#### Tests to add

**`pkg/config/json_update_test.go`:**

```go
func TestBuildConfigMap_EmitsHeadersWhenPresent(t *testing.T) {
    cfg := provider.MCPConfig{
        Type:    provider.TransportStreamableHTTP,
        URL:     "https://mcp.context7.com/mcp",
        Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"},
    }
    result, _ := config.UpdateMCPServersJSON([]byte("{}"), "context7", "mcpServers", "url", cfg, nil)
    if !bytes.Contains(result, []byte(`"headers"`)) {
        t.Errorf("expected headers in output:\n%s", result)
    }
    if !bytes.Contains(result, []byte(`"CONTEXT7_API_KEY"`)) {
        t.Errorf("expected header key in output:\n%s", result)
    }
}

func TestBuildConfigMap_NoHeadersForExa(t *testing.T) {
    cfg := provider.MCPConfig{
        Type: provider.TransportHTTP,
        URL:  "https://mcp.exa.ai/mcp?exaApiKey=test",
    }
    result, _ := config.UpdateMCPServersJSON([]byte("{}"), "exa", "mcpServers", "url", cfg, nil)
    if bytes.Contains(result, []byte(`"headers"`)) {
        t.Errorf("Exa output must not contain headers:\n%s", result)
    }
}
```

#### Acceptance

- [ ] `make test` passes
- [ ] Snapshot diff of an Exa-only fixture is zero bytes

---

### T-A3 — Extend `UpdateCodexTOML` to emit `http_headers`
<!-- execution: sonnet -->

**Phase:** A
**Depends on:** T-A1
**Blocks:** T-D1
**Files:** `pkg/config/toml_update.go` — modified

#### What to change

Add a `sortedKeys` helper in the same file, then extend the `block` construction in `UpdateCodexTOML`:

```go
// BEFORE (lines 15–18 approximately)
block := []string{
    fmt.Sprintf("[mcp_servers.%s]", providerID),
    fmt.Sprintf("url = %q", cfg.URL),
}

// AFTER
block := []string{
    fmt.Sprintf("[mcp_servers.%s]", providerID),
    fmt.Sprintf("url = %q", cfg.URL),
}
if len(cfg.Headers) > 0 {
    pairs := make([]string, 0, len(cfg.Headers))
    for _, k := range sortedKeys(cfg.Headers) {
        pairs = append(pairs, fmt.Sprintf("%q = %q", k, cfg.Headers[k]))
    }
    block = append(block, fmt.Sprintf("http_headers = { %s }", strings.Join(pairs, ", ")))
}
```

Add helper (after the `isSectionHeader` function):

```go
func sortedKeys(m map[string]string) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}
```

Add `"sort"` to the import block.

**Expected output for Context7:**
```toml
[mcp_servers.context7]
url = "https://mcp.context7.com/mcp"
http_headers = { "CONTEXT7_API_KEY" = "ctx7sk_..." }
```

#### Tests to add

```go
func TestUpdateCodexTOML_WritesHttpHeaders(t *testing.T) {
    cfg := provider.MCPConfig{
        Type:    provider.TransportStreamableHTTP,
        URL:     "https://mcp.context7.com/mcp",
        Headers: map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"},
    }
    result, err := config.UpdateCodexTOML([]byte(""), "context7", cfg)
    if err != nil { t.Fatal(err) }
    if !bytes.Contains(result, []byte(`http_headers`)) {
        t.Errorf("expected http_headers in TOML:\n%s", result)
    }
    if !bytes.Contains(result, []byte(`"CONTEXT7_API_KEY"`)) {
        t.Errorf("expected header key:\n%s", result)
    }
}
```

#### Acceptance

- [ ] `go test ./pkg/config/...` passes
- [ ] The existing Exa Codex TOML test passes unchanged (no `http_headers` line)

---

### T-A4 — Add Context7 verification branch
<!-- execution: sonnet -->

**Phase:** A
**Depends on:** T-A1
**Blocks:** T-D1
**Files:** `pkg/verify/verify.go` — modified

#### What to change

**`pkg/verify/verify.go:51-56`** — add a sibling branch to the existing Exa dispatch:

```go
// BEFORE
func VerifyProviderFile(path string, kind config.FileKind, providerID string, cfg provider.MCPConfig) Result {
    if providerID == "exa" {
        return verifyExaProviderFile(path, kind, cfg)
    }
    return verifyGenericProviderFile(path, kind, providerID, cfg)
}

// AFTER
func VerifyProviderFile(path string, kind config.FileKind, providerID string, cfg provider.MCPConfig) Result {
    switch providerID {
    case "exa":
        return verifyExaProviderFile(path, kind, cfg)
    case "context7":
        return verifyContext7ProviderFile(path, kind, cfg)
    default:
        return verifyGenericProviderFile(path, kind, providerID, cfg)
    }
}
```

Add new function after `verifyExaProviderFile`:

```go
func verifyContext7ProviderFile(path string, kind config.FileKind, cfg provider.MCPConfig) Result {
    switch kind {
    case config.FileKindMCPServers:
        return verifyContext7MCPServersFile(path, cfg)
    case config.FileKindBareMCPServers:
        return verifyContext7BareMCPServersFile(path, cfg)
    case config.FileKindNamedServer:
        return verifyContext7NamedServerFile(path, cfg)
    case config.FileKindCodexTOML:
        return verifyContext7CodexFile(path)
    default:
        return failure(path, "unsupported verification target for context7")
    }
}

func verifyContext7MCPServersFile(path string, cfg provider.MCPConfig) Result {
    if cfg.Type == provider.TransportStdio {
        server, err := readNestedServerEntry(path, "mcpServers", "context7")
        if err != nil { return failure(path, err.Error()) }
        details, ok := inspectStdioServer(server)
        return resultFrom(path, details, ok)
    }
    server, err := readNestedServerEntry(path, "mcpServers", "context7")
    if err != nil { return failure(path, err.Error()) }
    return inspectContext7Server(path, server)
}

func inspectContext7Server(path string, server map[string]any) Result {
    urlValue := getURLField(server)
    if urlValue == "" {
        return failure(path, "missing context7 URL field")
    }
    if !strings.Contains(urlValue, "mcp.context7.com") {
        return Result{Target: path, Status: StatusWarning,
            Details: []string{fmt.Sprintf("unexpected Context7 endpoint: %s", urlValue)}}
    }
    headers, _ := server["headers"].(map[string]any)
    if _, ok := headers["CONTEXT7_API_KEY"]; !ok {
        return failure(path, "missing CONTEXT7_API_KEY in headers")
    }
    return Result{
        Target:  path,
        Status:  StatusOK,
        Details: []string{"url present", "headers present: CONTEXT7_API_KEY"},
    }
}

func verifyContext7CodexFile(path string) Result {
    data, err := os.ReadFile(path)
    if err != nil { return failure(path, err.Error()) }
    text := string(data)
    if !strings.Contains(text, "[mcp_servers.context7]") {
        return failure(path, "missing [mcp_servers.context7] block")
    }
    if !strings.Contains(text, "http_headers") {
        return failure(path, "missing http_headers in context7 TOML block")
    }
    return Result{Target: path, Status: StatusOK,
        Details: []string{"block present", "http_headers present"}}
}
```

Add helper `resultFrom`:

```go
func resultFrom(path string, details []string, ok bool) Result {
    status := StatusOK
    if !ok { status = StatusFailed }
    return Result{Target: path, Status: status, Details: details}
}
```

(Add stubs for the Bare and Named variants following the same pattern as Exa.)

#### Tests to add

```go
func TestVerifyContext7File_HeadersPresent(t *testing.T) { ... }
func TestVerifyContext7File_HeadersMissing(t *testing.T) { ... }
func TestVerifyContext7CodexFile_HttpHeadersPresent(t *testing.T) { ... }
```

#### Acceptance

- [ ] `go test ./pkg/verify/...` passes
- [ ] `"headers present: CONTEXT7_API_KEY"` appears in result details — never the key value

---

## Phase B — Context7 provider

---

### T-B1 — Create `pkg/context7/` helpers
<!-- execution: sonnet -->

**Phase:** B
**Depends on:** T-A1
**Blocks:** T-B2
**Files:** `pkg/context7/keys.go`, `pkg/context7/url.go`, `pkg/context7/keys_test.go`, `pkg/context7/url_test.go` — all new

#### What to create

**`pkg/context7/url.go`:**
```go
package context7

const (
    Endpoint   = "https://mcp.context7.com/mcp"
    HeaderName = "CONTEXT7_API_KEY"
)
```

**`pkg/context7/keys.go`:**
```go
package context7

import (
    "fmt"
    "strings"
)

const keyPrefix = "ctx7sk_"
const minKeyLen = len(keyPrefix) + 8

// ParseKey validates a Context7 API key.
// Valid keys start with "ctx7sk_" and have sufficient length.
func ParseKey(key string) (string, error) {
    key = strings.TrimSpace(key)
    if !strings.HasPrefix(key, keyPrefix) {
        return "", fmt.Errorf("Context7 API key must start with %q", keyPrefix)
    }
    if len(key) < minKeyLen {
        return "", fmt.Errorf("Context7 API key is too short")
    }
    return key, nil
}

// RedactKey masks a Context7 API key for display.
// e.g. "ctx7sk_abcdef1234567890wxyz" → "ctx7sk_abcd...wxyz"
func RedactKey(key string) string {
    if len(key) <= len(keyPrefix)+8 {
        return key
    }
    suffix := key[len(keyPrefix):]
    if len(suffix) <= 8 {
        return key
    }
    return keyPrefix + suffix[:4] + "..." + suffix[len(suffix)-4:]
}
```

#### Tests to add

```go
func TestParseKey_Valid(t *testing.T) { ... }
func TestParseKey_MissingPrefix(t *testing.T) { ... }
func TestParseKey_TooShort(t *testing.T) { ... }
func TestRedactKey(t *testing.T) { ... }
```

#### Acceptance

- [ ] `go test ./pkg/context7/...` passes

---

### T-B2 — Implement `Context7Provider`
<!-- execution: opus -->

**Phase:** B
**Depends on:** T-B1
**Blocks:** T-B3, T-B4
**Files:** `pkg/provider/context7.go`, `pkg/provider/context7_test.go` — both new

#### What to create

**`pkg/provider/context7.go`:**
```go
package provider

import (
    "fmt"

    "github.com/nawodyaishan/universal-mcp-sync/pkg/context7"
)

type Context7Provider struct{}

func NewContext7Provider() *Context7Provider { return &Context7Provider{} }

func (p *Context7Provider) ID() string          { return "context7" }
func (p *Context7Provider) Name() string        { return "Context7" }
func (p *Context7Provider) Description() string {
    return "Up-to-date library documentation and code examples for AI agents."
}

func (p *Context7Provider) RequiredCredentials() []CredentialSpec {
    return []CredentialSpec{
        {
            Key:         "CONTEXT7_API_KEY",
            Label:       "Context7 API Key",
            Description: "Get your key at context7.com/dashboard. Format: ctx7sk_...",
            Secret:      true,
            MultiValue:  false,
            Validator: func(s string) error {
                _, err := context7.ParseKey(s)
                return err
            },
        },
    }
}

func (p *Context7Provider) GenerateConfig(credentials map[string]string) (MCPConfig, error) {
    key := credentials["CONTEXT7_API_KEY"]
    if _, err := context7.ParseKey(key); err != nil {
        return MCPConfig{}, fmt.Errorf("invalid Context7 API key: %w", err)
    }
    return MCPConfig{
        Type:    TransportStreamableHTTP,
        URL:     context7.Endpoint,
        Headers: map[string]string{context7.HeaderName: key},
        // BridgeOverride is set by client.Adapt for Claude Desktop (stdio path).
    }, nil
}
```

Note: `MultiValueParser` is NOT implemented — Context7 uses one key per profile. The `syncToContext()` generic fallback in `pkg/tui/setup_form.go` handles this correctly without changes.

#### Tests to add

```go
func TestContext7Provider_ID(t *testing.T) { ... }
func TestContext7Provider_GenerateConfig_Valid(t *testing.T) { ... }
func TestContext7Provider_GenerateConfig_InvalidKey(t *testing.T) { ... }
func TestContext7Provider_CredentialValidator(t *testing.T) { ... }
```

Assert: `GenerateConfig` returns `Type=TransportStreamableHTTP`, `URL=context7.Endpoint`, `Headers["CONTEXT7_API_KEY"]=key`.

#### Acceptance

- [ ] `go test ./pkg/provider/...` passes
- [ ] `Context7Provider` satisfies `MCPProvider` interface (compile check)

---

### T-B3 — Extend bridge mechanism for Claude Desktop stdio
<!-- execution: opus -->

**Phase:** B
**Depends on:** T-B2
**Blocks:** T-D1
**Files:** `pkg/client/adapter.go` — modified, `pkg/client/capabilities.go` — modified

#### What to change

The current `applyBridge` substitutes `{url}` in args. Context7's Claude Desktop bridge needs `{header:CONTEXT7_API_KEY}` — the value of a specific header — substituted instead of the URL.

**Approach: `BridgeOverride` field on `MCPConfig`** — kept in the provider layer, not the client layer.

**Step 1 — Add `BridgeOverride` to `MCPConfig`** (`pkg/provider/types.go`):
```go
type MCPConfig struct {
    // ... existing fields ...
    // BridgeOverride, when non-nil, is used by client.Adapt in place of the
    // Matrix bridge for this specific provider+client combination.
    BridgeOverride *BridgeConfig
}
```

**Step 2 — `BridgeConfig` needs access to `MCPConfig` for header extraction.** Extend `applyBridge`:
```go
func applyBridge(bridge *BridgeConfig, cfg provider.MCPConfig) provider.MCPConfig {
    args := make([]string, len(bridge.Args))
    for i, arg := range bridge.Args {
        // Substitute {url}
        arg = strings.ReplaceAll(arg, "{url}", cfg.URL)
        // Substitute {header:KEY} with the header value
        for k, v := range cfg.Headers {
            arg = strings.ReplaceAll(arg, "{header:"+k+"}", v)
        }
        args[i] = arg
    }
    return provider.MCPConfig{
        Type:    provider.TransportStdio,
        Command: bridge.Command,
        Args:    args,
    }
}
```

**Step 3 — `Adapt()` checks `BridgeOverride` first:**
```go
func Adapt(appID config.AppID, cfg provider.MCPConfig) provider.MCPConfig {
    // Provider-specified override takes priority over matrix bridge
    if cfg.BridgeOverride != nil {
        cap, ok := Matrix[appID]
        if ok && !supportsTransport(cap.Supports, cfg.Type) {
            return applyBridge(cfg.BridgeOverride, cfg)
        }
    }
    // ... rest of existing Adapt logic ...
}
```

**Step 4 — `Context7Provider.GenerateConfig` sets the override:**
```go
return MCPConfig{
    Type:    TransportStreamableHTTP,
    URL:     context7.Endpoint,
    Headers: map[string]string{context7.HeaderName: key},
    BridgeOverride: &provider.BridgeConfig{
        Command: "npx",
        Args:    []string{"-y", "@upstash/context7-mcp", "--api-key", "{header:CONTEXT7_API_KEY}"},
    },
}, nil
```

**Note:** `BridgeConfig` must be exported from `pkg/client` or moved to `pkg/provider/types.go`. Since providers set it, move the `BridgeConfig` struct definition to `pkg/provider/types.go` and import it in `pkg/client`.

#### Acceptance

- [ ] `go test ./pkg/client/...` passes
- [ ] `TestAdapt_Context7ClaudeDesktop_UsesBridgeOverride` passes — output is `command: npx, args: [-y @upstash/context7-mcp --api-key ctx7sk_...]`
- [ ] `TestAdapt_ExaCaudeDesktop_StillUsesMcpRemote` passes — Exa bridge unchanged

---

### T-B4 — Register in registry
<!-- execution: haiku -->

**Phase:** B
**Depends on:** T-B2
**Blocks:** T-D1
**Files:** `pkg/provider/registry.go` — modified

#### What to change

```go
// BEFORE
func DefaultRegistry() Registry {
    r := Registry{providers: make(map[string]MCPProvider), order: []string{}}
    r.register(NewExaProvider())
    return r
}

// AFTER
func DefaultRegistry() Registry {
    r := Registry{providers: make(map[string]MCPProvider), order: []string{}}
    r.register(NewExaProvider())
    r.register(NewContext7Provider())
    return r
}
```

Update `pkg/provider/registry_test.go` to assert the registry returns `["exa", "context7"]` in that order.

#### Acceptance

- [ ] `go test ./pkg/provider/...` passes

---

### T-B5 — Add Context7 key redaction
<!-- execution: haiku -->

**Phase:** B
**Depends on:** none (parallel with B1–B4)
**Files:** `pkg/redact/redact.go`, `pkg/redact/redact_test.go` — modified

#### What to change

Add a second regex to `pkg/redact/redact.go`:

```go
var ctx7RE = regexp.MustCompile(`ctx7sk_[A-Za-z0-9_\-]{8,}`)

func Text(s string) string {
    s = uuidRE.ReplaceAllStringFunc(s, Key)
    s = ctx7RE.ReplaceAllStringFunc(s, func(key string) string {
        return context7.RedactKey(key)
    })
    return s
}
```

Import `"github.com/nawodyaishan/universal-mcp-sync/pkg/context7"`.

#### Tests to add

```go
func TestText_RedactsContext7Key(t *testing.T) {
    key := "ctx7sk_abcdef1234567890wxyz"
    got := redact.Text("config key: " + key)
    if strings.Contains(got, key) {
        t.Errorf("full key must not appear in output: %s", got)
    }
}
```

#### Acceptance

- [ ] `go test ./pkg/redact/...` passes

---

## Phase C — Per-client header adaptation

---

### T-C1 — Add `client.HeadersFor`
<!-- execution: sonnet -->

**Phase:** C
**Depends on:** T-A1
**Blocks:** T-C2
**Files:** `pkg/client/adapter.go` — modified, `pkg/client/adapter_test.go` — modified

#### What to change

Append to `pkg/client/adapter.go`:

```go
// HeadersFor returns the headers map to write for appID.
// Gemini CLI requires an extra Accept header for SSE streaming.
// Returns nil when base is empty (prevents serializing "headers": {}).
func HeadersFor(appID config.AppID, base map[string]string) map[string]string {
    if len(base) == 0 {
        return nil
    }
    out := make(map[string]string, len(base)+1)
    for k, v := range base {
        out[k] = v
    }
    if appID == config.AppGeminiCLI {
        out["Accept"] = "application/json, text/event-stream"
    }
    return out
}
```

#### Tests to add

```go
func TestHeadersFor_GeminiAddsAccept(t *testing.T) {
    base := map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"}
    got := client.HeadersFor(config.AppGeminiCLI, base)
    if got["Accept"] == "" {
        t.Error("expected Accept header for Gemini CLI")
    }
}

func TestHeadersFor_NilBaseReturnsNil(t *testing.T) {
    if client.HeadersFor(config.AppCursor, nil) != nil {
        t.Error("nil base must return nil (no empty headers map)")
    }
}

func TestHeadersFor_CursorUnchanged(t *testing.T) {
    base := map[string]string{"CONTEXT7_API_KEY": "ctx7sk_test"}
    got := client.HeadersFor(config.AppCursor, base)
    if _, ok := got["Accept"]; ok {
        t.Error("Cursor must not gain Accept header")
    }
}
```

#### Acceptance

- [ ] `go test ./pkg/client/...` passes

---

### T-C2 — Wire `HeadersFor` into `prepareFileOperation`
<!-- execution: sonnet -->

**Phase:** C
**Depends on:** T-C1
**Blocks:** T-D1
**Files:** `pkg/app/app.go` — modified

#### What to change

**`pkg/app/app.go:464`** — at the start of `prepareFileOperation`, before the `switch op.Kind`:

```go
// BEFORE
func (m *Manager) prepareFileOperation(op Operation) (preparedWrite, error) {
    if err := validatePathWithinHome(m.HomeDir, op.Path); err != nil {
        return preparedWrite{}, fmt.Errorf("%s (%s): %w", op.AppName, op.FileLabel, err)
    }

    data, _, err := config.ReadFileOrEmpty(op.Path)
    // ...

// AFTER — add one line after ReadFileOrEmpty
    data, _, err := config.ReadFileOrEmpty(op.Path)
    if err != nil {
        return preparedWrite{}, err
    }

    op.Config.Headers = client.HeadersFor(op.AppID, op.Config.Headers) // augment per-client

    var updated []byte
    switch op.Kind {
```

Add `"github.com/nawodyaishan/universal-mcp-sync/pkg/client"` to the import block if not already present (it is — already imported at line 17).

#### Acceptance

- [ ] `make test` passes
- [ ] Gemini CLI fixture in QA tests gets `Accept` header (T-D1 assertion)

---

## Phase D — QA scenarios

---

### T-D1 — `TestQAContext7AllClients`
<!-- execution: sonnet -->

**Phase:** D
**Depends on:** T-C2, T-B4
**Files:** `pkg/app/qa_scenarios_test.go` — modified

#### What to add

```go
func TestQAContext7AllClients(t *testing.T) {
    homeDir := t.TempDir()

    // Write empty config files for all clients
    paths := map[config.AppID]string{
        config.AppClaudeDesktop: filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
        config.AppCursor:        filepath.Join(homeDir, ".cursor", "mcp.json"),
        config.AppVSCode:        filepath.Join(homeDir, ".vscode", "mcp.json"),
        config.AppWindsurf:      filepath.Join(homeDir, ".codeium", "windsurf", "mcp_config.json"),
        config.AppZed:           filepath.Join(homeDir, ".config", "zed", "settings.json"),
        config.AppRooCode:       filepath.Join(homeDir, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "mcp_settings.json"),
        config.AppOpenCode:      filepath.Join(homeDir, ".opencode.json"),
        config.AppKiro:          filepath.Join(homeDir, ".kiro", "settings", "mcp.json"),
        config.AppGeminiCLI:     filepath.Join(homeDir, ".gemini", "settings.json"),
        config.AppAntigravity:   filepath.Join(homeDir, ".gemini", "antigravity", "mcp_config.json"),
        config.AppCodexCLI:      filepath.Join(homeDir, ".codex", "config.toml"),
    }
    for _, p := range paths {
        mustWriteFile(t, p, []byte("{}"))
    }
    mustWriteFile(t, paths[config.AppCodexCLI], []byte(""))

    manager, _ := NewManager(homeDir, fixedNow(), fakeRunner{available: map[string]bool{"claude": true}})

    prov := provider.NewContext7Provider()
    key := "ctx7sk_" + strings.Repeat("a", 20)
    profiles := []provider.CredentialProfile{{
        ProviderID: "context7",
        Values:     map[string]string{"CONTEXT7_API_KEY": key},
        Label:      "ctx7sk_aaaa...aaaa",
    }}
    selected := make(map[config.AppID]bool)
    for id := range paths {
        selected[id] = true
    }
    assignments := DefaultAssignments(selected, 1)

    plan, err := manager.PrepareProvider(prov, profiles, selected, assignments)
    if err != nil { t.Fatalf("PrepareProvider: %v", err) }

    // Raw key must never appear in plan
    planText := FormatPlan(plan)
    if strings.Contains(planText, key) {
        t.Errorf("plan output must not contain raw API key")
    }

    _, err = manager.Apply(plan)
    if err != nil { t.Fatalf("Apply: %v", err) }

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

    // Gemini: also has Accept header
    data, _ = os.ReadFile(paths[config.AppGeminiCLI])
    if !bytes.Contains(data, []byte(`"Accept"`)) {
        t.Errorf("Gemini CLI: expected Accept header\n%s", data)
    }

    // Codex: http_headers in TOML
    data, _ = os.ReadFile(paths[config.AppCodexCLI])
    if !bytes.Contains(data, []byte(`http_headers`)) {
        t.Errorf("Codex: expected http_headers in TOML\n%s", data)
    }
}
```

---

### T-D2 — `TestQAExaAndContext7Coexist`
<!-- execution: sonnet -->

**Phase:** D
**Depends on:** T-D1
**Files:** `pkg/app/qa_scenarios_test.go` — modified

Assert both `exa` and `context7` entries present after sequential applies; Exa entry byte-identical to snapshot (no `headers` field added to Exa).

---

### T-D3 — Context7 idempotency
<!-- execution: haiku -->

**Phase:** D
**Depends on:** T-D1
**Files:** `pkg/app/qa_scenarios_test.go` — modified

Run Apply twice with the same Context7 key; assert file bytes identical between runs.

#### Acceptance (all D tasks)

- [ ] `go test -run TestQAContext7AllClients ./pkg/app/...` passes
- [ ] `go test -run TestQAExaAndContext7Coexist ./pkg/app/...` passes
- [ ] Exa fixture snapshot diff is zero bytes after Phase D

---

## Phase E — Documentation overhaul

Phases E and F are independent and can run in parallel after Phase D.

---

### T-E1 — README provider matrix
<!-- execution: sonnet -->

**File:** `README.md` lines 191–223 — modified

Replace the 3-step "Adding a Provider" list with:
- A `## Providers` subsection with a table: Provider | Transport | Auth method | Multi-key | Status
- Rows for Exa, Context7, and a placeholder row for GitHub (from `architecture-upgrade-plan.md` Phase 3B)
- Fix stale reference at line 210: replace `configForTarget` with `pkg/client.Adapt()`

---

### T-E2 — CONTRIBUTING.md rewrite
<!-- execution: sonnet -->

**File:** `CONTRIBUTING.md` lines 8–21 — modified

Replace the inline 4-step list with a pointer to `docs/contributors/adding-a-provider.md`. Fix stale `configForTarget` reference at line 15 (replace with `pkg/client.Adapt()` and `pkg/client.HeadersFor()`).

---

### T-E3 — `docs/contributors/adding-a-provider.md` (NEW)
<!-- execution: sonnet -->

**File:** `docs/contributors/adding-a-provider.md` — new (~400 lines)

Sections:
1. Decision tree (remote/stdio? URL-auth/header-auth? single/multi-key?)
2. Step-by-step using Context7 as example (T-B1→T-B5 with diff snippets)
3. Variant: URL-auth (Exa) — pointer to `pkg/provider/exa.go` and `pkg/exa/url.go`
4. Variant: multi-key paste (Exa) — `MultiValueParser` interface example
5. Per-client adaptation reference table (from Appendix B of this spec)
6. Verification checklist
7. Architectural context — links to `docs/specs/architecture-upgrade-plan.md` and `docs/specs/add-context7-provider.md`

---

### T-E4 — `docs/contributors/dogfooding-with-exa-context7.md` (NEW)
<!-- execution: sonnet -->

**File:** `docs/contributors/dogfooding-with-exa-context7.md` — new (~150 lines)

Walks contributors through using usync on themselves: build, get keys, sync both Exa + Context7, restart tools, example prompts. References `/add-provider` skill for adding new providers.

---

## Phase F — Project-scoped Claude skill

---

### T-F1 — `.claude/skills/add-provider/SKILL.md` (NEW)
<!-- execution: opus -->

**File:** `.claude/skills/add-provider/SKILL.md` — new

```yaml
---
name: add-provider
description: |
  Use when adding a new MCP server provider to usync. Triggers: "add a new
  provider", "add provider X", "scaffold provider", "support <NAME> MCP",
  "register an MCP server". Walks through official doc lookup, scaffolding,
  registration, header dispatch, QA scenarios, and documentation updates.
when_to_use: |
  Trigger on: 'add a provider', 'support <name> MCP server', 'scaffold provider'.
  Do NOT trigger for general Go refactors or test-only changes.
allowed-tools: Read, Grep, Glob, Bash, Edit, Write
---
```

Body (9-step procedure, under 500 lines):
1. **STOP and gather** — read `pkg/provider/types.go`, `pkg/provider/exa.go`, `pkg/provider/context7.go`, `docs/contributors/adding-a-provider.md`, `docs/specs/add-context7-provider.md`
2. **Look up official docs** — use Context7 MCP to query the target server's config schema
3. **Decide capabilities** — use decision tree from `adding-a-provider.md`
4. **Scaffold helpers** at `pkg/<id>/` — use Template 1 from `references/code-templates.md`
5. **Implement provider** at `pkg/provider/<id>.go` — use Template 2
6. **Register** in `pkg/provider/registry.go`
7. **Per-client adaptation** — add to `pkg/client/adapter.go` `HeadersFor` if headers needed
8. **QA scenarios** — use Template 3
9. **Docs** — add row to README provider matrix and `adding-a-provider.md`

End with the 14-item checklist from `references/checklist.md`.

---

### T-F2 — `.claude/skills/add-provider/references/checklist.md` (NEW)
<!-- execution: haiku -->

14-item checklist for PR descriptions covering all 9 steps + 5 verification gates.

---

### T-F3 — `.claude/skills/add-provider/references/code-templates.md` (NEW)
<!-- execution: sonnet -->

Three Go templates with `{{NAME}}`, `{{ID}}`, `{{ENV_VAR}}`, `{{ENDPOINT}}`, `{{HEADER_NAME}}` placeholders derived from the Context7 implementation.

---

### T-F4 — `.claude/skills/add-provider/references/per-client-headers.md` (NEW)
<!-- execution: haiku -->

Per-client output ground truth table (see Appendix B).

---

## Appendix A — File change index

| File | Task(s) | Type |
|---|---|---|
| `pkg/provider/types.go` | T-A1, T-B3 | Modified |
| `pkg/config/json_update.go` | T-A2 | Modified |
| `pkg/config/json_update_test.go` | T-A2 | Modified |
| `pkg/config/toml_update.go` | T-A3 | Modified |
| `pkg/config/toml_update_test.go` | T-A3 | Modified |
| `pkg/verify/verify.go` | T-A4 | Modified |
| `pkg/verify/verify_test.go` | T-A4 | Modified |
| `pkg/context7/keys.go` | T-B1 | New |
| `pkg/context7/keys_test.go` | T-B1 | New |
| `pkg/context7/url.go` | T-B1 | New |
| `pkg/context7/url_test.go` | T-B1 | New |
| `pkg/provider/context7.go` | T-B2 | New |
| `pkg/provider/context7_test.go` | T-B2 | New |
| `pkg/client/adapter.go` | T-B3, T-C1 | Modified |
| `pkg/client/adapter_test.go` | T-B3, T-C1 | Modified |
| `pkg/client/capabilities.go` | T-B3 | Modified (if ProviderBridges approach chosen) |
| `pkg/provider/registry.go` | T-B4 | Modified |
| `pkg/provider/registry_test.go` | T-B4 | Modified |
| `pkg/redact/redact.go` | T-B5 | Modified |
| `pkg/redact/redact_test.go` | T-B5 | Modified |
| `pkg/app/app.go` | T-C2 | Modified |
| `pkg/app/qa_scenarios_test.go` | T-D1, T-D2, T-D3 | Modified |
| `README.md` | T-E1 | Modified |
| `CONTRIBUTING.md` | T-E2 | Modified |
| `docs/contributors/adding-a-provider.md` | T-E3 | New |
| `docs/contributors/dogfooding-with-exa-context7.md` | T-E4 | New |
| `.claude/skills/add-provider/SKILL.md` | T-F1 | New |
| `.claude/skills/add-provider/references/checklist.md` | T-F2 | New |
| `.claude/skills/add-provider/references/code-templates.md` | T-F3 | New |
| `.claude/skills/add-provider/references/per-client-headers.md` | T-F4 | New |

---

## Appendix B — Per-client output ground truth

| Client | File kind | URL field | Headers field | Special |
|---|---|---|---|---|
| Cursor / Roo Code / Kiro | mcpServers JSON | `url` | `headers` | Roo also `"type":"streamable-http"` |
| OpenCode | named server `mcp` | `url` | `headers` | also `"type":"remote","enabled":true` |
| VS Code | named server `servers` | `url` | `headers` | also `"type":"http"` |
| Antigravity / Windsurf | mcpServers JSON | `serverUrl` | `headers` | — |
| Gemini CLI | mcpServers/bare | `httpUrl` | `headers` | + `Accept: application/json, text/event-stream` |
| Codex CLI | TOML | `url` | `http_headers` inline table | — |
| Zed | named server `context_servers` | `url` | `headers` | — |
| Claude Desktop | mcpServers JSON | n/a (stdio) | n/a | `npx -y @upstash/context7-mcp --api-key <key>` |
| Claude Code (CLI) | `claude mcp add` | n/a | `--header` flag | `--header "CONTEXT7_API_KEY: <key>"` |

---

## Appendix C — Definition of done

Context7 provider is **fully complete** when:

1. All tasks T-A1 through T-D3 are merged to `main`
2. `make test` passes with no regressions
3. `make lint` passes
4. `go test -run TestQAContext7AllClients ./pkg/app/...` passes
5. `go test -run TestQAExaAndContext7Coexist ./pkg/app/...` passes
6. Exa fixture snapshot diff is zero bytes (backward compat confirmed)
7. A manual run of `./bin/usync` with a real `ctx7sk_` key produces a valid `~/.cursor/mcp.json` entry with `"CONTEXT7_API_KEY"` in `headers`
8. The raw API key does not appear in any plan/log output
9. `docs/contributors/adding-a-provider.md` is published and linked from README and CONTRIBUTING
10. `.claude/skills/add-provider/SKILL.md` is present and invocable via `/add-provider`

---

## Out of scope

- OAuth flow (`/mcp/oauth` endpoint — defer to a future task)
- Mirroring the `add-provider` skill to `~/.codex/skills/`, `~/.gemini/skills/`, `~/.gemini/antigravity/skills/`
- Generic `verifyGenericProviderFile` refactor — planned as Phase 3A T07 in `architecture-upgrade-plan.md`
- CI/CD changes — existing Lefthook hooks cover all gates
