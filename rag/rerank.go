package rag

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// HTTPRerankConfig HTTP Rerank服务配置
type HTTPRerankConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// DefaultHTTPRerankConfig 默认配置
func DefaultHTTPRerankConfig(apiKey string) HTTPRerankConfig {
	return HTTPRerankConfig{
		APIKey:  apiKey,
		BaseURL: "",
		Model:   "rerank",
	}
}

// HTTPRerankService HTTP Rerank服务实现
type HTTPRerankService struct {
	client *resty.Client
	config HTTPRerankConfig
}

// NewHTTPRerankService 创建HTTP Rerank服务
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

// rerankDocument 文档结构
type rerankDocument struct {
	Text string `json:"text,omitempty"`
}

// rerankInput 输入结构
type rerankInput struct {
	Query     rerankDocument   `json:"query"`
	Documents []rerankDocument `json:"documents"`
}

// rerankParameters 参数结构
type rerankParameters struct {
	ReturnDocuments bool `json:"return_documents"`
	TopN            int  `json:"top_n"`
}

// rerankRequest Rerank请求
type rerankRequest struct {
	Model      string           `json:"model"`
	Input      rerankInput      `json:"input"`
	Parameters rerankParameters `json:"parameters"`
}

// rerankResult Rerank结果
type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
	Document       *struct {
		Text string `json:"text"`
	} `json:"document,omitempty"`
}

// rerankResponse Rerank响应
type rerankResponse struct {
	Output struct {
		Results []rerankResult `json:"results"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

// Rerank 对候选文档进行重排序
// 参数:
//   - ctx: 上下文
//   - query: 查询文本
//   - candidates: 候选文档块
//   - topK: 返回前 topK 个结果
func (s *HTTPRerankService) Rerank(ctx context.Context, query string, candidates []Chunk, topK int) ([]Chunk, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	docs := make([]rerankDocument, len(candidates))
	for i, chunk := range candidates {
		docs[i] = rerankDocument{Text: chunk.Content}
	}

	req := rerankRequest{
		Model: s.config.Model,
		Input: rerankInput{
			Query:     rerankDocument{Text: query},
			Documents: docs,
		},
		Parameters: rerankParameters{
			ReturnDocuments: false,
			TopN:            topK, // 只返回 topK 个结果
		},
	}

	var resp rerankResponse
	r := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp)

	_, err := r.Post("/services/rerank/text-rerank/text-rerank")
	if err != nil {
		return nil, fmt.Errorf("failed to call rerank API: %w", err)
	}

	result := make([]Chunk, len(resp.Output.Results))
	for i, item := range resp.Output.Results {
		result[i] = candidates[item.Index]
	}

	return result, nil
}
