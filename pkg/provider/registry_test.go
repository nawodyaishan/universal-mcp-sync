package provider

import (
	"testing"
)

func TestDefaultRegistryContainsAllProviders(t *testing.T) {
	r := DefaultRegistry()
	all := r.All()

	if len(all) != 8 {
		t.Fatalf("expected 8 providers, got %d", len(all))
	}

	if all[0].ID() != "exa" {
		t.Fatalf("expected first provider to be exa, got %s", all[0].ID())
	}
	if all[1].ID() != "github" {
		t.Fatalf("expected second provider to be github, got %s", all[1].ID())
	}
	if all[2].ID() != "context7" {
		t.Fatalf("expected third provider to be context7, got %s", all[2].ID())
	}
	if all[3].ID() != "tavily" {
		t.Fatalf("expected fourth provider to be tavily, got %s", all[3].ID())
	}
	if all[4].ID() != "playwright" {
		t.Fatalf("expected fifth provider to be playwright, got %s", all[4].ID())
	}
	if all[5].ID() != "kubernetes" {
		t.Fatalf("expected sixth provider to be kubernetes, got %s", all[5].ID())
	}
	if all[6].ID() != "terraform" {
		t.Fatalf("expected seventh provider to be terraform, got %s", all[6].ID())
	}
	if all[7].ID() != "codegraph" {
		t.Fatalf("expected eighth provider to be codegraph, got %s", all[7].ID())
	}

	p, ok := r.Get("exa")
	if !ok {
		t.Fatal("expected to find exa provider by ID")
	}

	if p.Name() != "Exa AI Search" {
		t.Fatalf("unexpected provider name: %s", p.Name())
	}
}

func TestExaProviderCredentialSpec(t *testing.T) {
	p := NewExaProvider()
	specs := p.RequiredCredentials()

	if len(specs) != 1 {
		t.Fatalf("expected 1 credential spec, got %d", len(specs))
	}

	spec := specs[0]
	if spec.Key != "EXA_API_KEY" {
		t.Fatalf("unexpected spec key: %s", spec.Key)
	}
	if !spec.Secret {
		t.Fatal("expected spec to be marked as secret")
	}
	if !spec.MultiValue {
		t.Fatal("expected spec to be marked as multi-value")
	}

	// Test validator
	err := spec.Validator("11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("expected valid key to pass validation, got error: %v", err)
	}

	err = spec.Validator("invalid")
	if err == nil {
		t.Fatal("expected invalid key to fail validation")
	}
}

func TestExaProvider(t *testing.T) {
	p := NewExaProvider()
	if p.Name() != "Exa AI Search" {
		t.Errorf("unexpected name: %s", p.Name())
	}
	if p.Description() == "" {
		t.Error("expected description")
	}

	creds := map[string]string{"EXA_API_KEY": "11111111-1111-1111-1111-111111111111"}
	cfg, err := p.GenerateConfig(creds)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}
	if cfg.Type != TransportHTTP {
		t.Errorf("expected TransportHTTP, got %s", cfg.Type)
	}

	profiles, err := p.ParseMultiValue("EXA_API_KEY", "11111111-1111-1111-1111-111111111111, 22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("ParseMultiValue failed: %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestGitHubProviderMetadata(t *testing.T) {
	p := NewGitHubProvider()
	if p.Name() != "GitHub" {
		t.Errorf("unexpected name: %s", p.Name())
	}
	if p.Description() == "" {
		t.Error("expected description")
	}
}
