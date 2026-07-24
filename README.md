<p align="center">
  <img src="assets/nexus-logo.png" alt="Nexus" width="120">
</p>

<h1 align="center">Nexus</h1>

<p align="center">
  <strong>The Most Powerful AI CLI Tool for Developers</strong>
</p>

<p align="center">
  <a href="#installation">Install</a> •
  <a href="#features">Features</a> •
  <a href="#usage">Usage</a> •
  <a href="#configuration">Config</a> •
  <a href="#plugins">Plugins</a> •
  <a href="#contributing">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-blue.svg" alt="Version">
  <img src="https://img.shields.io/badge/go-1.21+-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey.svg" alt="Platform">
</p>

---

**Nexus** is a terminal-based AI assistant that helps you write, edit, and understand code. It connects to local models via Ollama or cloud providers like OpenAI and Anthropic, giving you a ChatGPT-like experience directly in your terminal.

```
 ┌─────────────────────────────────────────────────────────┐
 │  nexus  🔧  ollama/qwen2.5-coder                      │
 │─────────────────────────────────────────────────────────│
 │                                                         │
 │  >_ Nexus                                               │
 │  model:     qwen2.5-coder /models to change            │
 │  directory: ~/projects/my-app                           │
 │  agent:     build                                      │
 │                                                         │
 │  Tip: Shift+Tab to switch agent • Tab cycle perms      │
 │                                                         │
 │─────────────────────────────────────────────────────────│
 │ ❯ Ask me anything...                                   │
 │                                                         │
 │ Tab:Auto • ↑↓ scroll • Enter send            0 msgs    │
 └─────────────────────────────────────────────────────────┘
```

---

## Features

### 🤖 AI Agent Loop

Nexus can autonomously plan, execute, and verify multi-step tasks:

```
You: refactor the auth module to use JWT tokens

[THINK] Analyzing the current auth implementation...
[PLAN] 1. Read auth.go 2. Add JWT library 3. Update handlers 4. Run tests
[ACT] Reading auth.go...
[VERIFY] File read successfully
[ACT] Writing updated auth.go with JWT support...
[VERIFY] Tests passing
[REFLECT] Auth module successfully refactored to use JWT
```

### 💭 Thinking Stream

Watch the AI reason in real-time:

```
💭 Thinking (842ms):
  Let me analyze the codebase structure...
  I see this is a Go project with multiple packages...
  The user wants to add authentication...
```

### 🔧 Built-in Tools

Nexus has 8 built-in tools the AI can use automatically:

| Tool | Description |
|------|-------------|
| `read_file` | Read file contents |
| `write_file` | Create or overwrite files |
| `edit_file` | Precise text replacement |
| `delete_file` | Remove files |
| `list_dir` | List directory contents |
| `run_command` | Execute shell commands |
| `grep` | Search with ripgrep |
| `glob` | Find files by pattern |

### 🔐 Secure Vault

Encrypted credential storage with AES-256:

```bash
/vault unlock my-master-password
/vault set github_token ghp_xxxxxxxxxxxx
/vault set api_key sk-xxxxxxxxxxxx
```

### 💰 Cost Tracking

Track token usage and costs per session:

```bash
/costs

Session Usage:
  gpt-4o: 15 requests, ~45000 tokens, $0.2340
  Total: $0.2340
  Duration: 12m 34s
```

### 🔌 Plugin System

Extend Nexus with custom tools:

```bash
~/.config/nexus/plugins/my-tool/plugin.json
```

### 📺 File Watcher

Monitor files for changes:

```bash
/watch          # Start watching
/watch stop     # Stop watching
```

### 🎨 Interactive Diff

Side-by-side diff viewer:

```bash
/diff main.go   # Show diff for file
```

---

## Installation

### Install Script (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/MrModelOS/Nexus/master/install.sh | sh
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/MrModelOS/Nexus/releases):

```bash
# Linux (amd64)
curl -L https://github.com/MrModelOS/Nexus/releases/download/v1.0.0/nex-linux-amd64 -o nex
chmod +x nex
sudo mv nex /usr/local/bin/

# macOS (arm64)
curl -L https://github.com/MrModelOS/Nexus/releases/download/v1.0.0/nex-darwin-arm64 -o nex
chmod +x nex
sudo mv nex /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/MrModelOS/Nexus.git
cd Nexus
make install
```

Or manually:

```bash
go build -o nexus .
cp nexus ~/.local/bin/
```

### Prerequisites

