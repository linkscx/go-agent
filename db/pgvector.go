package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	shared "go-agent/rag"
)

// PGVectorStore 使用 pgvector 存储和检索向量
type PGVectorStore struct {
	db        *gorm.DB
	dimension int
}

// DocumentChunk 文档块模型
type DocumentChunk struct {
	ID         uint      `gorm:"primaryKey"`
	Content    string    `gorm:"type:text;not null"`
	DocumentID string    `gorm:"type:text;not null;index"`
	StartPos   int       `gorm:"not null"`
	EndPos     int       `gorm:"not null"`
	Embedding  string    `gorm:"type:vector(512)"`
	CreatedAt  time.Time // 使用文件修改时间，不是 autoCreateTime
}

// TableName 指定表名
func (DocumentChunk) TableName() string {
	return "document_chunks"
}

// ToChunk 转换为 shared.Chunk
func (d *DocumentChunk) ToChunk() shared.Chunk {
	return shared.Chunk{
		Content: d.Content,
		Meta: shared.Meta{
			DocumentID: d.DocumentID,
			StartPos:   d.StartPos,
			EndPos:     d.EndPos,
		},
	}
}

// Config pgvector 配置
type Config struct {
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	Dimension int
}

// NewPGVectorStore 创建新的 pgvector 存储
func NewPGVectorStore(config Config) (*PGVectorStore, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&DocumentChunk{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 创建向量索引（如果维度支持）
	if config.Dimension <= 2000 {
		sql := `
			CREATE INDEX IF NOT EXISTS idx_document_chunks_embedding 
			ON document_chunks USING ivfflat (embedding vector_cosine_ops) 
			WITH (lists = 100)
		`
		db.Exec(sql)
	}

	return &PGVectorStore{
		db:        db,
		dimension: config.Dimension,
	}, nil
}

// vectorToPGVector 将 float32 slice 转换为 PostgreSQL vector 字符串格式
func vectorToPGVector(v []float32) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, f := range v {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%f", f))
	}
	sb.WriteString("]")
	return sb.String()
}

// pgVectorToVector 将 PostgreSQL vector 字符串格式转换为 float32 slice
func pgVectorToVector(s string) []float32 {
	// 移除方括号
	s = strings.Trim(s, "[]")
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]float32, len(parts))
	for i, p := range parts {
		var f float64
		fmt.Sscanf(p, "%f", &f)
		result[i] = float32(f)
	}
	return result
}

// InsertBatch 批量插入向量
func (s *PGVectorStore) InsertBatch(ctx context.Context, vps []shared.VectorPoint) error {
	docs := make([]*DocumentChunk, len(vps))
	for i, vp := range vps {
		if len(vp.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch at index %d: expected %d, got %d", i, s.dimension, len(vp.Vector))
		}

		// 如果没有设置 CreatedAt，使用当前时间
		createdAt := vp.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		docs[i] = &DocumentChunk{
			Content:    vp.Chunk.Content,
			DocumentID: vp.Chunk.Meta.DocumentID,
			StartPos:   vp.Chunk.Meta.StartPos,
			EndPos:     vp.Chunk.Meta.EndPos,
			Embedding:  vectorToPGVector(vp.Vector),
			CreatedAt:  createdAt,
		}
	}

	return s.db.WithContext(ctx).CreateInBatches(docs, 100).Error
}

// Search 执行向量相似度搜索
func (s *PGVectorStore) Search(ctx context.Context, queryVector shared.Vector, limit int) ([]shared.VectorPointResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	vectorStr := vectorToPGVector(queryVector)

	var results []DocumentChunk
	err := s.db.WithContext(ctx).Raw(`
		SELECT *, embedding <=> ? AS distance
		FROM document_chunks
		ORDER BY embedding <=> ?
		LIMIT ?
	`, vectorStr, vectorStr, limit).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 转换为结果格式
	vectorResults := make([]shared.VectorPointResult, len(results))
	for i, r := range results {
		vectorResults[i] = shared.VectorPointResult{
			VectorPoint: shared.VectorPoint{
				Vector: pgVectorToVector(r.Embedding),
				Chunk:  r.ToChunk(),
			},
			Score: 1.0, // 余弦相似度需要转换
		}
	}

	return vectorResults, nil
}

// SearchWithScore 执行向量相似度搜索并返回相似度分数
func (s *PGVectorStore) SearchWithScore(ctx context.Context, queryVector shared.Vector, limit int) ([]shared.VectorPointResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	vectorStr := vectorToPGVector(queryVector)

	var results []struct {
		DocumentChunk
		Distance float64 `gorm:"column:distance"`
	}

	err := s.db.WithContext(ctx).Raw(`
		SELECT *, embedding <=> ? AS distance
		FROM document_chunks
		ORDER BY embedding <=> ?
		LIMIT ?
	`, vectorStr, vectorStr, limit).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 转换为结果格式，余弦距离转换为相似度分数
	vectorResults := make([]shared.VectorPointResult, len(results))
	for i, r := range results {
		// 余弦距离范围是 [0, 2]，转换为相似度 [1, -1]，再归一化到 [1, 0]
		similarity := 1.0 - r.Distance/2.0
		vectorResults[i] = shared.VectorPointResult{
			VectorPoint: shared.VectorPoint{
				Vector: pgVectorToVector(r.Embedding),
				Chunk:  r.ToChunk(),
			},
			Score: float32(similarity),
		}
	}

	return vectorResults, nil
}

// DeleteByDocument 删除指定文档的所有向量
func (s *PGVectorStore) DeleteByDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Where("document_id = ?", documentID).Delete(&DocumentChunk{}).Error
}

// GetDocumentIndexedTime 获取文档的索引时间（返回最早的索引时间）
func (s *PGVectorStore) GetDocumentIndexedTime(ctx context.Context, documentID string) (time.Time, error) {
	var doc DocumentChunk
	err := s.db.WithContext(ctx).Where("document_id = ?", documentID).Order("created_at ASC").First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil // 文档不存在，返回零值时间
		}
		return time.Time{}, err
	}
	return doc.CreatedAt, nil
}

// GetDocumentChunkCount 获取文档的文档块数量
func (s *PGVectorStore) GetDocumentChunkCount(ctx context.Context, documentID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&DocumentChunk{}).Where("document_id = ?", documentID).Count(&count).Error
	return count, err
}

// Clear 清空所有向量数据
func (s *PGVectorStore) Clear(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&DocumentChunk{}).Error
}

// Close 关闭数据库连接
func (s *PGVectorStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
