package manifest

var allProviders = []ProviderMeta{
	{
		ID:      ProviderExa,
		Name:    "Exa AI Search",
		DocsURL: "https://exa.ai/docs/reference/exa-mcp",
		Credentials: []CredentialAcquisition{
			{
				Key:          "EXA_API_KEY",
				EnvVar:       "EXA_API_KEY",
				Required:     true,
				FormatHint:   "UUID-style API key",
				OfflineRegex: `^[0-9a-fA-F-]{36}$`,
				GetURL:       "https://dashboard.exa.ai/api-keys",
				DocsURL:      "https://exa.ai/docs/reference/quickstart",
			},
		},
		Sources: []SourceRef{
			{URL: "https://exa.ai/docs/reference/exa-mcp", Title: "Exa MCP", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:      ProviderGitHub,
		Name:    "GitHub",
		DocsURL: "https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens",
		Credentials: []CredentialAcquisition{
			{
				Key:          "GITHUB_PERSONAL_ACCESS_TOKEN",
				EnvVar:       "GITHUB_PERSONAL_ACCESS_TOKEN",
				Required:     true,
				FormatHint:   "ghp_..., github_pat_..., or legacy 40-char hex token",
				OfflineRegex: `^(?:[0-9a-f]{40}|ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{59,})$`,
				GetURL:       "https://github.com/settings/tokens",
				DocsURL:      "https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens",
				LiveValidation: &LiveValidationSpec{
					Method:     "GET",
					URL:        "https://api.github.com/user",
					AuthHeader: "Authorization: Bearer {key}",
					QuotaSafe:  true,
				},
			},
		},
		RuntimeIDs: []string{"node", "npx"},
		Sources: []SourceRef{
			{URL: "https://github.com/settings/tokens", Title: "GitHub Tokens", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:      ProviderContext7,
		Name:    "Context7",
		DocsURL: "https://context7.com/docs/api-guide",
		Credentials: []CredentialAcquisition{
			{
				Key:          "CONTEXT7_API_KEY",
				EnvVar:       "CONTEXT7_API_KEY",
				Required:     true,
				FormatHint:   "ctx7sk-... or ctx7sk_...",
				OfflineRegex: `^ctx7sk[-_][A-Za-z0-9]+$`,
				GetURL:       "https://context7.com/dashboard",
				DocsURL:      "https://context7.com/docs/api-guide",
			},
		},
		Sources: []SourceRef{
			{URL: "https://context7.com/docs/api-guide", Title: "Context7 API Guide", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:      ProviderTavily,
		Name:    "Tavily Search",
		DocsURL: "https://docs.tavily.com/documentation/mcp",
		Credentials: []CredentialAcquisition{
			{
				Key:          "TAVILY_API_KEY",
				EnvVar:       "TAVILY_API_KEY",
				Required:     true,
				FormatHint:   "tvly-...",
				OfflineRegex: `^tvly-[A-Za-z0-9]+$`,
				GetURL:       "https://app.tavily.com/home",
				DocsURL:      "https://docs.tavily.com/documentation/quickstart",
				LiveValidation: &LiveValidationSpec{
					Method:     "GET",
					URL:        "https://api.tavily.com/usage",
					AuthHeader: "Authorization: Bearer {key}",
					QuotaSafe:  true,
				},
			},
		},
		RuntimeIDs: []string{"node", "npx"},
		Sources: []SourceRef{
			{URL: "https://docs.tavily.com/documentation/mcp", Title: "Tavily MCP", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:         ProviderPlaywright,
		Name:       "Playwright",
		DocsURL:    "https://playwright.dev/docs/getting-started-mcp",
		RuntimeIDs: []string{"node", "npx"},
		Sources: []SourceRef{
			{URL: "https://playwright.dev/docs/getting-started-mcp", Title: "Playwright MCP", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:         ProviderKubernetes,
		Name:       "Kubernetes",
		DocsURL:    "https://github.com/manusa/kubernetes-mcp-server",
		RuntimeIDs: []string{"node", "npx"},
		Sources: []SourceRef{
			{URL: "https://github.com/manusa/kubernetes-mcp-server", Title: "Kubernetes MCP Server", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:      ProviderTerraform,
		Name:    "Terraform",
		DocsURL: "https://developer.hashicorp.com/terraform/mcp-server",
		Credentials: []CredentialAcquisition{
			{
				Key:          "TFE_TOKEN",
				EnvVar:       "TFE_TOKEN",
				Required:     false,
				FormatHint:   "Optional HCP Terraform token",
				OfflineRegex: `^.+$`,
				GetURL:       "https://app.terraform.io/app/settings/tokens",
				DocsURL:      "https://developer.hashicorp.com/terraform/mcp-server",
			},
		},
		RuntimeIDs: []string{"docker"},
		Sources: []SourceRef{
			{URL: "https://developer.hashicorp.com/terraform/mcp-server", Title: "Terraform MCP Server", VerifiedAt: "2026-05-21", Confidence: "official"},
		},
	},
	{
		ID:         ProviderCodeGraph,
		Name:       "CodeGraph",
		DocsURL:    "https://github.com/colbymchenry/codegraph",
		RuntimeIDs: []string{"node", "npx"},
		Sources: []SourceRef{
			{URL: "https://github.com/colbymchenry/codegraph", Title: "CodeGraph", VerifiedAt: "2026-08-04", Confidence: "official"},
		},
	},
}