- **Go 1.21+** (for building from source)
- **Ollama** (for local models) — [Install Ollama](https://ollama.ai)
- **ripgrep** (optional, for search) — `sudo pacman -S ripgrep`

---

## Quick Start

### 1. Configure Nexus

```bash
# Interactive setup
nex init

# Or edit config directly
vim ~/.config/nexus/config.yaml
```

### 2. Pull a Model

```bash
ollama pull qwen2.5-coder
```

### 3. Start Nexus

```bash
nex
```

### 4. Try These Commands

```
/help           # Show all commands
/models         # Pick a model
/search fmt     # Search code
/context        # Show project structure
/agent refactor the main module    # Multi-step task
```

---

## Usage

### Basic Chat

```
❯ Write a function to parse JSON
```

### Reference Files

```
❯ Analyze @main.go and suggest improvements
```

### Run Commands

```
❯ What does this project do? /context
```

### Multi-Step Agent

```
❯ /agent Add unit tests for the parser module
```

### Search Code

```
❯ /search TODO
```

---

## Configuration

Config file: `~/.config/nexus/config.yaml`

```yaml
default_model: qwen2.5-coder
temperature: 0.7
system_prompt: You are Nexus, a helpful AI assistant.

providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434
    models:
      - qwen2.5-coder
      - llama3.2
      - gemma3:1b

  openai:
    type: openai
    base_url: https://api.openai.com
    api_key: sk-your-key-here
    models:
      - gpt-4o
      - gpt-4o-mini

  anthropic:
    type: anthropic
    base_url: https://api.anthropic.com
    api_key: sk-ant-your-key-here
    models:
      - claude-sonnet-4-20250514
      - claude-3-5-haiku-20241022
```

---

## Commands Reference

### Navigation

| Key | Action |
|-----|--------|
| `↑/↓` | Scroll output |
| `PgUp/PgDn` | Half page scroll |
| `Enter` | Send message |
| `Tab` | Cycle permission mode |
| `Shift+Tab` | Toggle agent (build/plan) |
| `Esc` | Dismiss hints / quit |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show all commands |
| `/clear` | Clear chat history |
| `/models` | Interactive model picker |
| `/search <query>` | Search files with ripgrep |
| `/context` | Show project structure |
| `/git` | Show git status |
| `/commit` | Generate commit message |
| `/review` | Review staged changes |
| `/autofix` | Run tests/linters |
| `/agent <goal>` | Start agent loop |
| `/think` | Toggle thinking display |
| `/vault` | Credential management |
| `/costs` | Show token usage |
| `/plugins` | List installed plugins |
| `/prompts` | Manage prompt templates |
| `/watch` | Start file watcher |
| `/diff <file>` | View file diff |
| `/perm` | Manage permissions |
| `/skills` | Load skill files |
| `/sessions` | Manage saved sessions |
| `/compact` | Compress history |
| `/quit` | Exit Nexus |

### File References

Prefix with `@` to include file content:

```
Explain @src/main.go
```

### Skill Activation

Prefix with `$` to load a skill:

```
$review This code for security issues
```

---

## Plugins

### Creating a Plugin

Create `~/.config/nexus/plugins/my-tool/plugin.json`:

```json
{
  "name": "my-tool",
  "description": "My custom tool",
  "version": "1.0.0",
  "tools": [
    {
      "name": "deploy",
      "description": "Deploy to production",
      "schema": {
        "type": "object",
        "properties": {
          "env": {"type": "string"}
        }
      },
      "command": "echo Deploying to $NEXUS_TOOL_ARGS"
    }
  ]
}
```

### Using Plugins

```bash
/plugins           # List installed plugins
/tools             # See all available tools
```

The AI can then use your custom tools automatically.

---

## Skills

Skills are markdown files that provide specialized instructions.

### Creating a Skill

Create `~/.config/nexus/skills/review.md`:

```markdown
---
name: review
description: Code review assistance
---

When reviewing code:
1. Check for security vulnerabilities
2. Look for performance issues
3. Verify error handling
4. Suggest improvements
```

### Using Skills

```
$review Check this function for issues
```

---

## Agent Modes

### Build Mode (Default)

Full access to file system and commands. The AI can read, write, and execute freely.

### Plan Mode

Read-only analysis. The AI can only read files and suggest changes without modifying anything.

Toggle with `Shift+Tab`.

---

## Permission Modes

| Mode | Description |
|------|-------------|
| **Manual** | Ask before every action |
| **Accept Edits** | Auto-approve file edits |
| **Plan** | Read-only, suggest only |
| **Auto** | Full auto-execution |

Cycle with `Tab`.

---

## Project Structure

```
nexus/
├── main.go                 # Entry point
├── cmd/
│   ├── root.go            # Root command
│   ├── ask.go             # Quick ask command
│   ├── models.go          # List models
│   └── init.go            # Interactive setup
├── client/
│   ├── client.go          # Ollama client
│   └── multi.go           # Multi-provider
├── config/
│   └── config.go          # YAML config
├── internal/
│   ├── tui.go             # Main TUI (1700+ lines)
│   ├── tools.go           # Tool registry
│   ├── agent.go           # Agent loop
│   ├── thinking.go        # AI reasoning
│   ├── plugins.go         # Plugin system
│   ├── vault.go           # Encrypted vault
│   ├── costs.go           # Cost tracking
│   ├── prompts.go         # Prompt templates
│   ├── watcher.go         # File monitoring
│   ├── diff.go            # Diff viewer
│   ├── permissions.go     # Access control
│   ├── context.go         # Context management
│   ├── search.go          # Ripgrep integration
│   ├── session.go         # Session persistence
│   ├── skills.go          # Skill system
│   ├── compact.go         # History compression
│   ├── agents.go          # Agent profiles
│   ├── project.go         # Project context
│   ├── git.go             # Git integration
│   ├── mcp.go             # MCP protocol
│   ├── autofix.go         # Auto-fix loop
│   └── banner.go          # Startup banner
└── assets/
    └── nexus-logo.png     # Logo
```

---

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling
- [Glamour](https://github.com/charmbracelet/glamour) — Markdown rendering
- [Ollama](https://ollama.ai) — Local AI models
- [Ripgrep](https://github.com/BurntSushi/ripgrep) — Fast search

---

<p align="center">
  Made with ❤️ by the Nexus community
</p>
