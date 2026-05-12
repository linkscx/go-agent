package agent

type ConfirmationAction int

const (
	ConfirmAllow ConfirmationAction = iota
	ConfirmReject
	ConfirmAlwaysAllow
)

const (
	MessageTypeReasoning   = "reasoning"
	MessageTypeContent     = "content"
	MessageTypeToolCall    = "tool_call"
	MessageTypeError       = "error"
	MessageTypePolicy      = "policy"
	MessageTypeMemory      = "memory"
	MessageTypeToolConfirm = "tool_confirm"
)

type MessageVO struct {
	Type string `json:"type"`

	ReasoningContent        *string             `json:"reasoning_content,omitempty"`
	Content                 *string             `json:"content,omitempty"`
	ToolCall                *ToolCallVO         `json:"tool,omitempty"`
	Policy                  *PolicyVO           `json:"policy,omitempty"`
	Memory                  *MemoryVO           `json:"memory,omitempty"`
	ToolConfirmationRequest *ToolConfirmationVO `json:"tool_confirmation_request,omitempty"`
}

type PolicyVO struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   error  `json:"error"`
}

type MemoryVO struct {
	Running bool  `json:"running"`
	Error   error `json:"error"`
}

type ToolCallVO struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolConfirmationVO struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
}
