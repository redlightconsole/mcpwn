package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"mcpwn/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	httpRequestToolName = "http_request"

	defaultHTTPTimeoutSeconds int64 = 30
	maxHTTPTimeoutSeconds     int64 = 300
	maxHTTPPreviewBytes             = 1024 * 1024
)

// registerHTTPTool registers the native http_request tool, which sends an
// arbitrary HTTP request using Go's standard net/http package instead of
// shelling out to an external command.
func (ms *MCPServer) registerHTTPTool() {
	ms.srv.AddTool(&mcp.Tool{
		Name:        httpRequestToolName,
		Description: "Send a raw HTTP request using Go's standard net/http package and return the full response (status line, headers, and body).",
		InputSchema: rawSchema(map[string]interface{}{
			"url":                  map[string]interface{}{"type": "string", "description": "Target URL, including scheme (http:// or https://)."},
			"method":               map[string]interface{}{"type": "string", "description": "HTTP method (GET, POST, PUT, DELETE, ...). Defaults to GET."},
			"headers":              map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"type": "string"}, "description": "Request headers as a map of name to value. A Host header overrides the request Host."},
			"body":                 map[string]interface{}{"type": "string", "description": "Raw request body sent as-is."},
			"follow_redirects":     map[string]interface{}{"type": "boolean", "description": "Follow 3xx redirects. Defaults to true."},
			"insecure_skip_verify": map[string]interface{}{"type": "boolean", "description": "Skip TLS certificate verification. Defaults to false."},
			"timeout":              map[string]interface{}{"type": "number", "description": "Request timeout in seconds. Defaults to 30 and is capped at 300."},
		}, []string{"url"}),
	}, ms.handleCallTool)
}

func (ms *MCPServer) handleHTTPRequest(args map[string]interface{}) (*mcp.CallToolResult, error) {
	rawURL, err := requiredStringArg(args, "url")
	if err != nil {
		return errorResult(err.Error()), nil
	}

	method := strings.ToUpper(optionalStringArg(args, "method", http.MethodGet))

	headers, err := stringMapArg(args, "headers")
	if err != nil {
		return errorResult(err.Error()), nil
	}

	body := optionalStringArg(args, "body", "")

	followRedirects, err := optionalBoolArg(args, "follow_redirects", true)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	skipVerify, err := optionalBoolArg(args, "insecure_skip_verify", false)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	timeoutSeconds, err := optionalIntArg(args, "timeout", defaultHTTPTimeoutSeconds, 1, maxHTTPTimeoutSeconds)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build request: %v", err)), nil
	}

	for name, value := range headers {
		if strings.EqualFold(name, "Host") {
			req.Host = value
			continue
		}
		req.Header.Set(name, value)
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}
	if !followRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	ms.logger.InfoContext(ctx, "Executing http_request", "method", method, "url", rawURL, "follow_redirects", followRedirects, "insecure_skip_verify", skipVerify)

	resp, err := client.Do(req)
	if err != nil {
		ms.logger.ErrorContext(ctx, "http_request failed", "error", err)
		return errorResult(fmt.Sprintf("request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to read response body: %v", err)), nil
	}

	return ms.httpResult(formatHTTPResponse(resp, respBody)), nil
}

// formatHTTPResponse renders a response as raw HTTP text: status line, sorted
// headers, a blank line, then the body.
func formatHTTPResponse(resp *http.Response, body []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", resp.Proto, resp.Status)

	names := make([]string, 0, len(resp.Header))
	for name := range resp.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, value := range resp.Header[name] {
			fmt.Fprintf(&b, "%s: %s\n", name, value)
		}
	}

	b.WriteString("\n")
	b.Write(body)
	return b.String()
}

// httpResult persists the full response to the output store so large responses
// can be paged with the mcpwn_*_output tools, returning a truncated preview when
// the response exceeds the preview cap.
func (ms *MCPServer) httpResult(full string) *mcp.CallToolResult {
	store, err := output.DefaultStore()
	if err != nil {
		return textResult(full)
	}
	entry, writer, err := store.Create()
	if err != nil {
		return textResult(full)
	}
	if _, err := io.WriteString(writer, full); err != nil {
		writer.Close()
		return textResult(full)
	}
	if err := writer.Close(); err != nil {
		return textResult(full)
	}

	if len(full) <= maxHTTPPreviewBytes {
		return textResult(full)
	}

	preview := full[:maxHTTPPreviewBytes]
	preview += fmt.Sprintf("\n[WARN] Response exceeded %d bytes and was truncated.", maxHTTPPreviewBytes)
	preview += fmt.Sprintf("\n[INFO] Full output saved with output_id: %s", entry.ID)
	return textResult(preview)
}

// stringMapArg converts an object argument into a map[string]string, erroring on
// non-string values. A missing argument yields a nil map.
func stringMapArg(args map[string]interface{}, name string) (map[string]string, error) {
	raw, ok := args[name]
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter %s must be an object of string values", name)
	}
	result := make(map[string]string, len(m))
	for key, value := range m {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("parameter %s[%q] must be a string", name, key)
		}
		result[key] = str
	}
	return result, nil
}
