![logo](logo.png)

**mcpwn** is an MCP server that allows Large Language Models (LLMs) to execute security tools on your local machine. It currently supports **7** tools out of the box: `nmap`, `gobuster`, `ffuf`, `httpx`, `sqlmap`, `msfvenom`, and `msfconsole`.

## Table of Contents

- [Features](#features)
- [Project Structure](#project-structure)
- [Installation](#installation)
  - [Prerequisites and Info](#prerequisites-and-info)
  - [Security Architecture](#security-architecture)
  - [Build from source](#build-from-source)
  - [Automated Builds & Packaging](#automated-builds--packaging)
- [Configuration](#configuration)
  - [Example mcpwn.yaml](#example-mcpwnyaml)
- [Usage](#usage)
  - [Run Manually](#run-manually)
  - [Integration with Claude and Gemini (WIP)](#integration-with-claude-and-gemini-wip)
- [License](#license)
- [Contact](#contact)

## Features

- **Model Context Protocol**: fully compliant with the Model Context Protocol. it uses the [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk).
- **Dynamic tool registration**: define tools (like `nmap`, `gobuster`, etc...) via a simple `mcpwn.yaml` file.
- **Flexible arguments**: supports `extra_args` to pass any valid CLI flag to tools, providing full flexibility.
- **Docker integration**: runs tools in isolated containers for security and easy dependency management.
- **Cross-platform**: compiles for Linux, macOS, and Windows. Automated releases are built via Goreleaser, and `.deb` packaging is supported for Debian-based systems.

## Project Structure

```text
├── cmd
│   └── mcpwn                
│       └── main.go
├── debian
├── internal
│   ├── config                // YAML configuration loader
│   │   ├── config.go
│   │   └── config_test.go
│   ├── executor              
│   │   ├── run.go
│   │   └── run_test.go
│   └── server                // MCP protocol implementation and tool routing
│       ├── server.go
│       └── server_test.go
├── mcpwn.yaml                // Default MCP tools configuration out-of-the-box
└── go.mod & go.sum
```

## Installation

### Prerequisites and Info

- [Go 1.25](https://go.dev/dl/) or later.
- Docker

You can use `mcpwn` with your locally installed tools or with Docker (recommended).

If you decide not to use Docker, you can still use it but the security tools you want to use (e.g., `nmap`) must be installed and in your system `PATH`.

By default, tools run inside **Docker containers**, which provides a strong layer of isolation.

### Security Architecture

mcpwn is designed to prevent unauthorized access to your host machine:

1.  **Direct execution**: The server uses Go's `exec.Command`, which executes binaries directly without involving a system shell (like `/bin/sh` or `cmd.exe`). This means that special characters are passed as literal arguments rather than being interpreted as command separators.
2.  **Docker isolation**: Tools run in ephemeral containers with defaults:
    - `--cap-drop=ALL` and `--security-opt=no-new-privileges`
    - `--read-only`: The container's root filesystem is mounted as read-only.
    - **Resource Limits**: Containers are conservatively limited to `512MB` of RAM and `1` CPU.
3.  **Granular Customization**: Through `mcpwn.yaml`, you can selectively override these defaults per tool:
    - Mount temporary in-memory filesystems (`tmp_dirs: ["/tmp", "/root"]`).
    - Disable CPU limits for heavy multithreaded fuzzers (`cpus: "0"`).
    - Expose selected network interfaces (`network: "host"`) or paths (`volumes: []`).
    - Grant explicit capabilities (e.g., `capabilities: ["NET_RAW"]`).
4.  **Fire and forget**: Containers are run with the `--rm` flag.

### Build from source

```bash
git clone https://gitlab.com/parrotsec/project/mcpwn.git
cd mcpwn
go build -o mcpwn cmd/mcpwn/main.go
```

Furthermore, `mcpwn` is cross-platform, so you can build the project for GNU/Linux, macOS, and Windows. 

### Automated Builds & Packaging

We can use GoReleaser for automated build and creation of `.tar.gz` and `.deb` archives:
```bash
goreleaser build --snapshot --clean
```

For Debian-based systems like ParrotOS, a standard `debian/` directory is also provided for creating packages natively:
```bash
dpkg-buildpackage -us -uc -b # or via sbuild
```

## Configuration

By default, `mcpwn` looks for a configuration file named `mcpwn.yaml` in the same directory as the executable. For system-wide installations, it also checks for a configuration file at `/etc/mcpwn/mcpwn.yaml`.

You can easily extend `mcpwn` by adding new tool definitions to this file. **Docker support** is also available: if you specify a `docker` configuration block for a tool, `mcpwn` will automatically run it inside a temporary Docker container.

### Example `mcpwn.yaml`

```yaml
tools:
  - name: "nmap_scan"
    description: "Nmap is a free and open source utility for network discovery and security auditing."
    command: "nmap"
    docker:
      image: "parrotsec/nmap"
      capabilities: ["NET_RAW", "NET_ADMIN"]
      memory: "512m"
      cpus: "1"
    args:
      - name: "target"
        description: "Target IP/Domain"
        required: true
        positional: true
      - name: "aggressive"
        description: "Aggressive scan (-A)"
        flag: "-A"
        type: "boolean"
      - name: "extra_args"
        description: "Any additional nmap arguments"
        flag: ""
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

#### Gemini CLI

To use `mcpwn` with the [Gemini CLI](https://github.com/google-gemini/gemini-cli), follow these steps:

1. **Build the binary**:
   ```bash
   go build -o mcpwn cmd/mcpwn/main.go
   ```

2. **Register the server**: run the following command to add `mcpwn` to your Gemini CLI configuration (using your current absolute path):
   ```bash
   gemini mcp add mcpwn $(pwd)/mcpwn
   ```

3. **Start Gemini**:
   ```bash
   gemini
   ```
   ![Gemini CLI with mcpwn](example.png)

4. **Use**: inside the Gemini session, 
5. you can verify the connection by typing `/mcp list`. You can then ask the model to run your tools, e.g.:
   > Scan localhost using nmap_scan in fast mode.
   > Search for vulnerabilities on 192.168.1.1 using nmap_scan with the 'vuln' script.

*Note: If you modify `mcpwn.yaml`, you must restart the Gemini CLI session to refresh the tool definitions.*

## License

This project is licensed under the GPL v3.

## Contact

For further information and implementation, please contact `danterolle@parrotsec.org` or `team@parrotsec.org`.