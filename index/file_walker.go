package index

import (
	"os"
	"path/filepath"
	"strings"
)

// FileWalker 负责遍历目录查找文本文件
type FileWalker struct {
	excludeDirs []string
	extensions  []string
}

// NewFileWalker 创建一个新的 FileWalker
func NewFileWalker() *FileWalker {
	return &FileWalker{
		excludeDirs: []string{
			".git", ".idea", ".vscode", "node_modules",
			"vendor", "dist", "build", "target", "bin",
			".venv", "venv", "__pycache__", ".cache",
		},
		extensions: []string{
			".go", ".js", ".ts", ".tsx", ".jsx",
			".py", ".java", ".c", ".cpp", ".h", ".hpp",
			".rs", ".rb", ".php", ".cs", ".swift",
			".kt", ".scala", ".sh", ".bash", ".zsh",
			".md", ".txt", ".json", ".yaml", ".yml",
			".toml", ".xml", ".html", ".css", ".scss",
			".sql", ".graphql", ".proto",
		},
	}
}

// WithExcludeDirs 添加需要排除的目录
func (fw *FileWalker) WithExcludeDirs(dirs []string) *FileWalker {
	fw.excludeDirs = append(fw.excludeDirs, dirs...)
	return fw
}

// WithExtensions 添加支持的文件扩展名
func (fw *FileWalker) WithExtensions(exts []string) *FileWalker {
	fw.extensions = append(fw.extensions, exts...)
	return fw
}

// Walk 遍历指定路径，返回所有文本文件的路径
func (fw *FileWalker) Walk(rootPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			// 检查是否在排除列表中
			baseName := filepath.Base(path)
			for _, excludeDir := range fw.excludeDirs {
				if baseName == excludeDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 检查文件扩展名
		ext := strings.ToLower(filepath.Ext(path))
		valid := false
		for _, supportedExt := range fw.extensions {
			if ext == supportedExt {
				valid = true
				break
			}
		}

		if valid {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// IsTextFile 检查文件是否是文本文件（通过扩展名）
func (fw *FileWalker) IsTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, supportedExt := range fw.extensions {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// ShouldExcludeDir 检查目录是否应该被排除
func (fw *FileWalker) ShouldExcludeDir(dirName string) bool {
	for _, excludeDir := range fw.excludeDirs {
		if dirName == excludeDir {
			return true
		}
	}
	return false
}
