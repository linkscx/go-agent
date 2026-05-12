package index

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	shared "go-agent/rag"

	"golang.org/x/sync/errgroup"
)

// Indexer 索引器，负责将代码仓库索引到向量数据库
// 提供文件遍历、文本切分、向量嵌入和存储的完整流程
type Indexer struct {
	rootPath     string                  // 代码仓库根路径
	fileWalker   *FileWalker             // 文件遍历器
	chunker      shared.ChunkerService   // 文本切分器
	vectorStore  shared.VectorStore      // 向量数据库存储
	embedService shared.EmbeddingService // 向量嵌入服务
}

// IndexerConfig 索引器配置
type IndexerConfig struct {
	RootPath    string             // 代码仓库根路径
	ChunkerType shared.ChunkerType // 切分器类型（按行/按段落）
	MaxLines    int                // 每块最大行数
	MaxChars    int                // 每块最大字符数
}

// NewIndexer 创建新的索引器实例
//
// 参数:
//   - config: 索引器配置，包含根路径和切分策略
//   - vectorStore: 向量数据库接口，用于存储向量
//   - embedService: 嵌入服务接口，用于生成向量
//
// 返回值:
//   - *Indexer: 配置好的索引器实例
func NewIndexer(config IndexerConfig, vectorStore shared.VectorStore, embedService shared.EmbeddingService) *Indexer {
	return &Indexer{
		rootPath:     config.RootPath,
		fileWalker:   NewFileWalker(),
		chunker:      shared.NewChunker(config.ChunkerType, config.MaxLines, config.MaxChars),
		vectorStore:  vectorStore,
		embedService: embedService,
	}
}

// Index 串行执行索引操作
// 逐个处理文件，适用于文件数量较少或需要顺序处理的场景
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回值:
//   - *IndexResult: 索引结果统计
//   - error: 处理过程中的错误
func (idx *Indexer) Index(ctx context.Context) (*IndexResult, error) {
	startTime := time.Now()

	// 1. 遍历文件：获取所有需要索引的文件路径
	files, err := idx.fileWalker.Walk(idx.rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to walk files: %w", err)
	}

	result := &IndexResult{
		TotalFiles: len(files),
	}

	// 2. 串行处理每个文件
	for _, filePath := range files {
		fileResult, err := idx.indexFile(ctx, filePath)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
			continue
		}

		result.TotalChunks += fileResult.Chunks

		// 根据操作类型更新统计
		switch fileResult.Action {
		case IndexActionSkip:
			result.SkippedFiles++
		case IndexActionReindex:
			result.ReindexedFiles++
			result.SuccessFiles++
		default:
			result.SuccessFiles++
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// IndexConcurrent 并发执行索引操作
// 使用多个 goroutine 并行处理文件，适用于大量文件的场景
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - concurrency: 并发 worker 数量
//
// 返回值:
//   - *IndexResult: 索引结果统计
//   - error: 处理过程中的错误
//
// 实现说明:
//   - 使用 channel 作为任务队列
//   - 使用 WaitGroup 等待所有 worker 完成
//   - 使用 Mutex 保护结果统计的并发写入
func (idx *Indexer) IndexConcurrent(ctx context.Context, concurrency int) (*IndexResult, error) {
	startTime := time.Now()

	// 1. 遍历文件
	files, err := idx.fileWalker.Walk(idx.rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to walk files: %w", err)
	}

	result := &IndexResult{
		TotalFiles: len(files),
	}

	// 2. 创建任务队列（channel）
	// 使用缓冲 channel，容量为文件数量，避免阻塞
	fileChan := make(chan string, len(files))
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan) // 关闭 channel，表示没有更多任务

	// 3. 使用 WaitGroup 等待所有 goroutine 完成
	var wg sync.WaitGroup
	// 使用 Mutex 保护 result 的并发写入
	var mu sync.Mutex

	// 4. 启动多个 worker goroutine
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 从 channel 获取任务，直到 channel 关闭
			for filePath := range fileChan {
				fileResult, err := idx.indexFile(ctx, filePath)

				// 加锁保护共享的 result 变量
				mu.Lock()
				if err != nil {
					result.FailedFiles++
					result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
				} else {
					result.TotalChunks += fileResult.Chunks
					switch fileResult.Action {
					case IndexActionSkip:
						result.SkippedFiles++
					case IndexActionReindex:
						result.ReindexedFiles++
						result.SuccessFiles++
					default:
						result.SuccessFiles++
					}
				}
				mu.Unlock()
			}
		}()
	}

	// 5. 等待所有 worker 完成
	wg.Wait()
	result.Duration = time.Since(startTime)
	return result, nil
}

