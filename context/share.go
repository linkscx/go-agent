package context

import (
	"log"

	"github.com/tiktoken-go/tokenizer"

	"go-agent/shared"
)

var tokenEnc tokenizer.Codec

func init() {
	var err error
	tokenEnc, err = tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		log.Fatal(err)
	}
}

func CountTokens(message shared.OpenAIMessage) int {
	contentAny := message.GetContent().AsAny()
	switch contentAny := contentAny.(type) {
	case *string:
		count, _ := tokenEnc.Count(*contentAny)
		return count
	}
	return 0
}
