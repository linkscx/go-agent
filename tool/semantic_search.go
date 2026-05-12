package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"go-agent/rag"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

const AgentToolSemanticSearch AgentTool = "semantic_search"

type SemanticSearchTool struct {
	searcher *rag.SemanticSearcher
}

func NewSemanticSearchTool(searcher *rag.SemanticSearcher) *SemanticSearchTool {
	return &SemanticSearchTool{
		searcher: searcher,
	}
}

func (t *SemanticSearchTool) ToolName() AgentTool {
	return AgentToolSemanticSearch
}

func (t *SemanticSearchTool) Info() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        string(AgentToolSemanticSearch),
		Description: openai.String("Search for relevant code snippets or documentation in the codebase using semantic similarity. Returns the most relevant chunks with their file paths and content."),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query describing what you're looking for",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"description": "Number of results to return (default: 5, max: 20)",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
	})
}

type SemanticSearchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (t *SemanticSearchTool) Execute(ctx context.Context, argumentsInJSON string) (string, error) {
	var args SemanticSearchArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.TopK == 0 {
		args.TopK = 5
	}
	if args.TopK > 20 {
		args.TopK = 20
	}

	results, err := t.searcher.Search(ctx, args.Query, args.TopK)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No relevant results found.", nil
	}

	var output string
	for i, result := range results {
		output += fmt.Sprintf("## Result %d (Score: %.4f)\n", i+1, result.Score)
		output += fmt.Sprintf("**File:** %s\n", result.FilePath)
		if result.StartLine > 0 {
			output += fmt.Sprintf("**Lines:** %d-%d\n", result.StartLine, result.EndLine)
		}
		output += fmt.Sprintf("**Content:**\n```\n%s\n```\n\n", result.Content)
	}

	return output, nil
}
