# Go Agent

一个功能完整的 AI Agent 系统，使用 Go 语言开发，支持多轮对话、工具调用、知识检索增强、多端交互以及隔离沙盒运行环境。

## 项目简介

核心特性包括：
- Agent 框架与上下文管理（截断、卸载、摘要策略）
- Skills 与 MCP 扩展（本地工具 + MCP 协议）
- RAG 检索与向量存储（pgvector + 语义搜索）
- 多端交互（TUI 终端界面 + Web 服务）
- 沙盒安全（Docker 隔离 + 工具确认机制）

## 技术栈

- **语言**: Go 1.25+
- **LLM SDK**: OpenAI Go SDK v3
- **协议**: Model Context Protocol (MCP)
- **数据库**: PostgreSQL (pgvector), SQLite
- **TUI**: Bubble Tea
- **Web**: Gin + React + Tailwind CSS

## 快速开始

### 1. 配置文件

- `.env` - 环境变量配置
- `config.json` - LLM 模型配置
- `mcp-server.json` - MCP 服务器配置

### 2. 安装依赖

```bash
go mod download
```

### 3. 运行

#### TUI 模式（终端界面）

```bash
go run ./cmd/tui
```

#### Web 服务模式

```bash
# 启动后端服务（监听 :8080）
go run ./cmd/server
```

然后访问 http://localhost:8080 使用 Web 界面。

#### 前端开发（可选）

如果需要开发前端：

```bash
cd frontend
npm install
npm run dev
```

## 配置说明

### config.json

```json
{
  "llm_providers": {
    "front_model": {
      "base_url": "http://localhost:4141/v1",
      "model": "gpt-5-mini",
      "api_key": "dummy",
      "context_window": 1000
    },
    "back_model": {
      "base_url": "http://localhost:4141/v1",
      "model": "gpt-5-mini",
      "api_key": "dummy",
      "context_window": 128000
    }
  }
}
```

- `front_model`: 主对话模型（用于与用户交互）
- `back_model`: 后台模型（用于摘要、记忆更新等）

如需使用真实的 OpenAI API，修改 `base_url` 为 `https://api.openai.com/v1`，并填入真实的 `api_key`。

### .env

```bash
OPENAI_BASE_URL=http://localhost:4141/v1
OPENAI_API_KEY=dummy
OPENAI_MODEL=gpt-5-mini
```

### mcp-server.json

配置 MCP 服务器以扩展 Agent 能力：

```json
{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "${workspaceFolder}"]
  }
}
```

## 项目结构

```
go-agent/
├── agent/              # Agent 核心逻辑
├── context/            # 上下文管理
├── memory/             # 记忆系统
├── storage/            # 持久化存储
├── tool/               # 工具集
├── skill/              # 技能系统
├── rag/                # RAG 模块
├── db/                 # 数据库集成
├── server/             # Web 服务
├── tui/                # 终端界面
├── frontend/           # Web 前端
├── cmd/                # 入口程序
│   ├── tui/           # TUI 入口
│   └── server/        # Web 服务入口
└── shared/            # 共享工具
```

## 核心功能

### 1. 上下文管理
- **截断策略**: 删除旧消息，保留最近对话
- **卸载策略**: 将长消息存储到外部，保留恢复提示
- **摘要策略**: 使用 LLM 压缩历史对话

### 2. 记忆系统
- **Global Memory**: `~/.go-agent` (跨会话)
- **Workspace Memory**: `.go-agent/` (项目级)
- LLM 驱动的自动记忆提取和更新

### 3. 技能系统
- 技能是描述性提示，指导 LLM 如何使用工具
- 元数据在启动时注入系统提示
- 完整内容按需加载（通过 `load_skill` 工具）
- 技能文件存储在 `.go-agent/skills/` 目录

### 4. RAG 检索
- 使用 Embedding 模型将代码向量化
- 通过 pgvector 进行相似度检索
- 支持重排序提高检索精度
- 语义搜索工具集成

### 5. 沙盒安全
- Docker 容器隔离执行危险命令
- 人工确认机制（Allow/Reject/Always Allow）
- ESC 取消机制，随时终止 Agent 执行

## API 接口

### 创建会话
```bash
POST /api/conversations
Content-Type: application/json

{
  "title": "New Conversation"
}
```

### 发送消息（SSE 流式）
```bash
POST /api/conversations/:conversation_id/messages
Content-Type: application/json

{
  "parent_message_id": "",
  "content": "你好，请帮我分析这段代码"
}
```

### 获取会话列表
```bash
GET /api/conversations
```

## 许可证

Apache License 2.0

## 致谢

本项目基于 [baby-agent](https://github.com/baby-llm/baby-agent) 教学项目整合而成。
