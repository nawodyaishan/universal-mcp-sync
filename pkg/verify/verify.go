package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/exa"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
)

type Result struct {
	Target  string
	Status  Status
	Details []string
}

type Runner interface {
	LookPath(name string) (string, error)
	Run(name string, args ...string) (string, error)
}

func VerifyFile(path string, kind config.FileKind, expectedTools int) Result {
	switch kind {
	case config.FileKindMCPServers:
		return verifyMCPServersFile(path, expectedTools)
	case config.FileKindBareMCPServers:
		return verifyBareMCPServersFile(path, expectedTools)
	case config.FileKindNamedServer:
		return verifyNamedServerFile(path, expectedTools)
	case config.FileKindCodexTOML:
		return verifyCodexFile(path, expectedTools)
	default:
		return failure(path, "unsupported verification target")
	}
}

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
		if err != nil {
			return failure(path, err.Error())
		}
		details, ok := inspectStdioServer(server)
		return resultFrom(path, details, ok)
	}
	server, err := readNestedServerEntry(path, "mcpServers", "context7")
	if err != nil {
		return failure(path, err.Error())
	}
	return inspectContext7Server(path, server)
}

func verifyContext7BareMCPServersFile(path string, cfg provider.MCPConfig) Result {
	if cfg.Type == provider.TransportStdio {
		server, err := readRootServerEntry(path, "context7")
		if err != nil {
			return failure(path, err.Error())
		}
		details, ok := inspectStdioServer(server)
		return resultFrom(path, details, ok)
	}
	server, err := readRootServerEntry(path, "context7")
	if err != nil {
		return failure(path, err.Error())
	}
	return inspectContext7Server(path, server)
}

func verifyContext7NamedServerFile(path string, cfg provider.MCPConfig) Result {
	// Use readServerEntryByKind so files with a root key (Zed: context_servers,
	// VS Code: servers) are handled alongside bare root-level entries.
	server, err := readServerEntryByKind(path, config.FileKindNamedServer, "context7")
	if err != nil {
		return failure(path, err.Error())
	}
	if cfg.Type == provider.TransportStdio {
		details, ok := inspectStdioServer(server)
		return resultFrom(path, details, ok)
	}
	return inspectContext7Server(path, server)
}

