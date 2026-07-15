package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"mcpwn/internal/config"
	"mcpwn/internal/executor"
	"mcpwn/internal/output"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	readOutputToolName   = "mcpwn_read_output"
	tailOutputToolName   = "mcpwn_tail_output"
	searchOutputToolName = "mcpwn_search_output"

	defaultReadLimit     = 64 * 1024
	maxReadLimit         = 1024 * 1024
	defaultTailLines     = 100
	maxTailLines         = 1000
	defaultSearchMatches = 20
	maxSearchMatches     = 100
)

// MCPServer wraps the MCP SDK server and our tool configuration
type MCPServer struct {
	cfg    *config.Config
	srv    *mcp.Server
	logger *slog.Logger
}

func New(cfg *config.Config, version string) *MCPServer {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "mcpwn",
		Version: version,
	}, nil)

	ms := &MCPServer{
		cfg:    cfg,
		srv:    s,
		logger: slog.Default(),
	}

	ms.registerOutputTools()

	for _, t := range cfg.Tools {
		ms.logger.Debug("Registering tool", "name", t.Name)
		ms.srv.AddTool(&mcp.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: generateSchema(t),
		}, ms.handleCallTool)
	}

	return ms
}

func (ms *MCPServer) Serve() error {
	ctx := context.Background()
	transport := &mcp.StdioTransport{}
	session, err := ms.srv.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to transport: %w", err)
	}

	ms.logger.Info("Server listening on Stdio")
	return session.Wait()
}

func (ms *MCPServer) handleCallTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ms.logger.InfoContext(ctx, "Tool call received", "tool", req.Params.Name)

	if isOutputTool(req.Params.Name) {
		argsMap, err := parseArguments(req)
		if err != nil {
			ms.logger.ErrorContext(ctx, "Failed to unmarshal output tool arguments", "error", err)
			return nil, err
		}
		return ms.handleOutputTool(req.Params.Name, argsMap)
	}

	var selectedTool *config.Tool
	for _, t := range ms.cfg.Tools {
		if t.Name == req.Params.Name {
			selectedTool = &t
			break
		}
	}

	if selectedTool == nil {
		ms.logger.WarnContext(ctx, "Tool not found", "tool", req.Params.Name)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Tool not found"}},
			IsError: true,
		}, nil
	}

	argsMap, err := parseArguments(req)
	if err != nil {
		ms.logger.ErrorContext(ctx, "Failed to unmarshal arguments", "error", err)
		return nil, err
	}

	cliArgs, err := buildArgs(selectedTool, argsMap)
	if err != nil {
		ms.logger.WarnContext(ctx, "Argument build error", "tool", selectedTool.Name, "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	image := ""
	if selectedTool.Docker != nil {
		image = selectedTool.Docker.Image
	}
	ms.logger.InfoContext(ctx, "Executing tool", "command", selectedTool.Command, "args_count", len(cliArgs), "image", image)
	result, err := executor.Execute(ctx, selectedTool, cliArgs)
	if err != nil {
		ms.logger.ErrorContext(ctx, "Execution failure", "tool", selectedTool.Name, "error", err)
		message := executionErrorMessage(err, result.Output)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: message}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result.Output}},
	}, nil
}

func (ms *MCPServer) registerOutputTools() {
	ms.srv.AddTool(&mcp.Tool{
		Name:        readOutputToolName,
		Description: "Read a bounded byte range from a persisted mcpwn command output.",
		InputSchema: rawSchema(map[string]interface{}{
			"output_id": map[string]interface{}{"type": "string", "description": "Output ID returned by a truncated tool run."},
			"offset":    map[string]interface{}{"type": "number", "description": "Byte offset to start reading from. Defaults to 0."},
			"limit":     map[string]interface{}{"type": "number", "description": "Maximum bytes to read. Defaults to 65536 and is capped at 1048576."},
		}, []string{"output_id"}),
	}, ms.handleCallTool)

	ms.srv.AddTool(&mcp.Tool{
		Name:        tailOutputToolName,
		Description: "Read the last lines from a persisted mcpwn command output.",
		InputSchema: rawSchema(map[string]interface{}{
			"output_id": map[string]interface{}{"type": "string", "description": "Output ID returned by a truncated tool run."},
			"lines":     map[string]interface{}{"type": "number", "description": "Number of lines to read. Defaults to 100 and is capped at 1000."},
		}, []string{"output_id"}),
	}, ms.handleCallTool)

	ms.srv.AddTool(&mcp.Tool{
		Name:        searchOutputToolName,
		Description: "Search for a literal string in a persisted mcpwn command output.",
		InputSchema: rawSchema(map[string]interface{}{
			"output_id":   map[string]interface{}{"type": "string", "description": "Output ID returned by a truncated tool run."},
			"query":       map[string]interface{}{"type": "string", "description": "Literal string to search for."},
			"max_matches": map[string]interface{}{"type": "number", "description": "Maximum number of matches. Defaults to 20 and is capped at 100."},
		}, []string{"output_id", "query"}),
	}, ms.handleCallTool)
}

func isOutputTool(name string) bool {
	switch name {
	case readOutputToolName, tailOutputToolName, searchOutputToolName:
		return true
	default:
		return false
	}
}

