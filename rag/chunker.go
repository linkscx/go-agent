package rag

import (
	"bufio"
	"strings"
)

// ChunkerType 切分器类型
type ChunkerType string

const (
	ChunkerTypeLine      ChunkerType = "line"      // 按行切分
	ChunkerTypeParagraph ChunkerType = "paragraph" // 按段落切分
)

// NewChunker 创建 Chunker 的工厂函数
// 默认返回按行切分的实现
func NewChunker(chunkerType ChunkerType, maxLines, maxChars int) ChunkerService {
	switch chunkerType {
	case ChunkerTypeParagraph:
		return NewParagraphChunker(maxLines)
	case ChunkerTypeLine, "":
		return NewLineChunker(maxLines, maxChars)
	default:
		return NewLineChunker(maxLines, maxChars)
	}
}

// LineChunker 按行和字符数切分的实现
type LineChunker struct {
	maxLines int
	maxChars int
}

// NewLineChunker 创建按行切分的 Chunker
func NewLineChunker(maxLines, maxChars int) *LineChunker {
	if maxLines <= 0 {
		maxLines = 100
	}
	if maxChars <= 0 {
		maxChars = 2000
	}
	return &LineChunker{
		maxLines: maxLines,
		maxChars: maxChars,
	}
}

// Chunk 实现 ChunkerService 接口
func (c *LineChunker) Chunk(documentID, content string) []Chunk {
	var chunks []Chunk
	lines := strings.Split(content, "\n")

	currentChunk := []string{}
	startLine := 1
	currentSize := 0

	for i, line := range lines {
		lineNum := i + 1
		lineSize := len(line)

		// 检查是否需要开始新的块
		if len(currentChunk) > 0 &&
			(len(currentChunk) >= c.maxLines || currentSize+lineSize > c.maxChars) {
			chunks = append(chunks, Chunk{
				Content: strings.Join(currentChunk, "\n"),
				Meta: Meta{
					DocumentID: documentID,
					StartPos:   startLine,
					EndPos:     lineNum - 1,
				},
			})

			currentChunk = []string{}
			startLine = lineNum
			currentSize = 0
		}

		currentChunk = append(currentChunk, line)
		currentSize += lineSize
	}

	// 保存最后一个块
	if len(currentChunk) > 0 {
		chunks = append(chunks, Chunk{
			Content: strings.Join(currentChunk, "\n"),
			Meta: Meta{
				DocumentID: documentID,
				StartPos:   startLine,
				EndPos:     len(lines),
			},
		})
	}

	return chunks
}

// ParagraphChunker 按段落切分的实现
type ParagraphChunker struct {
	maxParagraphs int
}

// NewParagraphChunker 创建按段落切分的 Chunker
func NewParagraphChunker(maxParagraphs int) *ParagraphChunker {
	if maxParagraphs <= 0 {
		maxParagraphs = 5
	}
	return &ParagraphChunker{
		maxParagraphs: maxParagraphs,
	}
}

// Chunk 实现 ChunkerService 接口
func (c *ParagraphChunker) Chunk(documentID, content string) []Chunk {
	var chunks []Chunk
	scanner := bufio.NewScanner(strings.NewReader(content))

	currentParagraphs := []string{}
	startLine := 1
	currentLine := 1

	for scanner.Scan() {
		line := scanner.Text()

		// 空行表示段落结束
		if strings.TrimSpace(line) == "" {
			if len(currentParagraphs) > 0 {
				chunks = append(chunks, Chunk{
					Content: strings.Join(currentParagraphs, "\n"),
					Meta: Meta{
						DocumentID: documentID,
						StartPos:   startLine,
						EndPos:     currentLine - 1,
					},
				})
				currentParagraphs = []string{}
			}
			currentLine++
			startLine = currentLine
			continue
		}

		currentParagraphs = append(currentParagraphs, line)
		currentLine++

		// 检查是否需要开始新的块
		if len(currentParagraphs) >= c.maxParagraphs {
			chunks = append(chunks, Chunk{
				Content: strings.Join(currentParagraphs, "\n"),
				Meta: Meta{
					DocumentID: documentID,
					StartPos:   startLine,
					EndPos:     currentLine - 1,
				},
			})
			currentParagraphs = []string{}
		}
	}

	// 处理最后一个段落
	if len(currentParagraphs) > 0 {
		chunks = append(chunks, Chunk{
			Content: strings.Join(currentParagraphs, "\n"),
			Meta: Meta{
				DocumentID: documentID,
				StartPos:   startLine,
				EndPos:     currentLine - 1,
			},
		})
	}

	return chunks
}
