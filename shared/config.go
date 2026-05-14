package shared

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	LLMProviders struct {
		FrontModel ModelConfig `json:"front_model"`
		BackModel  ModelConfig `json:"back_model"`
	} `json:"llm_providers"`
	Indexer   IndexerConfig   `json:"indexer"`
	PGVector  PGVectorConfig  `json:"pgvector"`
	Embedding EmbeddingConfig `json:"embedding"`
	Rerank    RerankConfig    `json:"rerank"`
}

type ModelConfig struct {
	BaseURL       string `json:"base_url"`
	ApiKey        string `json:"api_key"`
	Model         string `json:"model"`
	ContextWindow int    `json:"context_window"`
}

type IndexerConfig struct {
	RootPath    string `json:"root_path"`
	ChunkerType string `json:"chunker_type"`
	MaxLines    int    `json:"max_lines"`
	MaxChars    int    `json:"max_chars"`
}

type PGVectorConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Database  string `json:"database"`
	Dimension int    `json:"dimension"`
}

type EmbeddingConfig struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

type RerankConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

func NewModelConfig() ModelConfig {
	return ModelConfig{
		BaseURL:       getEnvDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		ApiKey:        getEnvDefault("OPENAI_API_KEY", ""),
		Model:         getEnvDefault("OPENAI_MODEL", "gpt-5.2"),
		ContextWindow: 200000,
	}
}

func getEnvDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func LoadAppConfig(path string) (AppConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return AppConfig{}, err
	}
	var config AppConfig
	err = json.Unmarshal(content, &config)
	if err != nil {
		return AppConfig{}, err
	}
	return config, nil
}
