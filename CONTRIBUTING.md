# Contributing to Nexus

Thank you for your interest in contributing to Nexus! This document provides guidelines and information for contributors.

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/nexus.git
   cd nexus
   ```
3. Create a branch:
   ```bash
   git checkout -b feature/my-feature
   ```
4. Make your changes
5. Test your changes:
   ```bash
   make build
   ./nex
   ```
6. Commit and push:
   ```bash
   git add .
   git commit -m "Add my feature"
   git push origin feature/my-feature
   ```
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Ollama (for testing)
- ripgrep (optional, for search features)

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Install to ~/.local/bin
make install
```

### Running Tests

```bash
make test
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Run `make lint` before committing

## Project Structure

```
nexus/
├── main.go           # Entry point
├── cmd/              # CLI commands (Cobra)
├── client/           # AI provider clients
├── config/           # Configuration handling
└── internal/         # Core TUI and features
    ├── tui.go        # Main TUI loop
    ├── tools.go      # Tool registry
    ├── agent.go      # Agent loop
    └── ...
```

## Adding a New Tool

1. Create a new file in `internal/` (e.g., `mytool.go`)

2. Implement the `Tool` interface:
   ```go
   type MyTool struct{}
   
   func (t *MyTool) Name() string { return "my_tool" }
   func (t *MyTool) Description() string { return "Does something useful" }
   func (t *MyTool) Schema() map[string]interface{} {
       return map[string]interface{}{
           "type": "object",
           "properties": map[string]interface{}{
               "param": map[string]interface{}{
                   "type": "string",
               },
           },
       }
   }
   func (t *MyTool) Execute(args map[string]interface{}) (string, error) {
       // Your implementation
       return "result", nil
   }
   ```

3. Register in `internal/tools.go`:
   ```go
   func NewToolRegistry() *ToolRegistry {
       r := &ToolRegistry{...}
       // ... existing tools ...
       r.Register(&MyTool{})
       return r
   }
   ```

## Adding a New Command

1. Add to `internal/commands.go`:
   ```go
   func getCommands() []Command {
       return []Command{
           // ... existing commands ...
           {"/mycommand", "Description of my command"},
       }
   }
   ```

2. Handle in `internal/tui.go`:
   ```go
   func (m *Model) handleCommand(value string) []tea.Cmd {
       // ... existing handlers ...
       
       if value == "/mycommand" {
           m.output = append(m.output, "Command executed!")
           m.textinput.SetValue("")
           m.updateLayout()
           return nil
       }
       // ...
   }
   ```

## Adding a New Provider

1. Add provider type to `client/multi.go`:
   ```go
   const (
       ProviderMyProvider ProviderType = "myprovider"
   )
   ```

2. Implement chat methods:
   ```go
   func (c *MultiProviderClient) chatMyProvider(...) (string, error) {
       // Your implementation
   }
   ```

3. Add to switch statements in `Chat()` and `StreamChat()`

## Reporting Issues

- Use GitHub Issues
- Include steps to reproduce
- Include OS and Go version
- Include any error messages

## Feature Requests

- Open an issue with the "enhancement" label
- Describe the use case
- Provide examples if possible

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Code of Conduct

- Be respectful
- Help others learn
- Focus on constructive feedback
- Welcome newcomers

Thank you for contributing! 🎉
