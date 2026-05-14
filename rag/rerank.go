package rag

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
)

type HTTPRerankConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func DefaultHTTPRerankConfig(apiKey string) HTTPRerankConfig {
	return HTTPRerankConfig{
		APIKey:  apiKey,
		BaseURL: "",
		Model:   "rerank",
	}
}

type HTTPRerankService struct {
	client *resty.Client
	config HTTPRerankConfig
}

func NewHTTPRerankService(config HTTPRerankConfig) *HTTPRerankService {
	client := resty.New().
		SetBaseURL(config.BaseURL).
		SetHeader("Authorization", "Bearer "+config.APIKey).
		SetHeader("Content-Type", "application/json")

	return &HTTPRerankService{
		client: client,
		config: config,
	}
}

type xunfeiRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type xunfeiRerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
	Document       *struct {
		Text string `json:"text"`
	} `json:"document,omitempty"`
}

type xunfeiRerankResponse struct {
	Model   string                `json:"model"`
	Results []xunfeiRerankResult  `json:"results"`
	Usage   struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *HTTPRerankService) Rerank(ctx context.Context, query string, candidates []Chunk, topK int) ([]Chunk, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	docs := make([]string, len(candidates))
	for i, chunk := range candidates {
		docs[i] = chunk.Content
	}

	req := xunfeiRerankRequest{
		Model:     s.config.Model,
		Query:     query,
		Documents: docs,
	}

	var resp xunfeiRerankResponse
	r := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp)

	_, err := r.Post("/v2/rerank")
	if err != nil {
		return nil, fmt.Errorf("failed to call rerank API: %w", err)
	}

	result := make([]Chunk, len(resp.Results))
	for i, item := range resp.Results {
		if item.Index >= 0 && item.Index < len(candidates) {
			result[i] = candidates[item.Index]
		}
	}

	return result, nil
}