// indexFile 索引单个文件（带去重检测）
// 流程：检查文件是否已索引 → 读取文件 → 切分 → 嵌入 → 存储
//
// 参数:
//   - ctx: 上下文
//   - filePath: 文件绝对路径
//
// 返回值:
//   - *FileIndexResult: 单文件处理结果
//   - error: 处理错误
//
// 去重逻辑:
//   - 检查文件是否已索引（通过文件路径）
//   - 比较文件修改时间和索引时间
//   - 文件未修改则跳过
//   - 文件已修改则删除旧索引，重新索引
func (idx *Indexer) indexFile(ctx context.Context, filePath string) (*FileIndexResult, error) {
	// 转换为相对路径（用于存储和查询）
	relPath, err := filepath.Rel(idx.rootPath, filePath)
	if err != nil {
		relPath = filePath
	}

	// 获取文件信息（用于检查修改时间）
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// 检查文件是否已索引，获取上次索引时间
	indexedTime, err := idx.vectorStore.GetDocumentIndexedTime(ctx, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check file index status: %w", err)
	}

	// 如果文件已索引，检查是否需要更新
	if !indexedTime.IsZero() {
		// 将文件修改时间转换为 UTC 并截断到秒级精度
		// 避免毫秒级差异导致的不必要重新索引
		fileModTime := fileInfo.ModTime().UTC().Truncate(time.Second)
		// 数据库时间已经是 UTC，同样截断到秒级精度
		indexedTimeTruncated := indexedTime.UTC().Truncate(time.Second)

		// 调试日志：记录时间比较详情
		log.Printf("[DEBUG] %s: fileModTime=%v, indexedTime=%v, skip=%v",
			relPath, fileModTime, indexedTimeTruncated,
			!fileModTime.After(indexedTimeTruncated))

		// 文件未修改（修改时间 <= 索引时间），跳过
		if !fileModTime.After(indexedTimeTruncated) {
			return &FileIndexResult{
				FilePath: filePath,
				Chunks:   0,
				Action:   IndexActionSkip,
			}, nil
		}

		// 文件已修改，删除旧索引记录
		if err := idx.vectorStore.DeleteByDocument(ctx, relPath); err != nil {
			return nil, fmt.Errorf("failed to delete old index: %w", err)
		}
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 切分文本为多个块
	chunks := idx.chunker.Chunk(relPath, string(content))
	if len(chunks) == 0 {
		return &FileIndexResult{Chunks: 0, Action: IndexActionSkip}, nil
	}

	// 获取向量嵌入（并发处理）
	vectorPoints, err := idx.embedChunks(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to embed chunks: %w", err)
	}

	// 设置文件修改时间作为 CreatedAt（转换为 UTC 存储）
	// 用于后续的去重检测
	fileModTime := fileInfo.ModTime().UTC()
	for i := range vectorPoints {
		vectorPoints[i].CreatedAt = fileModTime
	}

	// 批量插入向量到数据库
	if err := idx.vectorStore.InsertBatch(ctx, vectorPoints); err != nil {
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}

	// 确定操作类型：新索引或重新索引
	action := IndexActionNew
	if !indexedTime.IsZero() {
		action = IndexActionReindex
	}

	return &FileIndexResult{
		FilePath: filePath,
		Chunks:   len(chunks),
		Action:   action,
	}, nil
}

// embedChunks 批量获取向量嵌入（并发处理）
// 为每个文本块生成向量表示，使用 errgroup 控制并发
//
// 参数:
//   - ctx: 上下文
//   - chunks: 文本块列表
//
// 返回值:
//   - []shared.VectorPoint: 向量点列表（包含向量和元数据）
//   - error: 处理错误（返回第一个错误）
//
// 并发控制:
//   - 使用 errgroup 管理 goroutine 生命周期
//   - 限制最大并发数为 10，避免资源耗尽
//   - 任一任务出错时取消其他任务
func (idx *Indexer) embedChunks(ctx context.Context, chunks []shared.Chunk) ([]shared.VectorPoint, error) {
	// 预分配结果切片
	vectorPoints := make([]shared.VectorPoint, len(chunks))

	// 创建 errgroup，限制并发数为 10
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	// 并发处理每个 chunk
	for i, chunk := range chunks {
		i, chunk := i, chunk // 避免闭包陷阱
		g.Go(func() error {
			// 调用嵌入服务获取向量
			vector, err := idx.embedService.Embed(ctx, chunk.Content)
			if err != nil {
				return err // 返回错误，errgroup 会取消其他任务
			}

			// 保存结果
			vectorPoints[i] = shared.VectorPoint{
				Vector: vector,
				Chunk:  chunk,
			}
			return nil
		})
	}

	// 等待所有任务完成，返回第一个错误
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return vectorPoints, nil
}

// Search 在索引中搜索相似内容
// 将查询文本转换为向量，在向量数据库中搜索最相似的块
//
// 参数:
//   - ctx: 上下文
//   - query: 查询文本
//   - limit: 返回结果数量限制
//
// 返回值:
//   - []shared.VectorPointResult: 搜索结果列表
//   - error: 搜索错误
func (idx *Indexer) Search(ctx context.Context, query string, limit int) ([]shared.VectorPointResult, error) {
	// 1. 将查询文本转换为向量
	queryVector, err := idx.embedService.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 2. 在向量数据库中搜索相似向量
	return idx.vectorStore.Search(ctx, queryVector, limit)
}

// Clear 清空索引
// 删除向量数据库中的所有数据
//
// 参数:
//   - ctx: 上下文
//
// 返回值:
//   - error: 操作错误
func (idx *Indexer) Clear(ctx context.Context) error {
	return idx.vectorStore.Clear(ctx)
}

// Close 关闭索引器
// 释放资源，关闭数据库连接等
//
// 返回值:
//   - error: 关闭错误
func (idx *Indexer) Close() error {
	return idx.vectorStore.Close()
}

// IndexResult 索引结果统计
type IndexResult struct {
	TotalFiles     int           // 总文件数
	SuccessFiles   int           // 成功索引的文件数
	FailedFiles    int           // 失败的文件数
	SkippedFiles   int           // 跳过的文件数（已索引且未修改）
	ReindexedFiles int           // 重新索引的文件数（已索引但已修改）
	TotalChunks    int           // 总块数
	Duration       time.Duration // 处理耗时
	Errors         []error       // 错误列表
}

// FileIndexResult 单文件索引结果
type FileIndexResult struct {
	FilePath string      // 文件路径
	Chunks   int         // 生成的块数
	Action   IndexAction // 执行的操作类型
}

// IndexAction 索引操作类型
type IndexAction int

const (
	IndexActionNew     IndexAction = iota // 新索引（首次索引）
	IndexActionReindex                    // 重新索引（文件已修改）
	IndexActionSkip                       // 跳过（文件未修改）
)