func inspectContext7Server(path string, server map[string]any) Result {
	urlValue := getURLField(server)
	if urlValue == "" {
		return failure(path, "missing context7 URL field")
	}
	if !strings.Contains(urlValue, "mcp.context7.com") {
		return Result{
			Target:  path,
			Status:  StatusWarning,
			Details: []string{fmt.Sprintf("unexpected Context7 endpoint: %s", urlValue)},
		}
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
	if err != nil {
		return failure(path, err.Error())
	}
	text := string(data)
	if !strings.Contains(text, "[mcp_servers.context7]") {
		return failure(path, "missing [mcp_servers.context7] block")
	}
	if !strings.Contains(text, "http_headers") {
		return failure(path, "missing http_headers in context7 TOML block")
	}
	return Result{
		Target:  path,
		Status:  StatusOK,
		Details: []string{"block present", "http_headers present"},
	}
}

func resultFrom(path string, details []string, ok bool) Result {
	status := StatusOK
	if !ok {
		status = StatusFailed
	}
	return Result{Target: path, Status: status, Details: details}
}

func verifyExaProviderFile(path string, kind config.FileKind, cfg provider.MCPConfig) Result {
	switch kind {
	case config.FileKindMCPServers:
		return verifyExaMCPServersFile(path, cfg)
	case config.FileKindBareMCPServers:
		return verifyExaBareMCPServersFile(path, cfg)
	case config.FileKindNamedServer:
		return verifyExaNamedServerFile(path, cfg)
	case config.FileKindCodexTOML:
		return verifyCodexFile(path, len(exa.DefaultTools))
	default:
		return failure(path, "unsupported verification target")
	}
}

func verifyGenericProviderFile(path string, kind config.FileKind, providerID string, cfg provider.MCPConfig) Result {
	server, err := readServerEntryByKind(path, kind, providerID)
	if err != nil {
		return failure(path, err.Error())
	}
	if cfg.Type == provider.TransportStdio {
		return verifyGenericStdioServer(path, server, cfg)
	}
	return verifyGenericHTTPServer(path, server)
}

func verifyGenericStdioServer(path string, server map[string]any, cfg provider.MCPConfig) Result {
	command, _ := server["command"].(string)
	if command == "" {
		if commandList, ok := server["command"].([]any); ok && len(commandList) > 0 {
			command, _ = commandList[0].(string)
		}
	}
	if command == "" {
		return failure(path, "missing stdio command field")
	}
	if command != cfg.Command {
		return Result{
			Target:  path,
			Status:  StatusWarning,
			Details: []string{fmt.Sprintf("command=%s (expected %s)", command, cfg.Command)},
		}
	}
	return Result{
		Target:  path,
		Status:  StatusOK,
		Details: []string{fmt.Sprintf("command=%s", command)},
	}
}

func verifyGenericHTTPServer(path string, server map[string]any) Result {
	urlValue := getURLField(server)
	if urlValue == "" {
		return failure(path, "missing URL field (checked: url, httpUrl, serverUrl)")
	}
	if _, err := url.Parse(urlValue); err != nil {
		return failure(path, fmt.Sprintf("invalid URL: %v", err))
	}
	return Result{
		Target:  path,
		Status:  StatusOK,
		Details: []string{"url present and valid"},
	}
}

// readServerEntryByKind dispatches to the correct reader based on FileKind.
func readServerEntryByKind(path string, kind config.FileKind, providerID string) (map[string]any, error) {
	switch kind {
	case config.FileKindMCPServers:
		return readNestedServerEntry(path, "mcpServers", providerID)
	case config.FileKindBareMCPServers:
		return readRootServerEntry(path, providerID)
	case config.FileKindNamedServer:
		// Try common root keys; fall back to root-level entry
		for _, rootKey := range []string{"servers", "context_servers", "mcp"} {
			s, err := readNestedServerEntry(path, rootKey, providerID)
			if err == nil {
				return s, nil
			}
		}
		return readRootServerEntry(path, providerID)
	case config.FileKindCodexTOML:
		return readCodexServerEntry(path, providerID)
	default:
		return nil, fmt.Errorf("verification not supported for kind %q", kind)
	}
}

func VerifyOptionalCLI(runner Runner, binary string, args ...string) Result {
	label := binary + " " + strings.Join(args, " ")
	if _, err := runner.LookPath(binary); err != nil {
		return Result{
			Target:  label,
			Status:  StatusSkipped,
			Details: []string{"CLI unavailable"},
		}
	}

	output, err := runner.Run(binary, args...)
	if err != nil {
		return Result{
			Target:  label,
			Status:  StatusWarning,
			Details: []string{exa.RedactText(err.Error())},
		}
	}

	return Result{
		Target:  label,
		Status:  StatusOK,
		Details: summarizeOutput(output),
	}
}

func verifyMCPServersFile(path string, expectedTools int) Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return failure(path, err.Error())
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return failure(path, fmt.Sprintf("parse JSON: %v", err))
	}

	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		return failure(path, "missing mcpServers object")
	}

	exaValue, ok := servers["exa"].(map[string]any)
	if !ok {
		return failure(path, "missing mcpServers.exa entry")
	}

	urlValue := getURLField(exaValue)
	return inspectFileURL(path, urlValue, expectedTools)
}

func verifyExaMCPServersFile(path string, cfg provider.MCPConfig) Result {
	server, err := readNestedServerEntry(path, "mcpServers", "exa")
	if err != nil {
		return failure(path, err.Error())
	}
	return inspectExaServer(path, server, cfg)
}

func verifyBareMCPServersFile(path string, expectedTools int) Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return failure(path, err.Error())
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return failure(path, fmt.Sprintf("parse JSON: %v", err))
	}

	exaValue, ok := root["exa"].(map[string]any)
	if !ok {
		return failure(path, "missing exa entry")
	}

	urlValue := getURLField(exaValue)
	return inspectFileURL(path, urlValue, expectedTools)
}

func verifyExaBareMCPServersFile(path string, cfg provider.MCPConfig) Result {
	server, err := readRootServerEntry(path, "exa")
	if err != nil {
		return failure(path, err.Error())
	}
	return inspectExaServer(path, server, cfg)
}

func verifyNamedServerFile(path string, expectedTools int) Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return failure(path, err.Error())
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return failure(path, fmt.Sprintf("parse JSON: %v", err))
	}

	exaValue, ok := root["exa"].(map[string]any)
	if !ok {
		return failure(path, "missing exa entry")
	}

	urlValue := getURLField(exaValue)
	return inspectFileURL(path, urlValue, expectedTools)
}

func verifyExaNamedServerFile(path string, cfg provider.MCPConfig) Result {
	// Use readServerEntryByKind so files with a root key (Zed: context_servers,
	// VS Code: servers) are handled alongside bare root-level entries.
	server, err := readServerEntryByKind(path, config.FileKindNamedServer, "exa")
	if err != nil {
		return failure(path, err.Error())
	}
	return inspectExaServer(path, server, cfg)
}

func readNestedServerEntry(path, rootKey, providerID string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse JSON: %v", err)
	}

	servers, ok := root[rootKey].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing %s object", rootKey)
	}

	server, ok := servers[providerID].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing %s.%s entry", rootKey, providerID)
	}
	return server, nil
}

func readRootServerEntry(path, providerID string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse JSON: %v", err)
	}

	server, ok := root[providerID].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing %s entry", providerID)
	}
	return server, nil
}

