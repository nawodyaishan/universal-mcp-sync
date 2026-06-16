# Provider Integration Specs

One-off specifications for adding new MCP providers to `pkg/provider`. Each file documents the provider's transport (`http` / `sse` / `stdio`), credential shape, validation strategy, and per-client adaptations.

| Spec | Provider |
|---|---|
| [`add-context7-provider.md`](add-context7-provider.md) | Context7 |
| [`add-kubernetes-provider.md`](add-kubernetes-provider.md) | Kubernetes |
| [`add-playwright-provider.md`](add-playwright-provider.md) | Playwright |
| [`add-tavily-provider.md`](add-tavily-provider.md) | Tavily |
| [`add-terraform-provider.md`](add-terraform-provider.md) | Terraform |

For the workflow that produced these, see [`docs/contributors/adding-a-provider.md`](../../contributors/adding-a-provider.md).
