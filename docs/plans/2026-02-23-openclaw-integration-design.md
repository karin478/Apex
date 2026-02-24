# OpenClaw 集成设计方案（路线 B：编排层）

> **定位**: Apex = openclaw 之上的编排层，不重复 openclaw 已有功能
> - openclaw 负责：模型路由、provider 认证、skills、channels、插件
> - Apex 负责：DAG 任务分解、治理审批、风险分级、沙箱隔离、审计链、kill switch
> - Apex 调用 openclaw gateway 作为 "模型访问层"

## 一、问题分析

### 当前状态

| 功能 | 现状 | 目标 |
|------|------|------|
| `/model` | 仅修改内存中的字符串 | 从 openclaw config 读取真实模型列表，交互式切换 |
| 执行器 | 硬绑定 `claude` CLI | 双执行器：Claude CLI + Gateway HTTP |
| `/status` | 调用空的 cobra handler | 显示 gateway 连通性 + 当前模型 + 认证状态 |
| `/doctor` | 调用空的 cobra handler | 检查 gateway/CLI/config 是否正常 |
| `/config` | 只读显示 apex config | 显示 apex config + openclaw 集成状态 |

### 不做的事（交给 openclaw CLI）

| 功能 | 原因 |
|------|------|
| Provider 认证管理 | `openclaw models auth login` 已有完整 OAuth/token 流程 |
| Plugin 启用/禁用 | `openclaw` CLI 已有，且需要重启 gateway 生效 |
| Skills 安装/管理 | `openclaw skills install` 已有 |
| Channel 管理 | openclaw 核心功能，不该在 Apex 重复 |
| Gateway 配置修改 | 直接编辑 `~/.openclaw/openclaw.json`，Apex 只读取 |

---

## 二、架构

### 双执行器 + 模型路由

```
User → Apex TUI → Model Router
                     │
         ┌───────────┴───────────┐
         │                       │
   claude-cli/*            其他 provider/*
         │                       │
   Claude CLI Executor    Gateway HTTP Executor
   (现有，保持兼容)       (新增，/v1/chat/completions)
         │                       │
   Anthropic API          openclaw gateway → 任意 provider
```

**路由规则**：
- `claude-cli/*` → 现有 Claude CLI executor（直接调用 `claude` binary）
- 其他（`google-antigravity/*`、`anthropic/*`、`openai/*` 等）→ Gateway HTTP
- 无 provider 前缀 → 查 openclaw config 别名，或默认 claude-cli
- Gateway 不可用时 → 只暴露 claude-cli 模型，优雅降级

---

## 三、实施计划（4 个 Phase）

### Phase 1: OpenClaw Config Bridge

**目标**：读取 `~/.openclaw/openclaw.json`，获取模型列表和 gateway 信息

**新增文件**：
- `internal/openclaw/config.go` — 解析 openclaw.json
- `internal/openclaw/models.go` — 模型发现

**数据模型**：
```go
// internal/openclaw/config.go
type Config struct {
    Auth    AuthConfig    `json:"auth"`
    Agents  AgentsConfig  `json:"agents"`
    Gateway GatewayConfig `json:"gateway"`
}

type AuthConfig struct {
    Profiles map[string]AuthProfile `json:"profiles"`
}

type AuthProfile struct {
    Provider string `json:"provider"`
    Mode     string `json:"mode"` // "token", "oauth"
}

type AgentsConfig struct {
    Defaults AgentDefaults `json:"defaults"`
}

type AgentDefaults struct {
    Model       ModelConfig            `json:"model"`
    Models      map[string]ModelMeta   `json:"models"`
    CLIBackends map[string]CLIBackend  `json:"cliBackends"`
}

type ModelConfig struct {
    Primary   string   `json:"primary"`
    Fallbacks []string `json:"fallbacks"`
}

type ModelMeta struct {
    Alias string `json:"alias,omitempty"`
}

type CLIBackend struct {
    Command      string            `json:"command"`
    ModelAliases map[string]string `json:"modelAliases"`
}

type GatewayConfig struct {
    Port int    `json:"port"`
    Mode string `json:"mode"`
    Bind string `json:"bind"`
}
```

