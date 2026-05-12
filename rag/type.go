package rag

import (
	"context"
	"time"
)

type Vector = []float32

type VectorPoint struct {
	Vector    Vector    `json:"vector"`
	Chunk     Chunk     `json:"chunk"`
	CreatedAt time.Time `json:"created_at"` // 使用文件修改时间作为创建时间
}

type Chunk struct {
	Content string `json:"content"`
	Meta    Meta   `json:"meta"`
}

type Meta struct {
	StartPos   int    `json:"start_pos"`   // 起始位置（可以是行号、字符偏移、段落索引等）
	EndPos     int    `json:"end_pos"`     // 结束位置
	DocumentID string `json:"document_id"` // 文档唯一标识（文件路径、URL、ID等）
}

type VectorPointResult struct {
	VectorPoint
	Score float32 `json:"score"`
}

type SearchResult struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
	Score     float32
}

type RerankService interface {
	Rerank(ctx context.Context, query string, candidates []Chunk, topK int) ([]Chunk, error)
}

type EmbeddingService interface {
	Embed(ctx context.Context, chunk string) (Vector, error)
}

// ChunkerService 文档切分服务接口
type ChunkerService interface {
	Chunk(documentID, content string) []Chunk
}

type VectorStore interface {
	// InsertBatch 批量插入向量点
	InsertBatch(ctx context.Context, vps []VectorPoint) error

	// Search 执行向量相似度搜索，返回最相似的结果
	Search(ctx context.Context, queryVector Vector, limit int) ([]VectorPointResult, error)

	// SearchWithScore 执行向量相似度搜索，返回带相似度分数的结果
	SearchWithScore(ctx context.Context, queryVector Vector, limit int) ([]VectorPointResult, error)

	// DeleteByDocument 删除指定文档的所有向量
	DeleteByDocument(ctx context.Context, documentID string) error

	// GetDocumentIndexedTime 获取文档的索引时间，用于去重判断
	// 返回零值时间表示文档不存在
	GetDocumentIndexedTime(ctx context.Context, documentID string) (time.Time, error)

	// Clear 清空所有向量数据
	Clear(ctx context.Context) error

	// Close 关闭连接
	Close() error
}

type SemanticSearcher struct {
	embedService  EmbeddingService
	vectorStore   VectorStore
	rerankService RerankService
}

func NewSemanticSearcher(embedService EmbeddingService, vectorStore VectorStore, rerankService RerankService) *SemanticSearcher {
	return &SemanticSearcher{
		embedService:  embedService,
		vectorStore:   vectorStore,
		rerankService: rerankService,
	}
}

func (s *SemanticSearcher) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	queryVector, err := s.embedService.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	vectorResults, err := s.vectorStore.Search(ctx, queryVector, topK*2)
	if err != nil {
		return nil, err
	}

	if len(vectorResults) == 0 {
		return []SearchResult{}, nil
	}

	candidates := make([]Chunk, len(vectorResults))
	for i, result := range vectorResults {
		candidates[i] = result.Chunk
	}

	var rerankedChunks []Chunk
	if s.rerankService != nil {
		rerankedChunks, err = s.rerankService.Rerank(ctx, query, candidates, topK)
		if err != nil || len(rerankedChunks) == 0 {
			rerankedChunks = candidates
			if len(rerankedChunks) > topK {
				rerankedChunks = rerankedChunks[:topK]
			}
		}
	} else {
		rerankedChunks = candidates
		if len(rerankedChunks) > topK {
			rerankedChunks = rerankedChunks[:topK]
		}
	}

	results := make([]SearchResult, len(rerankedChunks))
	for i, chunk := range rerankedChunks {
		results[i] = SearchResult{
			FilePath:  chunk.Meta.DocumentID,
			StartLine: chunk.Meta.StartPos,
			EndLine:   chunk.Meta.EndPos,
			Content:   chunk.Content,
			Score:     0.0,
		}
	}

	return results, nil
}
