# Add Terraform Provider

**Last updated:** 2026-05-12
**Status:** Approved
**Audience:** Implementation engineers and AI agents

---

## How to use this document

This spec is the implementation contract for adding the Terraform MCP provider. Work phases in order. Run `make fmt`, `make test`, and `make gitignore-check`; run `make lint` and `make verify` because this touches provider/client compatibility and prerequisite checks.

---

## Context

Why this change: The user requested Terraform MCP support after adding runtime and browser providers. Terraform adds Infrastructure as Code registry and HCP Terraform context for platform, DevOps, SRE, and cloud infrastructure workflows.

Intended outcome:
- A working Terraform provider registered in the provider registry.
- Terraform uses local `stdio` transport through Docker, matching the official container setup.
- `usync` performs better prerequisite checks because the provider requires Docker.
- Destructive Terraform operations stay disabled by default.
- No credential prompt is shown. The provider works for public Terraform Registry tools without a token; users can manually layer HCP Terraform credentials later if needed.
- Stdio-capable clients receive the same provider-generated command shape through existing generic config writers.
- Unsupported stdio clients are skipped with clear warnings instead of receiving malformed config.

**Official docs confirmed:** HashiCorp's Terraform MCP documentation and repository document Docker-based stdio usage:

```json
{
  "mcpServers": {
    "terraform": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "hashicorp/terraform-mcp-server:0.5.2"]
    }
  }
}
```

The docs also state:
- Docker must be installed and running for the container setup.
- `ENABLE_TF_OPERATIONS` defaults to `false`.
- Some destructive actions require `ENABLE_TF_OPERATIONS=true`.

---

## Architecture decision

### Decision: `TransportStdio` with Docker runtime, no credentials, and explicit safe defaults

**Why:** The official MCP server is distributed as a Docker image and the user specifically requested better prerequisite checks because Docker is required. Public Terraform Registry access does not require credentials, so the first provider version should not ask for HCP Terraform tokens.

**Transport shape:**

```go
provider.MCPConfig{
	Type:    provider.TransportStdio,
	Command: "docker",
	Args: []string{
		"run",
		"-i",
		"--rm",
		"-e",
		"ENABLE_TF_OPERATIONS=false",
		"hashicorp/terraform-mcp-server:0.5.2",
	},
	Runtime: &provider.PackageRuntime{Type: "oci"},
}
```

**Credential shape:** `RequiredCredentials()` returns nil. The existing TUI profile fallback creates one `Default` profile for providers with no credential specs.

**Safety decision:** Generated config explicitly passes `ENABLE_TF_OPERATIONS=false` into the container even though the upstream default is false. `usync` will not expose an enable-operations toggle in this task.

**Prerequisite decision:** Before adding operations for a provider config with `Command == "docker"`, `PrepareProvider` checks:
- `docker` is available with `LookPath("docker")`.
- The Docker daemon is reachable with `docker info --format {{.ServerVersion}}`.

If either check fails, planning skips that provider for selected apps and emits a clear warning. This avoids writing configs that are known to be unusable.

---

## Dependency graph

```text
Phase A (Prerequisites)
 ├─ T-A1  Add Docker prerequisite helper in pkg/app
 └─ T-A2  Add app tests for missing/stopped Docker
     └─ Phase B (Provider)
         ├─ T-B1  Implement pkg/provider/terraform.go
         ├─ T-B2  Register provider and update registry tests
         └─ T-B3  Add provider unit tests
             └─ Phase C (QA + docs)
                 ├─ T-C1  Add Terraform QA scenario
                 └─ T-C2  Update README provider matrix
```

---

## Phase A - Prerequisites

### T-A1 - Docker prerequisite helper

**Files:** `pkg/app/app.go` - modified

Add a helper used by `PrepareProvider` after `GenerateConfig` and before adding client operations:
- For non-Docker providers, return no warning.
- For Docker providers, check `LookPath("docker")`.
- Then run `docker info --format {{.ServerVersion}}`.
- Return a warning string if Docker is unavailable or the daemon is not reachable.

### T-A2 - App tests

**Files:** `pkg/app/app_test.go` - modified

Add tests that Terraform planning:
- Skips operations and warns when Docker is missing.
- Skips operations and warns when Docker is installed but daemon check fails.
- Produces operations when Docker is available and daemon check succeeds.

---

## Phase B - Provider

### T-B1 - Implement `TerraformProvider`

**Files:** `pkg/provider/terraform.go` - new

Implement `MCPProvider` with:
- ID: `terraform`
- Name: `Terraform`
- Description: Terraform Registry and HCP Terraform context for IaC workflows.
- No required credentials.
- `GenerateConfig` returns Docker stdio command with explicit `ENABLE_TF_OPERATIONS=false`.

### T-B2 - Register provider

**Files:** `pkg/provider/registry.go`, `pkg/provider/registry_test.go` - modified

Add `NewTerraformProvider()` to `DefaultRegistry()` after Kubernetes. Update count/order assertions.

### T-B3 - Provider tests

**Files:** `pkg/provider/terraform_test.go` - new

Assert metadata, zero credential specs, stdio config, OCI runtime, Docker command, image tag, and explicit `ENABLE_TF_OPERATIONS=false`.

---

## Phase C - QA and docs

### T-C1 - Terraform QA scenario

**Files:** `pkg/app/qa_scenarios_test.go` - modified

Add end-to-end coverage for Terraform across stdio-capable clients with Docker available. Verify Docker command, image tag, and explicit operations-disabled environment survive JSON, Codex TOML, and Claude Code CLI planning. Verify unsupported stdio clients are skipped.

### T-C2 - README provider matrix

**Files:** `README.md` - modified

Add Terraform to the supported MCP table with Docker stdio and no required auth.

---

## Acceptance criteria

- `provider.DefaultRegistry()` includes Terraform.
- Terraform generates a no-auth stdio config with Docker and `hashicorp/terraform-mcp-server:0.5.2`.
- Generated Docker args include `ENABLE_TF_OPERATIONS=false`.
- `PrepareProvider` skips Terraform with a clear warning if Docker is missing.
- `PrepareProvider` skips Terraform with a clear warning if Docker is installed but the daemon is unreachable.
- Claude Code CLI planning includes Docker args.
- Codex TOML includes Docker args.
- Gemini CLI and Antigravity are skipped for Terraform with clear unsupported-transport warnings when Docker is available.
- `make fmt`, `make test`, `make gitignore-check`, `make lint`, and `make verify` pass.
