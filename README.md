# mcpwn

`mcpwn` is an MCP server that allows Large Language Models (LLMs) to execute security tools on your local machine.

## Features

- **Model Context Protocol**: fully compliant with the Model Context Protocol. it uses the [modelcontextprotocol/go-sdk](github.com/modelcontextprotocol/go-sdk).
- **Dynamic tool registration**: define tools (like `nmap`, `gobuster`, etc...) via a simple `mcpwn.yaml` file.
- **Cross-platform**: compiles for Linux, macOS, and Windows.

## Installation

### Prerequisites

- [Go 1.25](https://go.dev/dl/) or later.
- Security tools you want to use (e.g., `nmap`) must be installed and in your system `PATH`.

## Security Warning

This tool allows an LLM to execute commands on your machine.

**Only use it with models you trust and in environments where execution is safe.**
The tool implements basic safety checks, but it does not replace a proper sandbox.

### Build from source

```bash
git clone https://gitlab.com/parrotsec/project/mcpwn.git
cd mcpwn
go build -o mcpwn cmd/mcpwn/main.go
```

## Configuration

Tools are defined in the `mcpwn.yaml` file located in the same directory as the executable.

### Example `mcpwn.yaml`

```yaml
tools:
  - name: "nmap_scan"
    description: "Nmap is a free and open source utility for network discovery and security auditing."
    command: "nmap"
    args:
      - name: "target"
        description: "Target IP/Domain"
        required: true
        positional: true
      - name: "ports"
        description: "Ports to scan (e.g. '80,443' or '-p-')"
        flag: "-p"
      - name: "fast_mode"
        description: "Fast scan (-F)"
        flag: "-F"
        type: "boolean"
```

## Usage

### Run Manually

You can run the server directly to test if it loads your configuration correctly:

```bash
./mcpwn
```

The server communicates via `Stdio` and you will see log messages on `Stderr`.

### Integration with Claude and Gemini (WIP)

#### Claude Code

Run the following command in your terminal:
```bash
claude mcp add --transport stdio mcpwn -- /path/to/your/mcpwn
```

#### Gemini
To use `mcpwn` with **Gemini** (via Gemini CLI or other MCP-compatible Google clients), ensure your environment supports MCP and add the server to your settings:

```json
{
  "mcpServers": {
    "mcpwn": {
      "command": "/path/to/your/mcpwn",
      "args": [],
      "transport": "stdio"
    }
  }
}
```

*Note: Replace `/path/to/your/mcpwn` with the absolute path to your compiled binary.*

## Project Structure

```
├── LICENSE
├── README.md
├── cmd
│   └── mcpwn
│       └── main.go
├── go.mod
├── go.sum
├── internal // logic for loading the YAML configuration.
│   ├── config
│   │   └── config.go
│   ├── executor
│   │   └── run.go
│   └── server // MCP protocol implementation and tool routing.
│       └── server.go
├── mcpwn.yaml
└── scripts
    └── build.sh
```

## License

This project is licensed under the GPL v3.