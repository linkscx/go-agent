package context

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/openai/openai-go/v3"

	"go-agent/memory"
	"go-agent/shared"
	"go-agent/skill"
)

type messageWrap struct {
	Message shared.OpenAIMessage
	Tokens  int
}

type Engine struct {
	systemPromptTemplate string
	messages             []messageWrap
	policies             []Policy
	onPolicyEvent        func(policyName string, running bool, err error)
	onMemoryEvent        func(running bool, err error)
	contextTokens        int
	contextWindow        int
	conversationID       string

	memory       memory.Memory
	skillManager SkillManager
}

type TokenBudget struct {
	ContextWindow int
}

type Usage struct {
	PromptTokens int
}

type TurnDraft struct {
	NewMessages []shared.OpenAIMessage
}

// SkillManager is the interface for accessing skill metadata
type SkillManager interface {
	FormatForPrompt() string
}

func NewContextEngine(memory memory.Memory, policies []Policy) *Engine {
	skillManager := skill.NewManager()
	_ = skillManager.LoadAll()
	return &Engine{
		policies:      policies,
		messages:      make([]messageWrap, 0),
		contextWindow: 200000,
		memory:        memory,
		skillManager:  skillManager,
	}
}

func (c *Engine) Init(systemPrompt string, budget TokenBudget) {
	c.systemPromptTemplate = systemPrompt
	if budget.ContextWindow > 0 {
		c.contextWindow = budget.ContextWindow
	}
}

func (c *Engine) BuildRequestMessages() []shared.OpenAIMessage {
	result := make([]shared.OpenAIMessage, 0, len(c.messages)+1)
	if c.systemPromptTemplate != "" {
		result = append(result, openai.SystemMessage(c.BuildSystemPrompt()))
	}
	for i := range c.messages {
		result = append(result, c.messages[i].Message)
	}
	return result
}

func (c *Engine) StartTurn(userMsg shared.OpenAIMessage) TurnDraft {
	return TurnDraft{
		NewMessages: []shared.OpenAIMessage{userMsg},
	}
}

func (c *Engine) CommitTurn(ctx context.Context, draft TurnDraft, usage Usage, skipPoliciesAndMemory bool) error {
	// 根据情况压缩上下文
	for i := range draft.NewMessages {
		msg := draft.NewMessages[i]
		c.messages = append(c.messages, messageWrap{Message: msg, Tokens: CountTokens(msg)})
	}
	c.recountTokens()

	if skipPoliciesAndMemory {
		return nil
	}

	if err := c.applyPolicies(ctx); err != nil {
		return err
	}
	// 只有当用户明确或暗示需要记忆时才更新记忆
	if c.shouldUpdateMemory(draft.NewMessages) {
		if c.onMemoryEvent != nil {
			c.onMemoryEvent(true, nil)
		}
		err := c.memory.Update(ctx, draft.NewMessages)
		if c.onMemoryEvent != nil {
			c.onMemoryEvent(false, err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Engine) AbortTurn(_ TurnDraft) {
	// no-op: draft is only in-memory and never committed unless CommitTurn is called.
}

func (c *Engine) GetMessages() []shared.OpenAIMessage {
	result := make([]shared.OpenAIMessage, len(c.messages))
	for i := range c.messages {
		result[i] = c.messages[i].Message
	}
	return result
}

func (c *Engine) GetContextUsage() float64 {
	if c.contextWindow <= 0 {
		return 0
	}
	return float64(c.contextTokens) / float64(c.contextWindow)
}

func (c *Engine) recountTokens() {
	totalTokens := 0
	for i := range c.messages {
		totalTokens += c.messages[i].Tokens
	}
	c.contextTokens = totalTokens
}

func (c *Engine) applyPolicies(ctx context.Context) error {
	for _, policy := range c.policies {
		if !policy.ShouldApply(ctx, c) {
			continue
		}
		if c.onPolicyEvent != nil {
			c.onPolicyEvent(policy.Name(), true, nil)
		}
		result, err := policy.Apply(ctx, c)
		if c.onPolicyEvent != nil {
			c.onPolicyEvent(policy.Name(), false, err)
		}
		if err != nil {
			return fmt.Errorf("apply policy %s: %w", policy.Name(), err)
		}
		c.messages = result.Messages
		c.recountTokens()
	}
	return nil
}

func (c *Engine) SetPolicyEventHook(hook func(policyName string, running bool, err error)) {
	c.onPolicyEvent = hook
}

func (c *Engine) SetMemoryEventHook(hook func(running bool, err error)) {
	c.onMemoryEvent = hook
}

func (c *Engine) SetConversationID(id string) {
	c.conversationID = id
}

func (c *Engine) GetConversationID() string {
	return c.conversationID
}

func (c *Engine) BuildSystemPrompt() string {
	replaceMap := make(map[string]string)
	replaceMap["{runtime}"] = runtime.GOOS
	replaceMap["{workspace_path}"] = shared.GetWorkspaceDir()

	if c.memory != nil {
		replaceMap["{memory}"] = c.memory.String()
	} else {
		replaceMap["{memory}"] = ""
	}

	// Add skills metadata
	if c.skillManager != nil {
		replaceMap["{skills}"] = c.skillManager.FormatForPrompt()
	} else {
		replaceMap["{skills}"] = ""
	}

	prompt := c.systemPromptTemplate
	for k, v := range replaceMap {
		prompt = strings.ReplaceAll(prompt, k, v)
	}
	return prompt
}

// SeedMessages loads external history into the engine (after Reset).
func (c *Engine) SeedMessages(msgs []shared.OpenAIMessage) {
	for _, msg := range msgs {
		c.messages = append(c.messages, messageWrap{Message: msg, Tokens: CountTokens(msg)})
	}
	c.recountTokens()
}

var memoryTriggerPhrases = []string{
	"remember", "don't forget", "keep in mind", "save this",
	"note this", "for future reference", "keep this", "store this",
	"记忆", "记住", "保存", "记下", "别忘了", "备忘",
}

func (c *Engine) shouldUpdateMemory(messages []shared.OpenAIMessage) bool {
	for _, msg := range messages {
		role := shared.GetRoleName(msg)
		if role != "user" {
			continue
		}
		contentAny := msg.GetContent().AsAny()
		contentStr, ok := contentAny.(*string)
		if !ok || contentStr == nil {
			continue
		}
		lower := strings.ToLower(*contentStr)
		for _, phrase := range memoryTriggerPhrases {
			if strings.Contains(lower, phrase) {
				return true
			}
		}
	}
	return false
}

func (c *Engine) ForceCompact(ctx context.Context) error {
	for _, policy := range c.policies {
		if policy.Name() == "summarize" {
			if c.onPolicyEvent != nil {
				c.onPolicyEvent(policy.Name(), true, nil)
			}
			result, err := policy.Apply(ctx, c)
			if c.onPolicyEvent != nil {
				c.onPolicyEvent(policy.Name(), false, err)
			}
			if err != nil {
				return fmt.Errorf("force compact: %w", err)
			}
			c.messages = result.Messages
			c.recountTokens()
			return nil
		}
	}
	return nil
}

func (c *Engine) Reset() {
	c.messages = make([]messageWrap, 0)
	c.contextTokens = 0
}
