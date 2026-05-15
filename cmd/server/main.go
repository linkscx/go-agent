package main

import (
	"context"
	"log"
	"path/filepath"

	"go-agent/agent"
	ctxengine "go-agent/context"
	"go-agent/db"
	"go-agent/index"
	"go-agent/memory"
	"go-agent/rag"
	"go-agent/server"
	"go-agent/shared"
	"go-agent/storage"
	"go-agent/tool"
)

func main() {
	ctx := context.Background()
	appConf, err := shared.LoadAppConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}

	pgsqlDB, err := server.NewDB(server.DBConfig{
		Host:     appConf.PGVector.Host,
		Port:     appConf.PGVector.Port,
		User:     appConf.PGVector.User,
		Password: appConf.PGVector.Password,
		Database: appConf.PGVector.Database,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer pgsqlDB.Close()

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

	dbStorage := server.NewDBStorage(pgsqlDB)
	summarizer := ctxengine.NewLLMSummarizer(appConf.LLMProviders.BackModel, 200)

	policies := []ctxengine.Policy{
		ctxengine.NewOffloadPolicy(dbStorage, 0.4, 0, 100),
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

	vectorStore, err := db.NewPGVectorStore(db.Config{
		Host:      appConf.PGVector.Host,
		Port:      appConf.PGVector.Port,
		User:      appConf.PGVector.User,
		Password:  appConf.PGVector.Password,
		Database:  appConf.PGVector.Database,
		Dimension: appConf.PGVector.Dimension,
	})
	if err != nil {
		log.Printf("Failed to initialize vector store: %v", err)
	} else {
		defer vectorStore.Close()

		embedService := rag.NewHTTPEmbeddingService(rag.HTTPEmbeddingConfig{
			APIKey:     appConf.Embedding.APIKey,
			BaseURL:    appConf.Embedding.BaseURL,
			Model:      appConf.Embedding.Model,
			Dimensions: appConf.Embedding.Dimensions,
		})

		idx := index.NewIndexer(index.IndexerConfig{
			RootPath:    appConf.Indexer.RootPath,
			ChunkerType: rag.ChunkerType(appConf.Indexer.ChunkerType),
			MaxLines:    appConf.Indexer.MaxLines,
			MaxChars:    appConf.Indexer.MaxChars,
		}, vectorStore, embedService)

		result, err := idx.Index(ctx)
		if err != nil {
			log.Printf("Failed to index workspace: %v", err)
		} else {
			log.Printf("Indexing completed: %d files, %d chunks, %d skipped, duration=%v",
				result.TotalFiles, result.TotalChunks, result.SkippedFiles, result.Duration)
		}
	}

	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.CreateBashTool(shared.GetWorkspaceDir()),
		tool.NewLoadStorageTool(dbStorage),
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

	service := server.NewService(agentInstance, pgsqlDB)
	controller := server.NewController(service)
	router := server.NewRouter(controller)

	log.Println("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
