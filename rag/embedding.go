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

type xunfeiEmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions"`
}

type xunfeiEmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type xunfeiEmbeddingResponse struct {
	ID    string                  `json:"id"`
	Data  []xunfeiEmbeddingData   `json:"data"`
	Model string                  `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *HTTPEmbeddingService) Embed(ctx context.Context, text string) (Vector, error) {
	req := xunfeiEmbeddingRequest{
		Model:      s.config.Model,
		Input:      []string{text},
		Dimensions: s.config.Dimensions,
	}

	var resp xunfeiEmbeddingResponse
	r := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp)

	_, err := r.Post("/v2/embeddings")
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	embedLen := len(resp.Data[0].Embedding)
	if embedLen == 0 {
		return nil, fmt.Errorf("embedding vector is empty")
	}
	if embedLen != s.config.Dimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", s.config.Dimensions, embedLen)
	}

	return Vector(resp.Data[0].Embedding), nil
}