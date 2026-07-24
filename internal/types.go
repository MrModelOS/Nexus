package internal

type ChatMessage struct {
	Role    string
	Content string
}

type StreamTokenMsg struct {
	Token string
}

type StreamDoneMsg struct {
	FullResponse string
	Error        error
}

type ToolResultMsg struct {
	Call   *ToolCall
	Result string
}
