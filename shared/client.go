package shared

import (
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func NewLLMClient(modelConf ModelConfig) openai.Client {
	client := openai.NewClient(
		option.WithBaseURL(modelConf.BaseURL),
		option.WithAPIKey(modelConf.ApiKey),
		option.WithHeader("X-Title", "go-agent"),
		option.WithHeader("HTTP-Referer", "https://github.com/baby-llm/baby-agent"),
	)
	return client
}
