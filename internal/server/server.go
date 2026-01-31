package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mcpwn/internal/config"
	"mcpwn/internal/executor"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	argsMap := make(map[string]interface{})
	if req.Params.Arguments != nil {
		if err := json.Unmarshal(req.Params.Arguments, &argsMap); err != nil {
			ms.logger.ErrorContext(ctx, "Failed to unmarshal arguments", "error", err)
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	cliArgs, err := buildArgs(selectedTool, argsMap)
	if err != nil {
		ms.logger.WarnContext(ctx, "Argument build error", "tool", selectedTool.Name, "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
	}

	ms.logger.InfoContext(ctx, "Executing tool", "command", selectedTool.Command, "args", cliArgs, "image", selectedTool.Image)
	output, err := executor.SafeExecute(ctx, selectedTool.Command, cliArgs, selectedTool.Image)
	if err != nil {
		ms.logger.ErrorContext(ctx, "Execution failure", "tool", selectedTool.Name, "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "System Error: " + err.Error()}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil
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
			} else {
				args = append(args, def.Flag, sVal)
			}
		}
	}
	return append(args, positional...), nil
}