```go
// internal/openclaw/models.go
type ModelEntry struct {
    ID       string // "google-antigravity/claude-opus-4-6-thinking"
    Provider string // "google-antigravity"
    Model    string // "claude-opus-4-6-thinking"
    Alias    string // optional, e.g. "sonnet"
    IsCLI    bool   // true if routed via CLI backend
}

// AvailableModels 从 config 提取所有模型
func AvailableModels(cfg *Config) []ModelEntry

// FindModel 模糊匹配模型名（支持别名、前缀省略）
func FindModel(models []ModelEntry, query string) *ModelEntry

// DefaultModel 获取默认模型
func DefaultModel(cfg *Config) string

// GatewayURL 构造 gateway base URL
func GatewayURL(cfg *Config) string
```

**JSON5 处理**：openclaw.json 是标准 JSON（无注释），直接 `json.Unmarshal`。

**修改文件**：
- `cmd/apex/interactive.go` — session 启动时加载 openclaw config

---

### Phase 2: Gateway HTTP Executor + Router

**目标**：新增 HTTP executor，双执行器路由

**新增文件**：
- `internal/executor/gateway.go` — OpenAI-compatible chat completions client
- `internal/executor/router.go` — 根据 model ID 路由

```go
// internal/executor/gateway.go
type GatewayExecutor struct {
    baseURL    string // "http://127.0.0.1:18789"
    httpClient *http.Client
}

func NewGateway(baseURL string, timeout time.Duration) *GatewayExecutor

// Run 通过 /v1/chat/completions 发送请求，返回完整结果
func (g *GatewayExecutor) Run(ctx context.Context, model, task string) (Result, error)

// RunStream SSE 流式，每个 delta 调用 onChunk
func (g *GatewayExecutor) RunStream(ctx context.Context, model, task string, onChunk func(string)) (Result, error)

// Health 检查 gateway 是否可达
func (g *GatewayExecutor) Health(ctx context.Context) error
```

```go
// internal/executor/router.go
type Router struct {
    claude  *Executor         // 现有 Claude CLI
    gateway *GatewayExecutor  // 新增 gateway
    cliPrefixes []string      // ["claude-cli"] — 走 CLI 的 provider 前缀
}

func NewRouter(claude *Executor, gateway *GatewayExecutor, cliPrefixes []string) *Router

// Run 自动路由：CLI prefix → claude executor, 其他 → gateway
func (r *Router) Run(ctx context.Context, model, task string, opts ...RunOption) (Result, error)

// Available 返回 gateway 是否可用
func (r *Router) GatewayAvailable() bool
```

**HTTP 请求格式**：
```json
POST /v1/chat/completions
{
  "model": "google-antigravity/gemini-3-flash",
  "messages": [{"role": "user", "content": "task text"}],
  "stream": true
}
```

**SSE 流式解析**：标准 `data: {...}` 格式，`data: [DONE]` 终止

**修改文件**：
- `cmd/apex/stream.go` — 使用 Router 替代直接 Executor
- `cmd/apex/interactive.go` — 初始化 Router

---

### Phase 3: 交互式模型选择

**目标**：`/model` 变成真正能切换模型的选择器

**修改文件**：
- `cmd/apex/slash.go` — 重写 `cmdModel`

**用户体验**：
```
❯ /model
  Current: claude-cli/claude-opus-4-6

  claude-cli
    ● claude-opus-4-6         (active)
      claude-sonnet-4-5

  google-antigravity
      claude-opus-4-5-thinking
      claude-opus-4-6-thinking
      claude-sonnet-4-5
      claude-sonnet-4-5-thinking
      gemini-3-flash
      gemini-3-pro-high
      gemini-3-pro-low
      gpt-oss-120b-medium

  Enter model name or number: _
```

**快捷切换**：
```
❯ /model gemini-3-flash            # fuzzy 匹配
❯ /model opus                       # 别名
❯ /model 5                          # 序号
```

**切换后效果**：
1. 更新 session 的 active model
2. Router 自动按新 model 路由到正确 executor
3. 状态栏实时显示新模型
4. Spinner 显示新模型名

