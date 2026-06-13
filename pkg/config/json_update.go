package config

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

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
		// HTTP or SSE
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

func UpdateMCPServersJSON(data []byte, providerID, rootKey, urlFieldName string, cfg provider.MCPConfig, extra map[string]any) ([]byte, error) {
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, err
	}

	if rootKey == "" {
		rootKey = "mcpServers"
	}

	servers := ensureObject(root, rootKey)
	servers[providerID] = buildConfigMap(cfg, urlFieldName, extra)

	return marshalJSON(root)
}

func UpdateBareMCPServersJSON(data []byte, providerID, urlFieldName string, cfg provider.MCPConfig, extra map[string]any) ([]byte, error) {
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, err
	}

	root[providerID] = buildConfigMap(cfg, urlFieldName, extra)

	return marshalJSON(root)
}

func UpdateNamedServerJSON(data []byte, providerID, rootKey, urlFieldName string, cfg provider.MCPConfig, extra map[string]any) ([]byte, error) {
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, err
	}

	var parent map[string]any
	if rootKey != "" {
		parent = ensureObject(root, rootKey)
	} else {
		parent = root
	}

	server := ensureObject(parent, providerID)
	// For named server updates (like Antigravity), we typically only update the URL field
	// and preserve the rest of the object.
	if cfg.Type != provider.TransportStdio {
		if urlFieldName == "" {
			urlFieldName = "url"
		}
		server[urlFieldName] = cfg.URL
		if len(cfg.Headers) > 0 {
			headers := make(map[string]string, len(cfg.Headers))
			for k, v := range cfg.Headers {
				headers[k] = v
			}
			server["headers"] = headers
		}
	} else {
		// If it switches to stdio, we overwrite with the full stdio map
		server = buildConfigMap(cfg, urlFieldName, extra)
		parent[providerID] = server
	}

	for k, v := range extra {
		server[k] = v
	}

	return marshalJSON(root)
}

func UpdateOpenCodeJSON(data []byte, providerID string, cfg provider.MCPConfig) ([]byte, error) {
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, err
	}

	servers := ensureObject(root, "mcp")
	entry := map[string]any{
		"enabled": true,
	}
	if cfg.Type == provider.TransportStdio {
		command := make([]string, 0, 1+len(cfg.Args))
		command = append(command, cfg.Command)
		command = append(command, cfg.Args...)
		entry["type"] = "local"
		entry["command"] = command
		if len(cfg.Env) > 0 {
			entry["environment"] = cfg.Env
		}
	} else {
		entry["type"] = "remote"
		entry["url"] = cfg.URL
		if len(cfg.Headers) > 0 {
			headers := make(map[string]string, len(cfg.Headers))
			for k, v := range cfg.Headers {
				headers[k] = v
			}
			entry["headers"] = headers
		}
	}
	servers[providerID] = entry

	return marshalJSON(root)
}

// UsyncMeta returns the _usync provenance annotation map for a server entry.
// planID may be "" for the legacy Apply path where no plan ID is available.
func UsyncMeta(planID, at string) map[string]any {
	return map[string]any{
		"_usync": map[string]any{
			"managedBy": "usync",
			"at":        at,
			"planID":    planID,
		},
	}
}

// VSCodeInput describes one entry in the VS Code mcp.json root-level "inputs" array.
// VS Code prompts the user for the value on first server start and stores it securely.
type VSCodeInput struct {
	Type        string `json:"type"` // always "promptString"
	ID          string `json:"id"`
	Description string `json:"description"`
	Password    bool   `json:"password"`
}

// MergeVSCodeInputs merges input definitions into the root-level "inputs" array of VS Code
// mcp.json content. Entries with the same id replace existing ones; all others are preserved.
// Safe to call with an empty inputs slice (returns data unchanged).
func MergeVSCodeInputs(data []byte, inputs []VSCodeInput) ([]byte, error) {
	if len(inputs) == 0 {
		return data, nil
	}
	root, err := decodeJSONObject(data)
	if err != nil {
		return nil, err
	}

	// Build ordered list of existing inputs, keyed by id for dedup.
	var order []string
	byID := make(map[string]VSCodeInput)

	if raw, ok := root["inputs"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" {
				continue
			}
			if _, seen := byID[id]; !seen {
				order = append(order, id)
			}
			byID[id] = decodeVSCodeInput(m)
		}
	}

	// Merge new inputs: replace on duplicate id, append otherwise.
	for _, inp := range inputs {
		if _, exists := byID[inp.ID]; !exists {
			order = append(order, inp.ID)
		}
		byID[inp.ID] = inp
	}

	// Rebuild ordered slice.
	merged := make([]any, 0, len(order))
	for _, id := range order {
		merged = append(merged, byID[id])
	}
	root["inputs"] = merged
	return marshalJSON(root)
}

func decodeVSCodeInput(m map[string]any) VSCodeInput {
	inp := VSCodeInput{}
	if v, ok := m["type"].(string); ok {
		inp.Type = v
	}
	if v, ok := m["id"].(string); ok {
		inp.ID = v
	}
	if v, ok := m["description"].(string); ok {
		inp.Description = v
	}
	if v, ok := m["password"].(bool); ok {
		inp.Password = v
	}
	return inp
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return map[string]any{}, nil
	}

	root := make(map[string]any)
	if err := json.Unmarshal(stripTrailingCommas(stripJSONComments(trimmed)), &root); err != nil {
		return nil, fmt.Errorf("parse JSON config: %w", err)
	}
	return root, nil
}

// stripTrailingCommas removes trailing commas before } or ] in JSONC input.
// String literals are preserved intact. Must be called after stripJSONComments
// so comment-induced whitespace is already gone.
func stripTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		if src[i] == '"' {
			// String: copy verbatim, respecting backslash escapes.
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == '"' {
					j++
					break
				}
				j++
			}
			out = append(out, src[i:j]...)
			i = j
			continue
		}
		if src[i] == ',' {
			// Look ahead past whitespace; if next structural char is } or ], drop the comma.
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				// Trailing comma: emit the whitespace but skip the comma.
				out = append(out, src[i+1:j]...)
				i = j
				continue
			}
		}
		out = append(out, src[i])
		i++
	}
	return out
}

// stripJSONComments removes // and /* */ comments from JSONC input.
// String literals are preserved intact so comment-like sequences inside
// them are not removed. Safe to call on plain JSON (no-op).
func stripJSONComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		switch {
		case src[i] == '"':
			// Consume string literal verbatim, respecting backslash escapes.
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == '"' {
					j++
					break
				}
				j++
			}
			out = append(out, src[i:j]...)
			i = j
		case i+1 < len(src) && src[i] == '/' && src[i+1] == '/':
			// Single-line comment: skip to end of line.
			j := i + 2
			for j < len(src) && src[j] != '\n' {
				j++
			}
			i = j
		case i+1 < len(src) && src[i] == '/' && src[i+1] == '*':
			// Block comment: skip to closing */.
			j := i + 2
			for j+1 < len(src) && (src[j] != '*' || src[j+1] != '/') {
				j++
			}
			if j+1 < len(src) {
				j += 2
			}
			i = j
		default:
			out = append(out, src[i])
			i++
		}
	}
	return out
}

func ensureObject(root map[string]any, key string) map[string]any {
	existing, ok := root[key]
	if !ok {
		child := make(map[string]any)
		root[key] = child
		return child
	}

	child, ok := existing.(map[string]any)
	if ok {
		return child
	}

	child = make(map[string]any)
	root[key] = child
	return child
}

func marshalJSON(root map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}
