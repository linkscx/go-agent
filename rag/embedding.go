package rag

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
)

type HTTPEmbeddingConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
}

func DefaultHTTPEmbeddingConfig(apiKey string) HTTPEmbeddingConfig {
	return HTTPEmbeddingConfig{
		APIKey:     apiKey,
		BaseURL:    "",
		Model:      "embedding",
		Dimensions: 512,
	}
}

type HTTPEmbeddingService struct {
	client *resty.Client
	config HTTPEmbeddingConfig
}

func NewHTTPEmbeddingService(config HTTPEmbeddingConfig) *HTTPEmbeddingService {
	client := resty.New().
		SetBaseURL(config.BaseURL).
		SetHeader("Authorization", "Bearer "+config.APIKey).
		SetHeader("Content-Type", "application/json")

	return &HTTPEmbeddingService{
		client: client,
		config: config,
	}
}

type dashscopeParameters struct {
	Dimension int `json:"dimension"`
}

type dashscopeRequest struct {
	Model string `json:"model"`
	Input struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	} `json:"input"`
	Parameters dashscopeParameters `json:"parameters"`
}

type dashscopeEmbedding struct {
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
}

type dashscopeResponse struct {
	Output struct {
		Embeddings []dashscopeEmbedding `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

func (s *HTTPEmbeddingService) Embed(ctx context.Context, text string) (Vector, error) {
	req := dashscopeRequest{
		Model: s.config.Model,
		Parameters: dashscopeParameters{
			Dimension: s.config.Dimensions,
		},
	}
	req.Input.Contents = []struct {
		Text string `json:"text"`
	}{
		{Text: text},
	}

	var resp dashscopeResponse
	r := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp)

	_, err := r.Post("/services/embeddings/multimodal-embedding/multimodal-embedding")
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}

	if len(resp.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	embedLen := len(resp.Output.Embeddings[0].Embedding)
	if embedLen == 0 {
		return nil, fmt.Errorf("embedding vector is empty")
	}
	if embedLen != s.config.Dimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", s.config.Dimensions, embedLen)
	}

	return Vector(resp.Output.Embeddings[0].Embedding), nil
}