**Gateway 降级**：gateway 不可用时只显示 claude-cli 模型，其他标灰 + "(gateway offline)"

---

### Phase 4: /status 与 /doctor 实战化

**目标**：真实反映系统状态

**修改文件**：
- `cmd/apex/slash.go` — 重写 `cmdStatus`、`cmdDoctor`

**`/status` 输出**：
```
  ◆ System Status

  Model       claude-cli/claude-opus-4-6
  Gateway     ✓ running (port 18789)
  Sandbox     ulimit (auto-detected)
  Session     5 turns · 2.3k chars context
  Risk gate   AUTO_APPROVE: LOW | CONFIRM: MEDIUM | REJECT: HIGH+
```

**`/doctor` 输出**：
```
  ◆ Health Check

  [✓] openclaw config     ~/.openclaw/openclaw.json found (11 models)
  [✓] Gateway reachable   http://127.0.0.1:18789
  [✓] Claude CLI           claude v1.x.x
  [✓] Auth profiles       2 profiles (anthropic, google-antigravity)
  [✓] Sandbox             ulimit available
  [✓] Memory store        ~/.apex/memory/ writable
  [✓] StateDB             ~/.apex/runtime/state.db OK

  Tip: Provider auth → openclaw models auth login --provider <name>
       Plugin mgmt  → openclaw (use openclaw CLI directly)
```

注意最后的 Tip — 明确引导用户用 `openclaw` CLI 管理 provider/plugin，不在 Apex 里重复。

---

## 四、文件变更清单

### 新增文件（6 个）

| 文件 | 用途 | Phase |
|------|------|-------|
| `internal/openclaw/config.go` | 解析 openclaw.json | 1 |
| `internal/openclaw/models.go` | 模型发现/匹配 | 1 |
| `internal/openclaw/config_test.go` | Config 解析测试 | 1 |
| `internal/openclaw/models_test.go` | 模型匹配测试 | 1 |
| `internal/executor/gateway.go` | HTTP executor | 2 |
| `internal/executor/router.go` | 执行器路由 | 2 |

### 修改文件（4 个）

| 文件 | 变更 | Phase |
|------|------|-------|
| `cmd/apex/interactive.go` | 加载 openclaw config, 初始化 Router, session 增加 ocConfig + router | 1,2 |
| `cmd/apex/stream.go` | 使用 Router.Run 替代 executor.Run | 2 |
| `cmd/apex/slash.go` | 重写 /model（交互选择）, /status（真实状态）, /doctor（健康检查）, /config（含 openclaw） | 3,4 |
| `go.mod` | 无新依赖（标准库 net/http + encoding/json 即可） | — |

---

## 五、风险与降级

| 场景 | 行为 |
|------|------|
| `~/.openclaw/openclaw.json` 不存在 | 纯 apex 模式：只有 claude-cli，/model 显示 "openclaw not found" |
| Gateway 未启动 | /model 只列出 claude-cli 模型，其他标灰 |
| Gateway 启动但 chatCompletions 未启用 | /doctor 提示启用方法 |
| OAuth token 过期 | 401 时提示 `openclaw models auth login` |
| 用户选了 gateway 模型但 gateway 挂了 | 执行时报错 + 建议 `/model` 切回 claude-cli |

---

## 六、测试策略

1. **Phase 1**: 单元测试 config 解析（含边界：空文件、缺字段、未知字段）、模型列表提取、fuzzy 匹配
2. **Phase 2**: Mock HTTP server 测试 gateway executor（正常、流式、超时、401、500）、路由逻辑
3. **Phase 3**: 模型匹配逻辑测试（别名、序号、fuzzy、大小写）
4. **Phase 4**: Gateway 健康检查 mock 测试
5. **集成**: 全量 `go test ./...` 确保不破坏现有 57 包

---

## 七、执行顺序

```
Phase 1 (Config Bridge) → Phase 2 (Gateway Executor) → Phase 3 (Model Picker) → Phase 4 (Status/Doctor)
```

Phase 1-3 完成后核心功能即可用：真实模型列表 + 交互式切换 + 双执行器路由。
Phase 4 是锦上添花。
