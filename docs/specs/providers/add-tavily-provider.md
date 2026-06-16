# Add Tavily Provider

**Last updated:** 2026-05-12
**Status:** Approved
**Audience:** Implementation engineers and AI agents

---

## How to use this document

Each task (T-A1, T-B1...) is independently executable. Work phases in order. Run `make test && make lint` after every task. 

---

## Context

Why this change: The user requested to add support for the Tavily MCP server to `usync` following the Spec-Driven Development (SDD) process and the `add-provider` skill.

Intended outcome:
- A working Tavily provider with full client-fleet support.
- Tavily will use the Stdio transport method via the official npm package `tavily-mcp`.
- The provider will pass the API key securely via environment variables (`TAVILY_API_KEY`), ensuring it is not leaked in command-line arguments.

**Critical discovery:**
The Tavily API uses keys prefixed with `tvly-`. 
We will implement the provider using `TransportStdio`, utilizing the `tavily-mcp@latest` package via `npx`.

---

## Architecture decision

### Decision: `TransportStdio` with `Env` authentication

**Why:** The Tavily documentation supports both a remote MCP and a local installation via `npx -y tavily-mcp@latest`. For `usync`, providing a local `stdio` installation is the most robust and universal method across all local AI clients, as it doesn't rely on remote HTTP servers or `mcp-remote` bridges. Furthermore, the API key is passed securely via the `TAVILY_API_KEY` environment variable, which prevents leaking the key in process listings or logs, aligning with the `adding-a-provider.md` guidelines.

**Trade-offs accepted:**
- Requires the user to have Node.js/`npx` installed locally. This is standard for most local MCPs and acceptable for AI users.

---

## Dependency graph

```
Phase A (Provider Implementation)
 ├─ T-A1  pkg/tavily/ helpers (keys.go, keys_test.go)
 ├─ T-A2  pkg/provider/tavily.go
 ├─ T-A3  register in registry
 └─ T-A4  redact tvly- keys
     └─ Phase B (QA)
         ├─ T-B1  TestQATavilyAllClients
         └─ Phase C (Docs)
             └─ T-C1  README provider matrix update
```

---

## Phase A — Provider Implementation

---

### T-A1 — Create `pkg/tavily/` helpers

**Phase:** A
**Files:** `pkg/tavily/keys.go`, `pkg/tavily/keys_test.go` — all new

#### What to create

**`pkg/tavily/keys.go`:**
```go
package tavily

import (
	"fmt"
	"strings"
)

const keyPrefix = "tvly-"
const minKeyLen = len(keyPrefix) + 8

// ParseKey validates a Tavily API key.
// Valid keys start with "tvly-" and have sufficient length.
func ParseKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, keyPrefix) {
		return "", fmt.Errorf("Tavily API key must start with %q", keyPrefix)
	}
	if len(key) < minKeyLen {
		return "", fmt.Errorf("Tavily API key is too short")
	}
	return key, nil
}

// RedactKey masks a Tavily API key for display.
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

#### Acceptance
- [ ] `go test ./pkg/tavily/...` passes.

---

### T-A2 — Implement `TavilyProvider`

**Phase:** A
**Depends on:** T-A1
**Files:** `pkg/provider/tavily.go`, `pkg/provider/tavily_test.go` — both new

#### What to create

**`pkg/provider/tavily.go`:**
```go
package provider

import (
	"fmt"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/tavily"
)

type TavilyProvider struct{}

func NewTavilyProvider() *TavilyProvider { return &TavilyProvider{} }

func (p *TavilyProvider) ID() string          { return "tavily" }
func (p *TavilyProvider) Name() string        { return "Tavily Search" }
func (p *TavilyProvider) Description() string {
	return "Real-time web search and data extraction for AI agents."
}

func (p *TavilyProvider) RequiredCredentials() []CredentialSpec {
	return []CredentialSpec{
		{
			Key:         "TAVILY_API_KEY",
			Label:       "Tavily API Key",
			Description: "Get your key at tavily.com. Format: tvly-...",
			Secret:      true,
			MultiValue:  false,
			Validator: func(s string) error {
				_, err := tavily.ParseKey(s)
				return err
			},
		},
	}
}

func (p *TavilyProvider) GenerateConfig(credentials map[string]string) (MCPConfig, error) {
	key := credentials["TAVILY_API_KEY"]
	if _, err := tavily.ParseKey(key); err != nil {
		return MCPConfig{}, fmt.Errorf("invalid Tavily API key: %w", err)
	}
	return MCPConfig{
		Type:    TransportStdio,
		Command: "npx",
		Args:    []string{"-y", "tavily-mcp@latest"},
		Env:     map[string]string{"TAVILY_API_KEY": key},
		Runtime: &PackageRuntime{Type: "npm"},
	}, nil
}
```

#### Acceptance
- [ ] `go test ./pkg/provider/...` passes.

---

### T-A3 — Register in registry

**Phase:** A
**Depends on:** T-A2
**Files:** `pkg/provider/registry.go`, `pkg/provider/registry_test.go` — modified

#### What to change
Add `r.register(NewTavilyProvider())` to `DefaultRegistry()`. Update `pkg/provider/registry_test.go`.

#### Acceptance
- [ ] `go test ./pkg/provider/...` passes.

---

### T-A4 — Add Tavily key redaction

**Phase:** A
**Depends on:** none
**Files:** `pkg/redact/redact.go`, `pkg/redact/redact_test.go` — modified

#### What to change
Add `tvlyRE = regexp.MustCompile(` + "`" + `tvly-[A-Za-z0-9_\-]{8,}` + "`" + `)` and redact it.

#### Acceptance
- [ ] `go test ./pkg/redact/...` passes.

---

## Phase B — QA

### T-B1 — `TestQATavilyAllClients`

**Phase:** B
**Depends on:** T-A1, T-A2, T-A3, T-A4
**Files:** `pkg/app/qa_scenarios_test.go` — modified

#### What to add
Write end-to-end scenarios for Tavily. Verify that `command: "npx"` and `env: {"TAVILY_API_KEY": ...}` are present in the JSON outputs.

---

## Phase C — Docs

### T-C1 — README provider matrix

Update `README.md` Provider Matrix with a row for Tavily.
