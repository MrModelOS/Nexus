package internal

type Command struct {
	cmd  string
	desc string
}

func getCommands() []Command {
	return []Command{
		{"/agent", "Agent loop status/control"},
		{"/agent on", "Enable agent loop"},
		{"/agent off", "Disable agent loop"},
		{"/autofix", "Run tests/linters auto-check"},
		{"/autofix on", "Enable auto-fix after edits"},
		{"/autofix off", "Disable auto-fix"},
		{"/clear", "Clear chat history"},
		{"/commit", "Generate commit message"},
		{"/compact", "Compress conversation history"},
		{"/context", "Show project structure"},
		{"/costs", "Show token usage and costs"},
		{"/ctx", "Show project structure (alias)"},
		{"/diff", "Show git diff"},
		{"/fetch", "Fetch URL content"},
		{"/git", "Show git status"},
		{"/help", "Show all commands"},
		{"/index", "Index codebase for search"},
		{"/mcp", "List MCP servers"},
		{"/mcp call", "Call MCP tool: /mcp call <server> <tool>"},
		{"/model", "Fetch and select model from Ollama"},
		{"/models", "Interactive model picker"},
		{"/pool", "Show model pool status and failover"},
		{"/perm", "Show permissions"},
		{"/perm grant", "Grant permission: /perm grant <path> <level>"},
		{"/perm revoke", "Revoke permission"},
		{"/plugins", "List installed plugins"},
		{"/prompt save", "Save prompt template"},
		{"/prompt use", "Load prompt template"},
		{"/prompts", "List saved prompts"},
		{"/remember", "Save insight to project memory"},
		{"/review", "Review staged git changes"},
		{"/search", "Search files with ripgrep"},
		{"/sessions", "Manage saved sessions"},
		{"/skills", "Show available skills"},
		{"/think", "Toggle thinking display"},
		{"/tokens", "Show token usage and context status"},
		{"/tools", "List available tools"},
		{"/usage", "Show token usage (alias)"},
		{"/url", "Search and fetch URLs"},
		{"/vault", "Show vault status"},
		{"/vault set", "Set vault entry"},
		{"/vault unlock", "Unlock vault with key"},
		{"/vim", "Toggle vim mode"},
		{"/watch", "Start file watcher"},
		{"/watch stop", "Stop file watcher"},
		{"/quit", "Exit Nexus"},
	}
}
