# Go Agent

一个功能完整的 AI Agent 系统，使用 Go 语言开发，支持多轮对话、工具调用、知识检索增强、多端交互以及隔离沙盒运行环境。

## 项目简介

Go Agent 是一个基于 Model Context Protocol (MCP) 的 AI 智能体框架，集成了 LLM 对话、工具调用、记忆管理、RAG 检索和沙盒执行等核心能力。

核心特性包括：
- **Agent 框架与上下文管理**：智能的上下文截断、卸载和摘要策略，支持长对话管理
- **Skills 与 MCP 扩展**：通过 MCP 协议集成本地工具和远程 MCP 服务器
- **RAG 检索与向量存储**：基于 pgvector 的语义搜索，支持代码知识库检索
- **多端交互**：支持 TUI 终端界面和 Web 前端界面
- **沙盒安全**：Docker 容器隔离执行危险命令，支持人工确认机制
- **记忆系统**：支持 Global Memory（全局）和 Workspace Memory（项目级）两级记忆

## 技术栈

- **语言**: Go 1.25+
- **LLM SDK**: OpenAI Go SDK v3
- **协议**: Model Context Protocol (MCP)
- **数据库**: PostgreSQL (pgvector), SQLite
- **TUI**: Bubble Tea v2
- **Web**: Gin Web Framework + React 19 + Tailwind CSS v4
- **前端构建**: Vite + TypeScript
- **工具**: Docker（沙盒隔离）

## 快速开始

### 1. 环境准备

#### 安装 PostgreSQL（带 pgvector 扩展）

使用 Docker 快速创建 PostgreSQL 容器：

```bash
docker run -d \
  --name go-agent-pgvector \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=vector_db \
  -p 5432:5432 \
  pgvector/pgvector:pg16
```

容器启动后，需要启用 pgvector 扩展：

```bash
docker exec -it go-agent-pgvector psql -U postgres -d vector_db -c "CREATE EXTENSION IF NOT EXISTS vector;"
```

验证扩展是否安装成功：

```bash
docker exec -it go-agent-pgvector psql -U postgres -d vector_db -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
```

#### 停止和删除容器

```bash
docker stop go-agent-pgvector
docker rm go-agent-pgvector
```

#### 数据持久化（可选）

如果需要持久化数据，可以挂载数据卷：

```bash
docker run -d \
  --name go-agent-pgvector \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=vector_db \
  -p 5432:5432 \
  -v pgvector_data:/var/lib/postgresql/data \
  pgvector/pgvector:pg16
```

### 2. 配置文件

复制示例配置文件：

```bash
cp config.example.json config.json
cp mcp-server.example.json mcp-server.json
```

创建环境变量文件：

```bash
cat > .env << EOF
OPENAI_API_KEY=your-openai-api-key
EMBEDDING_API_KEY=your-embedding-api-key
RERANK_API_KEY=your-rerank-api-key
POSTGRES_PASSWORD=123456
EOF
```

### 3. 安装依赖

```bash
go mod download
```

### 4. 运行

#### TUI 模式（终端界面）

```bash
go run ./cmd/tui
```

#### Web 服务模式

```bash
go run ./cmd/server
```

然后访问 http://localhost:8080 使用 Web 界面。

#### 前端开发（可选）

如果需要开发前端：

```bash
cd frontend

pnpm install

pnpm run dev
```

前端默认运行在 http://localhost:5173，需要通过后端代理访问（后端已配置代理）。

## 配置说明

### config.json

完整的配置文件结构：