func readCodexServerEntry(path, providerID string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	section := "[mcp_servers." + providerID + "]"
	lines := strings.Split(string(data), "\n")
	inSection := false
	server := make(map[string]any)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTOMLSectionHeader(trimmed) {
			if trimmed == section {
				inSection = true
				continue
			}
			if inSection {
				break
			}
		}
		if !inSection {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "url = "):
			server["url"] = strings.Trim(strings.TrimPrefix(trimmed, "url = "), `"`)
		case strings.HasPrefix(trimmed, "command = "):
			server["command"] = strings.Trim(strings.TrimPrefix(trimmed, "command = "), `"`)
		}
	}
	if !inSection {
		return nil, fmt.Errorf("missing %s block", section)
	}
	return server, nil
}

func isTOMLSectionHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

func inspectExaServer(path string, server map[string]any, cfg provider.MCPConfig) Result {
	if cfg.Type == provider.TransportStdio {
		details, ok := inspectStdioServer(server)
		if !ok {
			return Result{Target: path, Status: StatusFailed, Details: details}
		}
		return Result{Target: path, Status: StatusOK, Details: details}
	}

	urlValue := getURLField(server)
	return inspectFileURL(path, urlValue, len(exa.DefaultTools))
}

func inspectStdioServer(server map[string]any) ([]string, bool) {
	command, _ := server["command"].(string)
	if command == "" {
		return []string{"missing stdio command"}, false
	}

	rawArgs, _ := server["args"].([]any)
	args := make([]string, 0, len(rawArgs))
	for _, raw := range rawArgs {
		if value, ok := raw.(string); ok && value != "" {
			args = append(args, value)
		}
	}

	details := []string{
		fmt.Sprintf("command=%s", command),
		fmt.Sprintf("arg count=%d", len(args)),
	}

	if command != "npx" {
		return append(details, "unexpected stdio command"), false
	}
	if len(args) < 3 {
		return append(details, "missing mcp-remote bridge args"), false
	}
	if args[0] != "-y" || args[1] != "mcp-remote" {
		return append(details, "unexpected stdio bridge invocation"), false
	}

	urlDetails, ok := inspectURL(args[2], len(exa.DefaultTools))
	return append(details, urlDetails...), ok
}

func getURLField(obj map[string]any) string {
	fields := []string{"url", "httpUrl", "serverUrl"}
	for _, f := range fields {
		if val, ok := obj[f].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

func verifyCodexFile(path string, expectedTools int) Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return failure(path, err.Error())
	}

	text := string(data)
	section := "[mcp_servers.exa]"
	index := strings.Index(text, section)
	if index == -1 {
		return failure(path, "missing [mcp_servers.exa] block")
	}

	block := text[index:]
	lines := strings.Split(block, "\n")
	urlValue := ""
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			break
		}
		if strings.HasPrefix(trimmed, "url = ") {
			urlValue = strings.Trim(strings.TrimPrefix(trimmed, "url = "), `"`)
			break
		}
	}

	return inspectFileURL(path, urlValue, expectedTools)
}

func inspectFileURL(path, raw string, expectedTools int) Result {
	details, ok := inspectURL(raw, expectedTools)
	if !ok {
		return Result{Target: path, Status: StatusFailed, Details: details}
	}
	return Result{Target: path, Status: StatusOK, Details: details}
}

func inspectURL(raw string, expectedTools int) ([]string, bool) {
	if raw == "" {
		return []string{"missing Exa URL"}, false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return []string{fmt.Sprintf("parse URL: %v", err)}, false
	}

	query := parsed.Query()
	key := query.Get("exaApiKey")
	tools := strings.Split(query.Get("tools"), ",")

	details := []string{
		fmt.Sprintf("host=%s", parsed.Host),
		fmt.Sprintf("api key present=%t", key != ""),
		fmt.Sprintf("tool count=%d", len(filterEmpty(tools))),
	}

	if parsed.Scheme != "https" || parsed.Host != "mcp.exa.ai" {
		return append(details, "unexpected Exa MCP endpoint"), false
	}
	if key == "" {
		return append(details, "missing exaApiKey"), false
	}
	if len(filterEmpty(tools)) != expectedTools {
		return append(details, "unexpected tool count"), false
	}
	return details, true
}

func summarizeOutput(output string) []string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return []string{"command completed"}
	}

	lines := bytes.Split([]byte(trimmed), []byte("\n"))
	if len(lines) > 3 {
		lines = lines[:3]
	}

	summary := make([]string, 0, len(lines))
	for _, line := range lines {
		summary = append(summary, exa.RedactText(string(bytes.TrimSpace(line))))
	}
	return summary
}

func filterEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func failure(target, detail string) Result {
	return Result{Target: target, Status: StatusFailed, Details: []string{detail}}
}
