package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"go-agent/agent"
	ctxengine "go-agent/context"
	"go-agent/db"
	"go-agent/memory"
	"go-agent/rag"
	"go-agent/shared"
	"go-agent/storage"
	"go-agent/tool"
	"go-agent/tui"
)

func main() {
	ctx := context.Background()
	appConf, err := shared.LoadAppConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}

	mcpServerMap, err := shared.LoadMcpServerConfig("mcp-server.json")
	if err != nil {
		log.Printf("Failed to load MCP server configuration: %v", err)
	}
	mcpClients := make([]*agent.McpClient, 0)
	for k, v := range mcpServerMap {
		mcpClient := agent.NewMcpToolProvider(k, v)
		if err := mcpClient.RefreshTools(ctx); err != nil {
			log.Printf("Failed to refresh tools for MCP server %s: %v", k, err)
			continue
		}
		mcpClients = append(mcpClients, mcpClient)
	}

	memoryStorage := storage.NewMemoryStorage()
	summarizer := ctxengine.NewLLMSummarizer(appConf.LLMProviders.BackModel, 200)

	policies := []ctxengine.Policy{
		ctxengine.NewOffloadPolicy(memoryStorage, 0.4, 0, 100),
		ctxengine.NewSummaryPolicy(summarizer, 10, 20, 0.6),
		ctxengine.NewTruncatePolicy(0, 0.85),
	}

	homeStorage := storage.NewFileSystemStorage(filepath.Join(shared.GetHomeDir(), ".go-agent"))
	workspaceStorage := storage.NewFileSystemStorage(filepath.Join(shared.GetWorkspaceDir(), ".go-agent"))
	memoryUpdater := memory.NewLLMMemoryUpdater(appConf.LLMProviders.BackModel)
	multiLevelMemory := memory.NewMultiLevelMemory(homeStorage, workspaceStorage, memoryUpdater)

	contextEngine := ctxengine.NewContextEngine(multiLevelMemory, policies)

	confirmConfig := agent.ToolConfirmConfig{
		RequireConfirmTools: map[tool.AgentTool]bool{
			tool.AgentToolBash: true,
		},
	}

	var vectorStore *db.PGVectorStore

	vectorStore, err = db.NewPGVectorStore(db.Config{
		Host:      appConf.PGVector.Host,
		Port:      appConf.PGVector.Port,
		User:      appConf.PGVector.User,
		Password:  appConf.PGVector.Password,
		Database:  appConf.PGVector.Database,
		Dimension: appConf.PGVector.Dimension,
	})
	if err != nil {
		log.Printf("Failed to initialize vector store: %v", err)
		vectorStore = nil
	} else {
		defer vectorStore.Close()
	}

	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.CreateBashTool(shared.GetWorkspaceDir()),
		tool.NewLoadStorageTool(memoryStorage),
		tool.NewLoadSkillTool(),
	}

	if vectorStore != nil {
		embedService := rag.NewHTTPEmbeddingService(rag.HTTPEmbeddingConfig{
			APIKey:     appConf.Embedding.APIKey,
			BaseURL:    appConf.Embedding.BaseURL,
			Model:      appConf.Embedding.Model,
			Dimensions: appConf.Embedding.Dimensions,
		})

		rerankService := rag.NewHTTPRerankService(rag.HTTPRerankConfig{
			APIKey:  appConf.Rerank.APIKey,
			BaseURL: appConf.Rerank.BaseURL,
			Model:   appConf.Rerank.Model,
		})

		searcher := rag.NewSemanticSearcher(embedService, vectorStore, rerankService)
		tools = append(tools, tool.NewSemanticSearchTool(searcher))
	}

	agentInstance := agent.NewAgent(
		appConf.LLMProviders.FrontModel,
		agent.CodingAgentSystemPrompt,
		confirmConfig,
		tools,
		mcpClients,
		contextEngine,
	)

	log.SetOutput(io.Discard)
	p := tea.NewProgram(tui.NewModel(agentInstance, appConf.LLMProviders.FrontModel.Model))
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}
