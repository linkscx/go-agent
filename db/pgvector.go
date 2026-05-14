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

type PGVectorStore struct {
	db        *gorm.DB
	dimension int
}

type DocumentChunk struct {
	ID         uint      `gorm:"primaryKey"`
	Content    string    `gorm:"type:text;not null"`
	DocumentID string    `gorm:"type:text;not null;index"`
	StartPos   int       `gorm:"not null"`
	EndPos     int       `gorm:"not null"`
	Embedding  string    `gorm:"type:text;not null"`
	CreatedAt  time.Time
}

func (DocumentChunk) TableName() string {
	return "document_chunks"
}

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

type Config struct {
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	Dimension int
}

func NewPGVectorStore(config Config) (*PGVectorStore, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := createTableRaw(db, config.Dimension); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &PGVectorStore{
		db:        db,
		dimension: config.Dimension,
	}, nil
}

func createTableRaw(db *gorm.DB, dim int) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS document_chunks (
			id SERIAL PRIMARY KEY,
			content TEXT NOT NULL,
			document_id TEXT NOT NULL,
			start_pos INTEGER NOT NULL,
			end_pos INTEGER NOT NULL,
			embedding vector(%d),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, dim)
	if err := db.Exec(sql).Error; err != nil {
		return err
	}

	db.Exec(`CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks (document_id)`)

	if dim <= 2000 {
		indexSQL := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_document_chunks_embedding 
			ON document_chunks USING ivfflat (embedding vector_cosine_ops) 
			WITH (lists = 100)
		`)
		db.Exec(indexSQL)
	}

	return nil
}

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

func pgVectorToVector(s string) []float32 {
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

func (s *PGVectorStore) InsertBatch(ctx context.Context, vps []shared.VectorPoint) error {
	docs := make([]*DocumentChunk, len(vps))
	for i, vp := range vps {
		if len(vp.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch at index %d: expected %d, got %d", i, s.dimension, len(vp.Vector))
		}

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

func (s *PGVectorStore) Search(ctx context.Context, queryVector shared.Vector, limit int) ([]shared.VectorPointResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	vectorStr := vectorToPGVector(queryVector)

	type searchResult struct {
		DocumentChunk
		Distance float64 `gorm:"column:distance"`
	}

	var results []searchResult
	err := s.db.WithContext(ctx).Raw(`
		SELECT *, embedding <=> ? AS distance
		FROM document_chunks
		ORDER BY embedding <=> ?
		LIMIT ?
	`, vectorStr, vectorStr, limit).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	vectorResults := make([]shared.VectorPointResult, len(results))
	for i, r := range results {
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

func (s *PGVectorStore) SearchWithScore(ctx context.Context, queryVector shared.Vector, limit int) ([]shared.VectorPointResult, error) {
	return s.Search(ctx, queryVector, limit)
}

func (s *PGVectorStore) DeleteByDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Where("document_id = ?", documentID).Delete(&DocumentChunk{}).Error
}

func (s *PGVectorStore) GetDocumentIndexedTime(ctx context.Context, documentID string) (time.Time, error) {
	var doc DocumentChunk
	err := s.db.WithContext(ctx).Where("document_id = ?", documentID).Order("created_at ASC").First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return doc.CreatedAt, nil
}

func (s *PGVectorStore) GetDocumentChunkCount(ctx context.Context, documentID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&DocumentChunk{}).Where("document_id = ?", documentID).Count(&count).Error
	return count, err
}

func (s *PGVectorStore) Clear(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&DocumentChunk{}).Error
}

func (s *PGVectorStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}