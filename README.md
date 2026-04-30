# CTF-Agent

An AI-powered CTF solving agent that automatically analyzes and solves CTF challenges using Claude AI. Features a web interface for viewing solving history and a knowledge base system.

## Features

- **Web Security**: SQL injection, XSS, CSRF, file upload vulnerabilities
- **Binary Exploitation (Pwn)**: Buffer overflow, format string, heap vulnerabilities
- **Cryptography (Crypto)**: RSA, AES, classical ciphers, hash collisions
- **Reverse Engineering**: Disassembly, decompilation, dynamic analysis
- **Web Interface**: Dashboard with statistics, session history, and knowledge base
- **Knowledge Base**: Automatic extraction of solving techniques and patterns

## Installation

```bash
# Clone the repository
git clone https://github.com/Conly-Zy/CTF-Agent.git
cd CTF-Agent

# Build the binary
make build

# Or install globally
go install ./cmd/ctf-agent
```

## Configuration

Create a `config.yaml` file or set environment variables:

```yaml
anthropic:
  api_key: "your-api-key-here"
  model: "claude-opus-4-7"

agent:
  max_iterations: 50
  timeout: 10m
  verbose: false

sandbox:
  enabled: true
  image: "ctf-agent-sandbox:latest"
  timeout: 60s
```

Or set environment variable:

```bash
export ANTHROPIC_API_KEY="your-api-key-here"
```

## Usage

### CLI Mode

#### Solve a Web Challenge

```bash
ctf-agent solve \
  --type web \
  --description "Find the flag in this web application" \
  --target "http://challenge.example.com"
```

#### Solve a Binary Challenge

```bash
ctf-agent solve \
  --type pwn \
  --description "Exploit this binary to get the flag" \
  --files ./challenge.elf
```

#### Solve a Crypto Challenge

```bash
ctf-agent solve \
  --type crypto \
  --description "Decrypt this ciphertext" \
  --files ./ciphertext.txt
```

#### Solve a Reverse Challenge

```bash
ctf-agent solve \
  --type reverse \
  --description "Reverse engineer this binary" \
  --files ./binary.exe
```

### Web Interface

Start the web server:

```bash
ctf-agent server --addr :8080 --db ctf-agent.db
```

Then open http://localhost:8080 in your browser.

#### Web Interface Features

- **Dashboard**: View statistics, success rate, and recent sessions
- **Sessions**: Browse all solving sessions with filtering by type and status
- **Knowledge Base**: Search and view extracted techniques and patterns
- **Markdown Rendering**: Knowledge entries are rendered with full Markdown support including code highlighting

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Web Interface                          │
│            Dashboard | Sessions | Knowledge Base             │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    HTTP API Server                           │
│              (net/http + RESTful API)                        │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Agent Orchestrator                        │
│            (multi-turn agentic loop)                         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    LLM Interface (Claude API)                │
│              (tool use, streaming, thinking)                  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Tool Registry                             │
├────────┬────────┬────────┬────────┬─────────┬───────────────┤
│  Web   │  Pwn   │ Crypto │ Reverse│ Common  │  Knowledge    │
└────────┴────────┴────────┴────────┴─────────┴───────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Data Store (SQLite)                        │
│         Sessions | Knowledge | Tags | Conversations          │
└─────────────────────────────────────────────────────────────┘
```

## Knowledge Base System

The knowledge base automatically extracts and stores:

- **Solving Techniques**: Methods used to solve challenges
- **Vulnerability Patterns**: Common vulnerability types and exploitation methods
- **Tool Usage**: Tools and commands used during solving
- **Code Snippets**: Generated exploit scripts and solutions

Each knowledge entry includes:
- Markdown-formatted content with code highlighting
- Automatic tag extraction
- Session reference for context

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/dashboard` | Dashboard statistics |
| GET | `/api/sessions` | List sessions |
| GET | `/api/sessions/:id` | Get session details |
| GET | `/api/sessions/:id/messages` | Get session conversation |
| GET | `/api/knowledge` | List knowledge entries |
| GET | `/api/knowledge/:id` | Get knowledge details |
| GET | `/api/knowledge/search?q=` | Search knowledge |
| GET | `/api/tags` | List all tags |
| GET | `/api/stats` | Get statistics |

## Development

### Prerequisites

- Go 1.21+
- Docker (for sandbox execution)
- Claude API key

### Build

```bash
make build
```

### Test

```bash
make test
```

### Format Code

```bash
make fmt
```

## Project Structure

```
CTF-Agent/
├── cmd/ctf-agent/          # CLI entry point
│   ├── main.go             # Root command
│   ├── solve.go            # Solve command
│   └── server.go           # Server command
├── internal/
│   ├── agent/              # Agent orchestrator
│   ├── api/                # HTTP API server
│   ├── knowledge/          # Knowledge extraction
│   ├── llm/                # Claude API client
│   ├── tools/              # Tool implementations
│   │   ├── common/         # General tools
│   │   ├── web/            # Web security tools
│   │   ├── pwn/            # Binary exploitation tools
│   │   ├── crypto/         # Cryptography tools
│   │   └── reverse/        # Reverse engineering tools
│   ├── sandbox/            # Docker sandbox
│   ├── challenge/          # Challenge classification
│   ├── submitter/          # Flag submission
│   ├── store/              # SQLite data persistence
│   └── config/             # Configuration
├── web/                    # Frontend assets
│   ├── index.html          # Main page
│   └── static/             # CSS, JS files
├── pkg/                    # Public utilities
├── configs/                # Configuration files
└── scripts/                # Setup scripts
```

## License

MIT License