```json
{
  "llm_providers": {
    "front_model": {
      "base_url": "https://api.openai.com/v1",
      "api_key": "${OPENAI_API_KEY}",
      "model": "gpt-4o",
      "context_window": 128000
    },
    "back_model": {
      "base_url": "https://api.openai.com/v1",
      "api_key": "${OPENAI_API_KEY}",
      "model": "gpt-4o-mini",
      "context_window": 128000
    }
  },
  "indexer": {
    "root_path": "./.go-agent",
    "chunker_type": "line",
    "max_lines": 100,
    "max_chars": 2000
  },
  "pgvector": {
    "host": "localhost",
    "port": 5432,
    "user": "postgres",
    "password": "${POSTGRES_PASSWORD}",
    "database": "vector_db",
    "dimension": 32
  },
  "embedding": {
    "api_key": "${EMBEDDING_API_KEY}",
    "base_url": "https://api.openai.com/v1",
    "model": "text-embedding-3-small",
    "dimensions": 32
  },
  "rerank": {
    "api_key": "${RERANK_API_KEY}",
    "base_url": "https://api.openai.com/v1",
    "model": "rerank-model"
  }
}
```

#### 配置项说明

**llm_providers**
- `front_model`: 主对话模型（用于与用户交互）
- `back_model`: 后台模型（用于摘要、记忆更新等）

**indexer**
- `root_path`: 索引存储根目录
- `chunker_type`: 分块类型（line: 按行分块）
- `max_lines`: 最大行数
- `max_chars`: 最大字符数

**pgvector**
- `host`: PostgreSQL 主机地址
- `port`: PostgreSQL 端口
- `user`: 数据库用户名
- `password`: 数据库密码（支持环境变量占位符）
- `database`: 数据库名称
- `dimension`: 向量维度

**embedding**
- `api_key`: Embedding API 密钥
- `base_url`: Embedding API 基础 URL
- `model`: Embedding 模型名称
- `dimensions`: 向量维度

**rerank**
- `api_key`: Rerank API 密钥
- `base_url`: Rerank API 基础 URL
- `model`: Rerank 模型名称

**注意**：配置文件中的 `api_key` 和 `password` 支持环境变量占位符 `${VARIABLE_NAME}`，实际使用时请在 `.env` 文件中设置对应的值。

### .env

