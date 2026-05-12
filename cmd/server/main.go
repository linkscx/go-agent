package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/joho/godotenv"

	"go-agent/agent"
	ctxengine "go-agent/context"
	"go-agent/memory"
	"go-agent/server"
	"go-agent/shared"
	"go-agent/storage"
	"go-agent/tool"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	appConf, err := shared.LoadAppConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}

	db, err := server.NewDB("./agent.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

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

	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.CreateBashTool(shared.GetWorkspaceDir()),
		tool.NewLoadStorageTool(memoryStorage),
		tool.NewLoadSkillTool(),
	}

	agentInstance := agent.NewAgent(
		appConf.LLMProviders.FrontModel,
		agent.CodingAgentSystemPrompt,
		confirmConfig,
		tools,
		mcpClients,
		contextEngine,
	)

	service := server.NewService(agentInstance, db)
	controller := server.NewController(service)
	router := server.NewRouter(controller)

	log.Println("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
