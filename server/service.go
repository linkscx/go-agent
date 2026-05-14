package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"

	"go-agent/agent"
	"go-agent/shared"
)

type Service struct {
	agent *agent.Agent
	db    *DB
}

func NewService(agentInstance *agent.Agent, db *DB) *Service {
	return &Service{
		agent: agentInstance,
		db:    db,
	}
}

type StreamEvent struct {
	MessageID        string `json:"message_id"`
	Event            string `json:"event"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCall         string `json:"tool_call,omitempty"`
	ToolArguments    string `json:"tool_arguments,omitempty"`
	ToolResult       string `json:"tool_result,omitempty"`
}

type RunResult struct {
	Response string
	Rounds   []shared.OpenAIMessage
	Usage    openai.CompletionUsage
}

func (s *Service) CreateConversation(ctx context.Context, title string) (*Conversation, error) {
	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *Service) GetConversation(ctx context.Context, conversationID string) (*Conversation, error) {
	return s.db.GetConversation(ctx, conversationID)
}

func (s *Service) ListConversations(ctx context.Context) ([]*Conversation, error) {
	return s.db.ListConversations(ctx)
}

func (s *Service) UpdateConversation(ctx context.Context, conversationID string, title *string, archived *bool) (*Conversation, error) {
	if title != nil {
		if err := s.db.UpdateConversationTitle(ctx, conversationID, *title); err != nil {
			return nil, err
		}
	}
	if archived != nil {
		if err := s.db.ArchiveConversation(ctx, conversationID, *archived); err != nil {
			return nil, err
		}
	}
	return s.db.GetConversation(ctx, conversationID)
}

func (s *Service) DeleteConversation(ctx context.Context, conversationID string) error {
	return s.db.DeleteConversation(ctx, conversationID)
}

func (s *Service) ListMessages(ctx context.Context, conversationID string) ([]*ChatMessage, error) {
	return s.db.ListMessages(ctx, conversationID)
}

func (s *Service) SendMessage(ctx context.Context, conversationID, parentMessageID, userQuery string, eventCh chan<- StreamEvent) (*ChatMessage, error) {
	history, err := s.buildHistory(ctx, conversationID, parentMessageID)
	if err != nil {
		return nil, fmt.Errorf("failed to build history: %w", err)
	}

	userMsg := &ChatMessage{
		ID:              uuid.New().String(),
		ConversationID:  conversationID,
		ParentMessageID: parentMessageID,
		Role:            "user",
		Content:         userQuery,
		Rounds:          mustMarshalJSON([]openai.ChatCompletionMessageParamUnion{openai.UserMessage(userQuery)}),
		CreatedAt:       time.Now(),
	}
	if err := s.db.CreateMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	assistantMsgID := uuid.New().String()

	s.agent.ResetSession()
	s.agent.SeedHistory(history)

	viewCh := make(chan agent.MessageVO, 100)
	confirmCh := make(chan agent.ConfirmationAction, 1)

	go func() {
		defer close(viewCh)
		if err := s.agent.RunStreaming(ctx, userQuery, viewCh, confirmCh); err != nil {
			log.Printf("agent error: %v", err)
			eventCh <- StreamEvent{MessageID: assistantMsgID, Event: "error", Content: err.Error()}
		}
	}()

	var assistantContent string

	for vo := range viewCh {
		switch vo.Type {
		case agent.MessageTypeContent:
			if vo.Content != nil {
				assistantContent += *vo.Content
				eventCh <- StreamEvent{MessageID: assistantMsgID, Event: "content", Content: *vo.Content}
			}
		case agent.MessageTypeReasoning:
			if vo.ReasoningContent != nil {
				eventCh <- StreamEvent{MessageID: assistantMsgID, Event: "reasoning", ReasoningContent: *vo.ReasoningContent}
			}
		case agent.MessageTypeToolCall:
			if vo.ToolCall != nil {
				eventCh <- StreamEvent{
					MessageID:     assistantMsgID,
					Event:         "tool_call",
					ToolCall:      vo.ToolCall.Name,
					ToolArguments: vo.ToolCall.Arguments,
				}
			}
		case agent.MessageTypeError:
			if vo.Content != nil {
				eventCh <- StreamEvent{MessageID: assistantMsgID, Event: "error", Content: *vo.Content}
			}
		case agent.MessageTypeToolConfirm:
			confirmCh <- agent.ConfirmAllow
		}
	}

	roundMessages := s.agent.GetTurnMessages()

	assistantMsg := &ChatMessage{
		ID:              assistantMsgID,
		ConversationID:  conversationID,
		ParentMessageID: userMsg.ID,
		Role:            "assistant",
		Content:         assistantContent,
		Rounds:          mustMarshalJSON(roundMessages),
		CreatedAt:       time.Now(),
	}
	if err := s.db.CreateMessage(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}

	if err := s.db.UpdateConversation(ctx, conversationID, time.Now()); err != nil {
		log.Printf("failed to update conversation: %v", err)
	}

	return assistantMsg, nil
}

func (s *Service) buildHistory(ctx context.Context, conversationID, parentMessageID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	if parentMessageID == "" {
		return []openai.ChatCompletionMessageParamUnion{}, nil
	}

	messages, err := s.db.GetMessageChain(ctx, conversationID, parentMessageID)
	if err != nil {
		return nil, err
	}

	var history []openai.ChatCompletionMessageParamUnion
	for _, msg := range messages {
		var rounds []openai.ChatCompletionMessageParamUnion
		if err := json.Unmarshal([]byte(msg.Rounds), &rounds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rounds: %w", err)
		}
		history = append(history, rounds...)
	}

	return history, nil
}

func mustMarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