func parseArguments(req *mcp.CallToolRequest) (map[string]interface{}, error) {
	argsMap := make(map[string]interface{})
	if req.Params.Arguments == nil {
		return argsMap, nil
	}
	if err := json.Unmarshal(req.Params.Arguments, &argsMap); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return argsMap, nil
}

func (ms *MCPServer) handleOutputTool(name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	switch name {
	case readOutputToolName:
		return ms.handleReadOutput(args)
	case tailOutputToolName:
		return ms.handleTailOutput(args)
	case searchOutputToolName:
		return ms.handleSearchOutput(args)
	default:
		return errorResult("Output tool not found"), nil
	}
}

func (ms *MCPServer) handleReadOutput(args map[string]interface{}) (*mcp.CallToolResult, error) {
	outputID, err := requiredStringArg(args, "output_id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	offset, err := optionalIntArg(args, "offset", 0, 0, math.MaxInt64)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	limit, err := optionalIntArg(args, "limit", defaultReadLimit, 1, maxReadLimit)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	store, err := output.DefaultStore()
	if err != nil {
		return errorResult(err.Error()), nil
	}
	result, err := store.Read(outputID, offset, limit)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	text := string(result.Data)
	text += fmt.Sprintf("\n[INFO] offset=%d next_offset=%d size=%d eof=%t", result.Offset, result.NextOffset, result.Size, result.EOF)
	return textResult(text), nil
}

func (ms *MCPServer) handleTailOutput(args map[string]interface{}) (*mcp.CallToolResult, error) {
	outputID, err := requiredStringArg(args, "output_id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	lines, err := optionalIntArg(args, "lines", defaultTailLines, 1, maxTailLines)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	store, err := output.DefaultStore()
	if err != nil {
		return errorResult(err.Error()), nil
	}
	text, err := store.Tail(outputID, int(lines))
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (ms *MCPServer) handleSearchOutput(args map[string]interface{}) (*mcp.CallToolResult, error) {
	outputID, err := requiredStringArg(args, "output_id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	query, err := requiredStringArg(args, "query")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	maxMatches, err := optionalIntArg(args, "max_matches", defaultSearchMatches, 1, maxSearchMatches)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	store, err := output.DefaultStore()
	if err != nil {
		return errorResult(err.Error()), nil
	}
	matches, err := store.Search(outputID, query, int(maxMatches))
	if err != nil {
		return errorResult(err.Error()), nil
	}
	return textResult(formatSearchMatches(matches)), nil
}

func requiredStringArg(args map[string]interface{}, name string) (string, error) {
	raw, ok := args[name]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", name)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("parameter %s must be a non-empty string", name)
	}
	return value, nil
}

func optionalIntArg(args map[string]interface{}, name string, defaultValue, minValue, maxValue int64) (int64, error) {
	raw, ok := args[name]
	if !ok {
		return defaultValue, nil
	}

	value, ok := numericArg(raw)
	if !ok {
		return 0, fmt.Errorf("parameter %s must be an integer", name)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("parameter %s must be between %d and %d", name, minValue, maxValue)
	}
	return value, nil
}

func numericArg(raw interface{}) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		if math.Trunc(value) != value {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}

func formatSearchMatches(matches []output.SearchMatch) string {
	if len(matches) == 0 {
		return "No matches found."
	}

	var builder strings.Builder
	for _, match := range matches {
		fmt.Fprintf(&builder, "%d:%s\n", match.Line, match.Text)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}
}

func executionErrorMessage(err error, output string) string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return "Execution Error: " + err.Error()
	}

	return fmt.Sprintf("Execution Error: %v\nOutput:\n%s", err, output)
}

func generateSchema(t config.Tool) json.RawMessage {
	props := make(map[string]interface{})
	var required []string
	for _, arg := range t.Args {
		pType := "string"
		if arg.Type == "boolean" {
			pType = "boolean"
		}
		props[arg.Name] = map[string]interface{}{"type": pType, "description": arg.Description}
		if arg.Required {
			required = append(required, arg.Name)
		}
	}
	return rawSchema(props, required)
}

func rawSchema(props map[string]interface{}, required []string) json.RawMessage {
	b, _ := json.Marshal(map[string]interface{}{
		"type": "object", "properties": props, "required": required,
	})
	return b
}

// buildArgs translates the incoming MCP map of arguments into a CLI-ready slice of strings
func buildArgs(t *config.Tool, inputs map[string]interface{}) ([]string, error) {
	args := append([]string{}, t.FixedArgs...)
	var positional []string

	for _, def := range t.Args {
		val, exists := inputs[def.Name]
		if !exists {
			if def.Required {
				return nil, fmt.Errorf("missing required parameter: %s", def.Name)
			}
			continue
		}

		if def.Type == "boolean" {
			if b, ok := val.(bool); ok && b {
				args = append(args, def.Flag)
			}
		} else {
			sVal := fmt.Sprintf("%v", val)
			if def.Positional {
				positional = append(positional, sVal)
			} else if def.Flag == "" {
				// If no flag is defined, split the value and add as raw arguments
				parts := strings.Fields(sVal)
				args = append(args, parts...)
			} else {
				args = append(args, def.Flag, sVal)
			}
		}
	}
	return append(args, positional...), nil
}
