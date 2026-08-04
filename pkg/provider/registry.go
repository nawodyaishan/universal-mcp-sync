package provider

type Registry struct {
	providers map[string]MCPProvider
	order     []string
}

func DefaultRegistry() Registry {
	r := Registry{
		providers: make(map[string]MCPProvider),
		order:     []string{},
	}
	r.register(NewExaProvider())
	r.register(NewGitHubProvider())
	r.register(NewContext7Provider())
	r.register(NewTavilyProvider())
	r.register(NewPlaywrightProvider())
	r.register(NewKubernetesProvider())
	r.register(NewTerraformProvider())
	r.register(NewCodeGraphProvider())
	return r
}

func (r *Registry) register(p MCPProvider) {
	if _, exists := r.providers[p.ID()]; !exists {
		r.order = append(r.order, p.ID())
	}
	r.providers[p.ID()] = p
}

func (r Registry) All() []MCPProvider {
	all := make([]MCPProvider, 0, len(r.order))
	for _, id := range r.order {
		all = append(all, r.providers[id])
	}
	return all
}

func (r Registry) Get(id string) (MCPProvider, bool) {
	p, ok := r.providers[id]
	return p, ok
}