```bash
OPENAI_API_KEY=your-openai-api-key
EMBEDDING_API_KEY=your-embedding-api-key
RERANK_API_KEY=your-rerank-api-key
POSTGRES_PASSWORD=123456
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

支持的 MCP 服务器类型包括：
- `filesystem`: 文件系统访问
- `postgres`: PostgreSQL 数据库访问
- `git`: Git 仓库操作
- 更多服务器可通过 MCP 协议集成

## 项目结构

```
go-agent/
├── agent/              # Agent 核心逻辑
│   ├── agent.go        # Agent 主结构体
│   ├── mcp.go          # MCP 客户端集成
│   ├── prompt.go       # 系统提示词管理
│   └── vo.go           # 请求/响应数据结构
├── context/            # 上下文管理
│   ├── policy_truncate.go      # 截断策略
│   ├── policy_summary.go       # 摘要策略
│   └── policy_test.go          # 策略测试
├── memory/             # 记忆系统
│   ├── memory.go       # 记忆管理
│   └── update.go       # 记忆更新
├── storage/            # 持久化存储
│   ├── storage.go      # 存储接口
│   └── filesystem.go   # 文件系统存储
├── tool/               # 工具集
│   ├── tool.go         # 工具接口定义
│   ├── factory.go      # 工具工厂
│   ├── bash.go         # Bash 命令工具
│   ├── docker_bash.go  # Docker 沙盒工具
│   ├── read.go         # 文件读取工具
│   ├── load_skill.go   # 技能加载工具
│   ├── load_storage.go # 存储加载工具
│   └── semantic_search.go  # 语义搜索工具
├── skill/              # 技能系统
│   ├── skill.go        # 技能管理
│   └── load.go         # 技能加载
├── rag/                # RAG 模块
│   ├── chunker.go      # 文本分块
│   ├── embedding.go    # Embedding 生成
│   ├── rerank.go       # 重排序
│   └── type.go         # RAG 类型定义
├── db/                 # 数据库集成
│   └── pgvector.go     # PostgreSQL 向量存储
├── index/              # 索引模块
│   ├── indexer.go      # 索引管理
│   └── file_walker.go  # 文件遍历
├── server/             # Web 服务
│   ├── service.go      # 业务逻辑
│   ├── controller.go   # HTTP 控制器
│   └── db.go           # 数据库操作
├── tui/                # 终端界面
│   ├── tui.go          # TUI 主程序
│   └── entry.go        # 入口文件
├── frontend/           # Web 前端
│   ├── src/            # React 源码
│   ├── package.json    # 前端依赖
│   ├── vite.config.js  # Vite 配置
│   └── tailwind.config.js  # Tailwind CSS 配置
├── cmd/                # 入口程序
│   ├── tui/            # TUI 入口
│   └── server/         # Web 服务入口
├── shared/             # 共享工具
│   ├── config.go       # 配置管理
│   ├── client.go       # LLM 客户端
│   ├── env.go          # 环境变量
│   ├── log/            # 日志
│   ├── mcp.go          # MCP 协议
│   ├── type.go         # 共享类型
│   └── util.go         # 工具函数
├── memory/             # Go Agent 记忆存储
├── agent.db            # SQLite 数据库
├── go.mod              # Go 模块定义
├── go.sum              # 依赖锁定
├── config.json         # 配置文件
├── config.example.json # 配置示例
├── mcp-server.json     # MCP 配置
├── mcp-server.example.json  # MCP 配置示例
├── .env                # 环境变量（需创建）
└── README.md           # 项目文档
```

## 核心功能

### 1. 上下文管理

智能的对话历史管理策略：

- **截断策略 (Truncate)**: 当上下文超限时，删除最旧的消息，保留最近对话
- **卸载策略 (Unload)**: 将长消息存储到外部存储，保留恢复提示
- **摘要策略 (Summary)**: 使用 LLM 压缩历史对话，保留关键信息

### 2. 记忆系统

支持两级记忆管理：

- **Global Memory**: `~/.go-agent` 目录，跨会话持久化
- **Workspace Memory**: 项目根目录的 `.go-agent/` 文件夹，项目级记忆
- LLM 驱动的自动记忆提取和更新

### 3. 技能系统

- 技能是描述性提示，指导 LLM 如何使用工具
- 元数据在启动时注入系统提示
- 完整内容按需加载（通过 `load_skill` 工具）
- 技能文件存储在 `.go-agent/skills/` 目录

### 4. RAG 检索

- 使用 Embedding 模型将代码/文档向量化
- 通过 pgvector 进行相似度检索
- 支持重排序提高检索精度
- 集成语义搜索工具

### 5. 沙盒安全

- Docker 容器隔离执行危险命令
- 人工确认机制（Allow/Reject/Always Allow）
- ESC 取消机制，随时终止 Agent 执行

### 6. 工具系统

内置工具包括：
- **bash**: 执行 Shell 命令
- **docker_bash**: 在 Docker 容器中执行命令（沙盒）
- **read**: 读取文件内容
- **semantic_search**: 语义搜索代码/文档
- **load_skill**: 动态加载技能文件
- **load_storage**: 加载存储内容

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

### 获取会话详情
```bash
GET /api/conversations/:conversation_id
```

### 获取消息列表
```bash
GET /api/conversations/:conversation_id/messages
```

### 保存记忆
```bash
POST /api/conversations/:conversation_id/memory
Content-Type: application/json

{
  "key": "user_preference",
  "value": "喜欢简洁的代码风格"
}
```

## 许可证

Apache License 2.0

## 致谢

本项目基于 [baby-agent](https://github.com/baby-llm/baby-agent) 教学项目整合而成。

## 相关资源

- [MCP 协议文档](https://modelcontextprotocol.io)
- [OpenAI Go SDK](https://github.com/openai/openai-go)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Gin Web Framework](https://github.com/gin-gonic/gin)
- [pgvector](https://github.com/pgvector/pgvector)

## 贡献

欢迎提交 Issue 和 Pull Request！
