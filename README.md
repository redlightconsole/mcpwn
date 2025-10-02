# mcpwn

A web application to remotely execute security tools and shell commands.

It consists of two main components:
- **`api-server`**: The worker. It receives commands via a REST API and executes them on the host machine.
- **`mcp-server`**: The controller. It acts as a user-facing proxy that delegates tasks to the `api-server`.

## Project Structure

```
mcpwn/
├── cmd/
│   ├── api-server/main.go   # API server entrypoint
│   └── mcp-server/main.go   # MCP server entrypoint
├── internal/
│   ├── api/                 # API handlers and router
│   ├── client/              # Client for the api-server
│   ├── command/             # Command execution logic
│   └── models/              # Data structures
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.25
- Required command-line tools (e.g., `nmap`) must be installed and available in the `PATH` of the `api-server`'s machine.

## Quickstart

The API server and the MCP server run on the same machine.

Open two terminals and run the following commands:

### 1. Run the api-server

```bash
go build -o api-server ./cmd/api-server
./api-server --port 5000
```

- --port: Port for the API server (default: 5000).
- --timeout: Default command timeout in seconds (default: 180).

### 2. Run the mcp-server
```bash
go build -o mcp-server ./cmd/mcp-server
./mcp-server --port 8000 --server "http://localhost:5000"
```

- --port: Port for the MCP server (default: 8000).
- --server: URL of the API server (default: http://localhost:5000).

## API Usage

All requests should be sent to the **mcp-server**.

```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"command": "whoami"}' \
  http://localhost:8000/tools/command
```

```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"target": "scanme.nmap.org", "ports": "80,443"}' \
  http://localhost:8000/tools/nmap
```

Also, you can check the api-server status and available tools by hitting its `/health` endpoint directly:

```bash
curl http://localhost:5000/health
```