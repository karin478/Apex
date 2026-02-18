# Apex Agent: Claude Code 长期记忆自治代理系统

## 架构设计文档 v11.0

**版本**: 11.0 | **更新**: 2026-02-18 | **作者**: Karin & Claude

> 设计原则：文件系统优先、零强制外部依赖、渐进增强。所有状态存于 `~/.claude/`，Docker / inotifywait 为可选能力。

**v11.0 变更摘要**（相对 v10.0）：

| # | 修复 | 影响范围 |
|---|------|---------|
| F1 | DAG 状态机补全 `RESUMING` 状态，定义合法转换 | §2.2 Orchestrator |
| F2 | Invariant I3 终态集扩展，覆盖 SUSPENDED/CANCELLED | §2.11 Correctness Invariants |
| F3 | CANCELLED 节点快照自动回滚策略 | §2.2 Cancellation Propagation |
| F4 | Fail-Closed 优先级高于环境策略，明确声明 | §2.1 Governance |
| F5 | 动态风险提升使用独立缩短 SLA | §2.1 Human Escalation SLA |
| F6 | vectors.db 双库写入事务边界与补偿 | §2.4 Memory Layer |
| F7 | replan 后断路器计数继承策略 | §2.2 DAG Invalidation |
| F8 | Kill Switch 独立 polling loop 明确声明 | §2.1 Kill Switch |
| F9 | Outbox 原子写入 tmpdir 同 mount 约束 | §2.11 Action Outbox |
| F10 | 新增 R43-R46 风险条目 | §7 风险与缓解 |
| F11 | Invariant 扩展为 I1-I9（新增 I8 双库最终一致、I9 Lock Ordering 运行时强制） | §2.11 Correctness Invariants |

---

## 1. 系统架构

### 1.1 总览

```
                            ┌────────┐
                            │  User  │
                            └───┬────┘
                                │
              ┌─────────────────▼──────────────────┐
              │         GOVERNANCE LAYER             │
              │  Risk Classifier │ Approval Gates    │
              │  Kill Switch     │ Human Escalation  │
              │  Dry-run / Plan  │ Platform Risk     │
              │  Fail-Closed Gate                    │
              └─────────────────┬──────────────────-┘
                                │
              ┌─────────────────▼──────────────────┐
              │        RUNTIME PRECHECK              │
              │  FS type │ Schema ver                │
              │  Platform Capability Matrix          │
              │  System Health Level                 │
              └─────────────────┬──────────────────-┘
                                │
              ┌─────────────────▼──────────────────┐
              │      ORCHESTRATOR (Microkernel)      │
              │  Planner │ State Engine              │
              │  Context Builder + Paging            │
              │  Snapshot Engine (CoW)               │
              │  Mode Selector │ Progress Tracker     │
              │  Multi-Workspace Coordinator          │
              └──┬──────┬──────┬──────┬─────────────┘
                 │      │      │      │
         ┌───────▼┐ ┌───▼──┐ ┌▼────┐ ┌▼──────────┐
         │ Agent  │ │Mem-  │ │Arti-│ │Cost &     │
         │ Pool   │ │ory   │ │fact │ │Resource   │
         │        │ │Layer │ │Reg. │ │Engine     │
         │        │ │+KG   │ │+DAG │ │+Circuit   │
         │        │ │+Vec  │ │Inv. │ │Breaker    │
         │        │ │DB    │ │+Path│ │+QoS       │
         │        │ │      │ │Sec. │ │           │
         └───────┬┘ └───┬──┘ └┬────┘ └┬──────────┘
                 └──────┴─────┴───────┘
                                │
              ┌─────────────────▼──────────────────┐
              │           EXECUTION LAYER            │
              │  Sync Runner  │ Async Runtime        │
              │  Event Runtime + Priority Router     │
              │  External Data Puller                │
              │  Execution Sandbox + Container       │
              │  Credential Injector (env-only)      │
              │  Aggregation │ Redaction              │
              └─────────────────┬──────────────────-┘
                                │
              ┌─────────────────▼──────────────────┐
              │           RUNTIME STATE              │
              │  Layered Lock + Ordering Protocol    │
              │  runtime.db (SQLite WAL) + Queue     │
              │  vectors.db (sqlite-vec, 独立)       │
              │  actions_wal.jsonl                   │
              │  Action Outbox Protocol              │
              │  Correctness Invariants              │
              └─────────────────┬──────────────────-┘
                                │
              ┌─────────────────▼──────────────────┐
              │  REASONING │ OBSERVABILITY & AUDIT   │
              │  Adversarial Review │ Causal Chain   │
              │  Audit Trail + Hash Chain + Anchor   │
              │  Trace ID │ Live Dashboard + TUI     │
              │  Run Manifest │ doctor + gc           │
              │  Metrics Export (可选)                │
              └────────────────────────────────────-┘
```

### 1.2 数据流

```
用户指令 → [环境预检 + Capability Matrix + System Health Check]
  → [Layered Lock + Ordering]
  → [Governance 预检 + Fail-Closed（优先级高于环境策略）]
  ├── [apex plan] → DAG 预览 + 成本估算 + 风险点 + rollback_quality → 等待确认
  └── [执行] → [Planner 规划]（分配 trace_id）
        ├── Context Builder 检索 + 分级压缩 + Paging
        ├── Cost Engine 预估 → 构建 DAG → Action Outbox 写入
        │     ├── Snapshot Engine（MEDIUM+ 前 CoW 快照 + 敏感排除）
        │     ├── Agent ↔ Artifact Registry（Path Security）→ 去抖 + 分桶
        │     └── 工具调用（Credential env 注入 + Outbox + 对账）→ Sandbox
        ├── 聚合（Aggregation）+ 可选推理增强（Reasoning Protocols）
        ├── Memory Staged Commit（Importance Routing + UNVERIFIED 隔离 + 双库补偿）
        ├── Redaction（含 error path）→ Audit + Hash Chain + Anchor
        └── 错误流: FAILED → retry_policy → RETRYING / ESCALATED / NEEDS_HUMAN
```

---

## 2. 各层详解

### 2.1 Governance Layer — 治理层

**风险分级**

| 级别 | 操作示例 | 策略 |
|------|---------|------|
| LOW | 读文件、运行测试、生成文档 | 自动执行 |
| MEDIUM | 写代码、修改配置、安装依赖 | 终端确认 |
| HIGH | 数据库迁移、删除数据、部署 staging | 审批文件（含 rollback_quality） |
| CRITICAL | 生产部署、修改加密密钥、基础设施变更 | 多人审批（强制展示 sandbox_level + rollback_quality） |

策略按环境覆盖：`dev`（全自动）→ `staging`（MEDIUM+ 确认）→ `prod`（LOW+ 确认，破坏性拒绝）

**动态风险提升**

| 触发条件 | 提升规则 |
|---------|---------|
| Connector `idempotency_support: none` | → MEDIUM+，无人审批禁止调用 |
| Sandbox 降级到 ulimit（macOS 等） | 涉及 `require_isolation_for` 列表 → 提升一档 |
| System Health Level ≥ RED | 仅允许 LOW 风险操作（见 2.14） |

**Fail-Closed 门槛**

`require_isolation_for` 中的 profile（默认 `plugin, ml_engineer, statistician`）+ 涉及敏感路径（`~/.aws`, `~/.ssh`, keychain）的操作：sandbox_level < bubblewrap 时**直接拒绝执行**。

> **[F4] Fail-Closed 优先级声明**：Fail-Closed 规则**无条件高于环境策略覆盖**。即使在 `dev` 环境（策略为"全自动"），Fail-Closed 条件命中时仍然拒绝执行。环境策略仅覆盖审批流程（是否需要人工确认），不覆盖安全硬约束。评估顺序：Fail-Closed 检查 → 通过后才进入环境策略的审批逻辑。

**macOS 替代执行路径**：Fail-Closed 前按 Capability Matrix 查询可用隔离后端：Linux bwrap 最优 → macOS/Windows 走 Docker/Colima/Lima container backend → 均不可用才 fail-closed。Container backend 作为可选 Execution Backend，不破坏"零强制依赖"。

**Kill Switch**

创建 `~/.claude/KILL_SWITCH` → Lock 持有者 200ms 检测 → **`kill -SIGTERM -${PGID}`**（进程组级）→ 5s 后未退出 `kill -SIGKILL -${PGID}` → Docker 容器联动 `docker kill` → 写入 Audit（含 hash chain）。Daemon 模式额外支持 SIGUSR1 即时触发。

> **[F8] Kill Switch 独立 Polling Loop**：Kill Switch 检测是**独立的 200ms polling loop**（`stat("~/.claude/KILL_SWITCH")` 每 200ms 一次），**不经由 Event Runtime**。这确保 Kill Switch 响应时间不受 Event Runtime 负载、优先级调度、或降级策略的影响。实现上：daemon 模式下为独立线程/goroutine；CLI 模式下在 Agent 执行的 wrapper 循环中内嵌检测。Event Runtime 的 IMMEDIATE 事件（500ms 轮询）仅处理业务级紧急事件，不包含 Kill Switch。

恢复：手动删除 KILL_SWITCH → `apex doctor` → `apex resume`。

**Dry-run / Plan 模式**

```bash
apex plan "审计 15 个微服务"              # 文本输出
apex plan "审计 15 个微服务" --format mermaid  # Mermaid 图
```

输出：DAG 结构、风险级别、Connector 列表、token 成本区间、受影响文件、Platform Capability 评估、rollback_quality 预估。不执行副作用。

**Human Escalation SLA**

| 审批类型 | 来源 | 等待上限 | 超时行为 |
|---------|------|---------|---------|
| MEDIUM 终端确认 | 原生 | 10 分钟 | 取消任务，释放并发槽 |
| MEDIUM 终端确认 | 动态提升 | 5 分钟 | 取消任务，释放并发槽 |
| HIGH 审批文件 | 原生 | 4 小时 | 暂停下游 DAG，提醒事件 |
| HIGH 审批文件 | 动态提升 | 1 小时 | 暂停下游 DAG，提醒事件 |
| CRITICAL 多人审批 | 任意 | 24 小时 | 升级告警，DAG BLOCKED |

> **[F5] 动态提升 SLA 缩短**：动态风险提升意味着运行环境偏离正常状态（sandbox 降级、幂等缺失等），此时需要更快的人工介入。动态提升到 MEDIUM 的操作使用 5 分钟 SLA（而非原生 MEDIUM 的 10 分钟），动态提升到 HIGH 使用 1 小时 SLA（而非原生 HIGH 的 4 小时）。CRITICAL 不区分来源，统一 24 小时。审批事件中记录 `escalation_source: native | dynamic_risk_升级原因`。

等待期间立即归还并发槽，通过后重新申请。

---

### 2.2 Orchestrator — 编排器

**微内核架构**

```
Planner:        build_dag / validate_dag / replan_subgraph
StateEngine:    transition / acquire_slot / release_slot / suspend / resume
ContextBuilder: build_context（弱幂等）+ context_page(id, range)
SnapshotEngine: create_snapshot / rollback / verify_rollback / gc_snapshots
```

ContextBuilder 和 SnapshotEngine 为异步非阻塞——它们的失败不阻塞 StateEngine 的 DAG 调度。ContextBuilder 依赖 Memory 检索，同输入不同时刻可能因 memory 更新产生不同输出，定义为**弱幂等**（输入需包含 memory_snapshot_version 才可复现）。

**执行模式**

| 模式 | 触发条件 | 行为差异 |
|------|---------|---------|
| NORMAL | 默认 | 标准流程 |
| URGENT | 关键词 / 事件优先级 | 可抢占，省略非必要验证（见下方不可跳过清单）；保留 20% token 预算 |
| EXPLORATORY | 开放式研究 | 多路并行，Adversarial Review |
| BATCH | 大批量同质任务 | 抽样验证，聚合压缩 |
| LONG_RUNNING | 复杂度 > 0.8 或跨 session | checkpoint，进度文件 |

**URGENT 模式不可跳过验证清单**

即使在 URGENT 模式下，以下验证**永不跳过**：

- Governance Fail-Closed 检查
- Sandbox re-check（MEDIUM+ 操作前）
- Path Security Contract
- Credential Injector（零信任注入）
- Audit hash chain 写入
- Outbox 协议步骤 1（WAL STARTED fsync）
- Lock Ordering 运行时断言（见 §2.11）

URGENT 可跳过的验证：Adversarial Review、非关键 Artifact checksum 全量校验、非关键 Memory NLI 检测、aggregation 压缩。

**DAG 状态机**

```
[F1] 完整状态定义:

核心生命周期:
  PENDING → BLOCKED → READY → RUNNING → COMPLETED
                                     → FAILED → RETRYING → COMPLETED
                                              → ESCALATED
                                              → NEEDS_HUMAN

中断与恢复:
  RUNNING → SUSPENDED（URGENT 抢占）
  SUSPENDED → RESUMING → RUNNING（变更权重 < 1.5，context 重建后）
  SUSPENDED → RESUMING → REPLANNING（变更权重 ≥ 1.5，需重新审批）
  REPLANNING → PENDING（新 DAG 子图注入）

级联取消:
  任意非终态 → CANCELLED（父节点 CANCELLED/FAILED(non-retriable)触发）

Invalidation:
  COMPLETED → INVALIDATED（Artifact 变更触发）
  INVALIDATED → PENDING（重新排入）

终态集: {COMPLETED, FAILED(non-retriable), ESCALATED, NEEDS_HUMAN, CANCELLED}
瞬态集: {PENDING, BLOCKED, READY, RUNNING, SUSPENDED, RESUMING, REPLANNING,
         RETRYING, INVALIDATED}

状态转换约束:
  - 终态不可转出（COMPLETED 除外，可被 INVALIDATED）
  - RESUMING 是受保护的瞬态，最大停留时间 30s，超时 → ESCALATED
  - REPLANNING 最大停留时间 60s，超时 → ESCALATED
```

**模式抢占与恢复**

URGENT 可抢占 → 被抢占任务进入 SUSPENDED。恢复时（**错峰调度，间隔 2s**）：

```
SUSPENDED → RESUMING
  → 检查 required Artifacts checksum 变化（对比 snapshot 前 checksum）
  → 计算变更影响权重:
      artifact_change_weight = Σ(changed_artifact.importance_score)
      # importance: exact=1.0, structural=0.7, reference=0.5, summarizable=0.3
  → weight ≥ 1.5: RESUMING → REPLANNING → Planner.replan_subgraph → 新 DAG 需重新审批
  → weight < 1.5: 变化节点走 INVALIDATED 流程，其余重建 context → RESUMING → RUNNING
```

强制重新执行 ContextBuilder，不复用暂停前的 `memory_context.md`。

**DAG 调度**

额外状态：`SUSPENDED`（被抢占）、`RESUMING`（恢复中，见上方状态机）、`INVALIDATED`（Artifact 变更）、`REPLANNING`、`CANCELLED`（父节点取消级联）

**Cancellation Propagation**：父节点 CANCELLED/FAILED(non-retriable) → 所有后代节点自动 CANCELLED，资源有序释放。

> **[F3] CANCELLED 节点快照回滚策略**：
>
> CANCELLED 节点如果已创建快照（MEDIUM+ 操作在执行前创建），按以下策略处理：
>
> | 节点状态 | 快照处理 | 原因 |
> |---------|---------|------|
> | CANCELLED（未开始执行，仅 PENDING/BLOCKED/READY） | 无快照，无需处理 | 未到快照创建阶段 |
> | CANCELLED（RUNNING 中被取消，有快照） | **自动 rollback** + verify_rollback | 已执行部分副作用，需恢复到快照状态 |
> | CANCELLED（SUSPENDED 中被取消，有快照） | **自动 rollback** + verify_rollback | 已执行部分副作用，恢复前需回滚 |
> | CANCELLED（RESUMING 中被取消，有快照） | **自动 rollback** + verify_rollback | 恢复过程中被取消 |
>
> 回滚执行顺序：叶子节点先回滚 → 逐级向上（避免父节点回滚覆盖子节点变更）。
>
> 回滚失败处理：单节点回滚失败 → 该节点标记 `CANCELLED_ROLLBACK_FAILED` + `rollback_quality` + 写 Audit → 不阻塞其他节点回滚 → 最终 ESCALATED 人工处理。
>
> 快照释放：成功回滚后快照不立即删除，保留至 gc 周期，以备人工审查。

**Dynamic DAG Invalidation**

```
Artifact 变更
  → 去抖（500ms / BATCH 2s）→ 语义规范化 checksum
  → normalized_checksum 未变 → 跳过
  → 变化 → 循环检测（DFS）→ 有循环则停止 + 告警
  → 分桶断路器: 计数维度 (source_artifact, consumer_node, 10min_window)
    同维度 > 3 次 → consumer ESCALATED + 阻断传播
    窗口过期 → 计数清零
  → 正常: consumers 置 INVALIDATED → 写 Audit（含原因链 + trace_id）
```

> **[F7] Replan 后断路器计数继承**：
>
> `Planner.replan_subgraph` 生成新 DAG 子图时，新节点获得全新 `node_id`，但**必须继承**被替换节点的断路器计数：
>
> ```
> replan_subgraph(old_subgraph) → new_subgraph:
>   for each (old_node, new_node) in node_mapping:
>     new_node.invalidation_buckets = old_node.invalidation_buckets
>     # 继承维度: (source_artifact, old_consumer_node) → (source_artifact, new_consumer_node)
>     # 计数值和窗口起点不变，仅替换 consumer_node 标识
> ```
>
> 这防止通过反复 replan 绕过断路器保护。新 DAG 子图的 `replan_source_node_id` 字段记录原始节点 ID，用于审计追溯。

**Snapshot & Rollback Engine**

MEDIUM+ 操作前自动快照（**执行前 lightweight capability re-check**，验证 sandbox/fs 后端仍可用）：

| 场景 | 方式 | 要点 |
|------|------|------|
| Git 工作区 | `git stash push --include-untracked -m "apex_snapshot_{id}"` | apply+drop（非 pop）；rollback 前校验 HEAD 一致性 |
| 非 Git (CoW FS) | `cp -a --reflink=auto` | Btrfs/XFS/APFS 零拷贝，写时复制天然隔离 |
| 非 Git (无 CoW) | `rsync -a --link-dest=prev` + OverlayFS | bwrap 模式下 OverlayFS 写隔离；无 bwrap 则 rsync fallback |

**为何不用裸 Hardlink**：Agent 脚本（Python `open('f','w')`、Node `writeFileSync`）原地修改文件时，写入同一 inode，穿透 hardlink 污染历史快照。reflink（CoW 浅拷贝）和 OverlayFS 均在写入时自动分离，确保快照不可变。Capability Matrix 启动时检测 FS 是否支持 reflink，不支持时降级 rsync + OverlayFS 或纯 rsync。

`snapshot_exclude.txt` 默认排除：`.env*`, `*.pem`, `*.key`, `id_rsa*`, `**/secrets/**`, `**/.aws/**` 等。快照后扫描遗漏的敏感文件（仅记录路径 hash）。

单文件超 `snapshot_max_file_size_mb`（默认 50MB）→ 跳过 + Audit 警告。

**rollback_quality 分级**

| 质量 | 条件 | 行为 |
|------|------|------|
| FULL | 无文件跳过 | 正常回滚 + `verify_rollback`（文件数 + checksum 校验） |
| PARTIAL | 存在大文件跳过 | 审批强提示 + verify 报告覆盖率 |
| STRUCTURAL_ONLY | 跳过含 .db/migrations/schema | 仅快照校验和+结构，回滚后需人工验证 |

Rollback 后自动执行 `apex verify-rollback`（校验关键路径文件数与 checksum 覆盖率），结果写 Audit。

Rollback：Git 先校验 HEAD → 不一致 ESCALATED；apply 冲突保留 stash → ESCALATED。配额：max_snapshots=10, max_snapshot_disk_mb=2048，超限 LRU 清理。

**Context Builder**

Token 预算默认 60k，四级压缩策略：

| Policy | 适用类型 | 行为 |
|--------|---------|------|
| exact | api_contract, schema, security_finding | 永不压缩；空间不足 → 降级 reference |
| reference | exact 降级 / 大型 artifact | 注入路径+checksum+关键片段索引 |
| structural | code_module, config, test_suite | 保留签名/类型/结构骨架，截断实现 |
| summarizable | report, dataset, issue | Haiku 摘要；失败降级 structural |

Reference 的实际读取必须经由 Artifact Registry（Path Security Contract 约束）。

**Context Paging Tool**：Agent 可主动调用 `context_page(artifact_id, line_range | ast_query)` 按需获取被压缩区段的原始内容。将静态压缩变为"静态摘要 + 动态按需 Fetch"，避免因过度压缩导致关键细节丢失。每次 Paging 消耗 Token 预算并记入 Cost Engine。

**Paging 配额**：`max_paging_per_task`（默认 10 次）、`max_paging_tokens_per_task`（默认 8k tokens）。超限 → ESCALATED，防止 Agent 陷入"page → 发现不够 → 再 page"循环。

驱逐优先级：summarizable → structural → reference → exact。仅当 reference 也无法容纳时报错重规划。

KG 查询深度限制：默认 depth=2，可按查询类型配置（`policy.yaml: kg_query_depth`）。额外 `max_kg_nodes`（默认 200）限制扇出，超深仅记录 entity_id 引用。

---

### 2.3 Agent Pool — 代理池

| Profile | 职责 |
|---------|------|
| backend / frontend | 后端/前端开发 |
| security_auditor / devops | 安全审计 / 部署运维 |
| ml_engineer / statistician | ML / 量化分析（**require_isolation**） |
| architect / causal_analyst | 技术选型 / 因果推理 |
| critic / advocate / judge | Red Team / Blue Team / 裁判 |
| quality_judge | 输出质量评估 |
| incident_responder | URGENT 专属 |
| knowledge_curator | KG 维护 |

Profile 结构：`role | system_prompt | allowed_tools | allowed_read_paths | allowed_risk_level | model | version`

超时：URGENT 60s / NORMAL 300s / LONG_RUNNING 1800s / BATCH 120s per item

自调用模式：Sub-Agent Delegation / Recursive Self-Call / Memory-Triggered / Verification Loop

---

### 2.4 Memory Layer — 记忆层

**三级架构**

| 级别 | 路径 | 生命周期 |
|------|------|---------|
| L1 工作记忆 | `session/{id}/` | 会话内 |
| L2 项目记忆 | `{project}/.claude/memory/` | 跟随项目 |
| L3 全局记忆 | `~/.claude/global_memory/` | 永久 |

多终端隔离：`--namespace {name}`。跨项目一致性：L3 对 KG 同名实体做交叉校验，矛盾写入 runtime.db `cross_project_links` 表（享受 Writer Queue 保护）。定期 `apex memory reconcile-cross-project` 合并或升级人工。

**向量检索基础设施**：使用 sqlite-vec 扩展（零外部依赖），在**独立的 `vectors.db`** 中维护 HNSW 索引（与 runtime.db 分离，避免向量扩展 corruption 影响核心事务数据）。100k 级条目检索 < 50ms。Embedding 模型优先本地（如 ONNX Runtime）→ 降级 API。

**Memory Staged Commit（分阶段提交）**

```
阶段 1: Extractor 提取候选 → staging_memories 表（状态 PENDING）

阶段 2: 分层冲突检测
  第一层 — 结构化字段比对:
    字段: (entity_id, property, value, cardinality, valid_from/to, observed_at, evidence)
    cardinality=single: 同 entity+property 不同 value → 检查时间重叠 → 矛盾 or 演进
    cardinality=set: 合并集合（不视为冲突）
    cardinality=map: 按 key 逐项比对
  第二层 — Embedding 初筛（cosine > 0.85，vectors.db）
  重要度路由:
    Ephemeral / Patterns(confidence<0.5) → 跳过 NLI，仅依赖前两层
    Facts / Decisions / Incidents → 进入第三层
  第三层 — NLI 三分类:
    CONTRADICTION → 冲突解决 / ENTAILMENT → 去重 / NEUTRAL → 共存
    旁路降级: 超时 3s 或 API 错误 → 标记 UNVERIFIED（confidence ×0.8）
    NLI 实现: 优先本地模型 → 降级 Haiku API → 兜底跳过仅依赖前两层

阶段 3: 提交
  PENDING → VERIFIED → COMMITTED（写入正式 memory 表）
  PENDING → UNVERIFIED → COMMITTED（带标记写入）
  PENDING → REJECTED → 保留 30 天后 gc 清理
  PENDING 超时（默认 1 小时）→ EXPIRED → gc 清理
```

> **[F6] 双库写入事务边界与补偿**
>
> Memory COMMITTED 涉及两个独立 SQLite 文件的写入（runtime.db 写记忆记录 + vectors.db 写 embedding 向量），这两个写入**不在同一事务中**。采用"runtime.db 优先 + vectors.db 异步补偿"策略：
>
> ```
> 提交顺序（严格）:
>   1. runtime.db: 写入 memory 表（COMMITTED 状态）+ vec_sync_status=PENDING
>   2. vectors.db: 写入 embedding → 成功后回写 runtime.db vec_sync_status=SYNCED
>
> 故障场景:
>   A. runtime.db 写成功，vectors.db 写失败:
>      → 记忆存在，但不可向量检索（仅关键词检索可达）
>      → vec_sync_status=PENDING 保持
>      → 后台补偿: 定期扫描 vec_sync_status=PENDING 的条目 → 重试写入 vectors.db
>      → 补偿超时（默认 3 次重试，间隔 exponential backoff）→ 标记 VEC_FAILED + Audit
>
>   B. runtime.db 写失败:
>      → 整条记忆未提交，vectors.db 不写入，无不一致
>
>   C. vectors.db 全库损坏:
>      → 降级为纯关键词检索（已有策略）
>      → 重建时: 扫描 runtime.db 所有 COMMITTED 记忆 → 批量重建 embedding
>      → 重建完成后: 批量更新 vec_sync_status=SYNCED
>
> 启动对账（新增步骤）:
>   → 扫描 runtime.db vec_sync_status=PENDING 的条目
>   → 逐条检查 vectors.db 中是否存在对应 embedding
>   → 存在 → 更新为 SYNCED（补上次崩溃丢失的回写）
>   → 不存在 → 重新写入 vectors.db
> ```
>
> 设计决策：**runtime.db 是记忆的 source of truth**，vectors.db 是可重建的派生索引。任何不一致状态下，系统以 runtime.db 为准。Invariant I8 保障最终一致。

**UNVERIFIED 记忆隔离与撤回**

- **消费隔离**：Agent 读取 UNVERIFIED 记忆时，ContextBuilder 标注 `[UNVERIFIED]`。
- **撤回传播**：补检发现 CONTRADICTION → 标记 `retracted=true` + `retracted_at` → 扫描撤回窗口内 Audit 中引用该记忆的 Action → 写入 `retraction_warning` 事件（含 trace_id）→ 受影响的未完成 DAG 节点触发 INVALIDATED。超窗口已完成的 DAG 仅写入警告事件，不触发自动回溯。
- **撤回 blast radius 限制**：单次撤回传播最多 INVALIDATED `max_retraction_invalidation`（默认 20）个 DAG 节点。超限时：停止自动传播 → 写入 `RETRACTION_OVERFLOW` 事件 → ESCALATED 人工确认剩余受影响节点。
- **可配置撤回窗口**（`policy.yaml: retraction_window_days`）：按记忆类型区分——Decisions 默认 30 天，Facts/Incidents 默认 14 天，其余默认 7 天。长期项目可全局上调。
- **entity-level 乐观锁**：staging_memories 写入时携带 `entity_version`（monotonic timestamp，避免 crash 后递增 ID 的 ABA 问题），并发写入同一 entity 时后者需重新检测。
- **冲突解决记录**：每次冲突解决写入 `resolution_reason`（时间演进 / 置信度差异 / 用户确认 / 人工）+ `superseded_entries` 列表，撤回传播时可解释"为何 invalidate"。

**类型化 Evidence**

```yaml
evidence: { type: file_hash|artifact_id|log_line|user_confirmation|conversation,
            ref: "sha256:...", locator: "L42" | "$.spec.endpoints[0]" | null }
# 强度: file_hash=1.0, artifact_id=0.9, log_line=0.7, user_confirmation=1.0, conversation=0.5
```

**置信度**：一致性×0.4 + Evidence 强度×0.4 + 来源可信度×0.2（不依赖 LLM 自评）

高风险（Decisions/Incidents）置信度 < 0.9 → 人工确认。

冲突解决：先检查时间有效性（过期直接 supersede）→ 置信度差 > 0.2 的获胜 → 相近取时间新 → 高风险升级人工。

**衰减与淘汰**

| 类型 | 衰减 | | 类型 | 衰减 |
|------|------|-|------|------|
| Decisions | 永不 | | Patterns | 按使用频率 |
| Facts | 按准确性 | | Incidents | 慢速 |
| Preferences | 永不 | | Ephemeral | 7 天清理 |

演化：初始 0.80 → 命中 +0.05 → 7 天未访问 ×0.95 → 用户确认 1.00 → NLI 旁路 ×0.8

**自动淘汰**：当 Memory 条目接近容量上限 80% 时，gc 自动淘汰 `confidence < 0.3 且 30 天未访问` 的条目（Decisions/Preferences 豁免）。

版本链：`mem_v1 → superseded_by → mem_v2`，带 `valid_to` 过期后自动 `is_current=false`。

混合检索：向量 0.5 + 关键词 0.3 + 时间衰减 0.2（向量由 vectors.db 提供）

**Knowledge Graph**：`kg.db`（SQLite），实体 `(type, canonical_name, project, namespace)` 四元组，关系含类型化 evidence 引用，schema_version 迁移。

---

### 2.5 Artifact Registry — 工件注册表

类型：`code_module | api_contract | dataset | model | config | test_suite | report | security_finding`

结构：`id | type | name | path | version | checksum | normalized_checksum | compression_policy | producer | consumers | dependencies | schema | run_id`

**代理读取接口**：`artifact_read(id, section?)` → Path Security Contract → Redaction → 返回内容 + Audit 记录。支持 AST grep / semantic search 辅助定位。

**Path Security Contract**

Registry 是信任边界的核心，所有文件读取必须遵守以下系统调用级约束：

```
读取前: realpath() 规范化 → 拒绝含 ".." 的路径 → 拒绝 symlink 目标在 allowed_read_paths 外
打开时: openat(dir_fd, path, O_NOFOLLOW | O_RDONLY) → dir_fd 限定到 artifact 根目录
读取后: 记录 (dev, inode) 到 Audit，保证"读的是同一个文件实体"
Section 读取: AST/semantic search 返回结构化偏移，禁止 Agent 提供任意 byte offset
```

Agent 不可直接 open path，path 仅做定位不做授权。

compression_policy 默认分配：exact(api_contract/schema/security_finding) / structural(code_module/config/test_suite) / summarizable(report/dataset) / reference(仅 exact 降级时)

内容寻址：同 checksum 不重复存储，缓存命中跳过 Agent 调用。

垃圾回收：`apex artifact gc [--dry-run]`（清理无引用 + 超 7 天的 artifact）

血缘图：`apex artifact impact {id}` → 返回所有直接/间接依赖链。

契约层：Backend 发布 `api_contract` → Registry 通知 Frontend → Frontend 获取 spec 后开发。

---

### 2.6 Execution Layer — 执行层

**Sync Runner**：加载记忆 → ContextBuilder 压缩/引用/Paging → Profile → Artifact → prompt → `claude --print` → 解析 → 发布产物 → Memory Staged Commit → Redaction → Audit（hash chain + anchor）

**Credential Injector（凭证零信任注入）**

Agent System Prompt 和工具输出中不出现真实 Token 值。LLM 看到的是占位符（如 `<GRAFANA_TOKEN_REF>`）。

| 场景 | 注入方式 | 要点 |
|------|---------|------|
| HTTP Connector | 执行层替换 HTTP header/body | Agent 不接触真实值 |
| 脚本执行 | `bwrap --setenv` / 临时 env var | **严禁**将 Token 替换到落盘代码文件或命令行参数 |
| Audit 记录 | 仅记录占位符 | 含异常栈/HTTP dump |

**错误路径防泄漏**：Connector 层强制要求错误对象不得包含原始请求 header/body。所有落盘日志（actions_wal / runtime.db error 字段 / audit）写入前过 Redaction，异常栈中的 Token/Secret 一律替换为占位符。

**Async Runtime**：DB Writer Queue 提交 → nohup 执行 → 周期 checkpoint → 新 session 恢复 → `active_pids.json` 孤儿清理

**Event Runtime**

| 优先级 | 轮询 | 场景 |
|--------|------|------|
| IMMEDIATE | 500ms | 生产告警（注：Kill Switch 不经由 Event Runtime，见 §2.1） |
| HIGH | 2s | PR、测试失败 |
| NORMAL | 5s（高负载动态降至 10s） | 定时、文件变更 |
| LOW | 30s | 报告、同步 |

可选 inotifywait/fsevents 驱动，轮询作为兜底。

**External Data Puller**

```yaml
# ~/.claude/data_sources/{name}.yaml
url: https://api.example.com/cves/latest
schedule: "0 */6 * * *"
auth:
  type: oauth2_client_credentials
  token_cache: { strategy: keychain_first, file_fallback: ~/.claude/runtime/tokens/{name}.cache }
transform: jq '.items[]'
emit_event: cve_fetched
timeout: 30s
retry: { max_attempts: 3, backoff: exponential }
rate_limit: { requests_per_minute: 10 }
```

**Execution Sandbox**

三档隔离 + Container Backend：

| 档位 | 网络隔离 | 路径隔离 | 依赖 | 平台 |
|------|---------|---------|------|------|
| Docker | ✓ | ✓ | docker | 全平台 |
| bubblewrap | ✓ | ✓（最小化挂载 + OverlayFS） | bwrap | Linux |
| Container Backend | ✓ | ✓ | Colima/Lima | macOS/Windows |
| unshare | ✓ | ✗ | linux | Linux |
| ulimit | ✗ | ✗ | 无 | 全平台 |

`require_isolation_for` 列表中的 profile → 按 Capability Matrix 查找可用隔离后端（**执行前 re-check 后端可用性**，< 50ms）→ 全部不可用才 fail-closed 拒绝。

**进程隔离**：所有 Agent 执行放入独立 PGID（`setsid`）。Kill/清理信号发送至 `-${PGID}`。Docker 环境联动 `docker kill`。可选 systemd cgroup 进一步隔离。

**幂等框架（四层保障）**

| 层 | 机制 | 作用 |
|----|------|------|
| 1 | runtime.db `actions` 表 precheck | 快速路径：已完成直接返回缓存 |
| 2 | `actions_wal.jsonl` 追加式日志 | 防 DB 回滚丢记录（O_APPEND + fsync） |
| 3 | Connector 幂等键 | 远端去重（header/param/body_field） |
| 4 | Connector 对账查询 | WAL STARTED 无 COMPLETED → 远端确认 |

Connector Spec 新增 `reconciliation` 声明：

```yaml
reconciliation:
  type: query_by_correlation  # query_by_correlation | query_by_temporal_attributes | none
  correlation_field: action_id
  query_endpoint: get_x_by_correlation
  response_exists_field: "$.data"
```

恢复时：有对账能力 → 自动查询（已完成补写 COMPLETED / 不存在标记 RETRIABLE / 查询失败 NEEDS_HUMAN）；无能力 → 直接 NEEDS_HUMAN。

retry_policy 分级：RETRIABLE（网络超时）/ NEEDS_HUMAN（业务错误）/ FORBIDDEN（生产破坏性）

**Connector 熔断器**

连续失败 5 次 → OPEN（60s 冷却）→ HALF_OPEN（单次探测）→ 成功进入 **RECOVERING**（渐进放量 1→2→4→正常，类似 slow start）/ 失败加倍冷却（最大 300s）。OPEN 期间依赖节点直接 BLOCKED。

**Redaction Pipeline**

覆盖所有落盘数据（**含异常栈、HTTP dump、error 字段**）。规则优先级：Credential Injector 占位符（第一道防线）→ 结构化字段（`*_TOKEN`/`*_SECRET`/`*_KEY`）→ 自定义 secrets（`policy.yaml → redaction.custom_secrets`）→ 正则模式（Bearer/ghp_/sk-/IP）→ 全扫描字段（reasoning/action/error/**exception_trace**）。IP 过滤可配置：`redact_ips: private_only | all | none`。可选 AES-256-GCM 加密 at-rest。

**Aggregation Pipeline**

```
Summarize:        每批 → Mini-Agent 压缩 → 汇总 → 报告
Structured Merge: 同 schema JSON → 合并去重 → 排序
Statistical Reduce: 数值 → statistician → Sandbox Python → 统计摘要
```

---

### 2.7 Cost & Resource Engine

Token 成本：复杂度路由 Haiku/Sonnet/Opus + 全局预算 + 熔断。NLI Haiku 调用纳入预算。Context Paging 消耗纳入预算。

**资源 QoS**：URGENT 模式保留 20% token 预算和 2 个并发槽，不可被 NORMAL/LONG_RUNNING 占满。

| 资源 | 默认上限 | 资源 | 默认上限 |
|------|---------|------|---------|
| 全局并发 Agent | 8 | 单 workspace 并发 | 4 |
| 单任务磁盘 | 500MB | 单任务内存 | 256MB |
| 快照总配额 | 2048MB | 快照数 | 10 |
| 快照单文件 | 50MB | CPU 优先级 | nice=10 |
| Paging 次数/任务 | 10 | Paging Token/任务 | 8k |

**Rate Limit Groups**

支持多个 Connector 共享同一个 rate limit pool，避免同一基础设施下的不同 Connector 各自满载导致基础设施过载：

```yaml
# policy.yaml
rate_limit_groups:
  k8s_internal:
    requests_per_minute: 30
    members: [grafana, prometheus, alertmanager]
  external_api:
    requests_per_minute: 60
    members: [github, jira]
```

单 Connector 的 `rate_limit` 仍然生效，取 min(自身限制, 所属 group 剩余配额)。

---

### 2.8 Reasoning Protocols

| 协议 | 触发条件 | 流程 |
|------|---------|------|
| Adversarial Review | 技术决策 | Advocate → Critic → Judge |
| Causal Chain | 跨系统调试 | 各服务收集 → Causal Analyst 因果链 |
| Hypothesis Board | 多代理调试 | propose / challenge / confirm / reject |
| Evidence Scoring | 所有论断 | 已验证 / 有支撑 / 推测 / 无根据 |

---

### 2.9 Observability & Audit

**审计字段**：`timestamp | trace_id | parent_action_id | session_id | agent_id@version | run_id | action_id | event_type | action | target_files | reasoning | risk_level | approval_status | outcome | cost_usd | rollback_ref | rollback_quality | sandbox_level | platform | prev_hash`

**Trace ID**：每个用户指令分配 `root_trace_id`，DAG 内所有 Action/Event 继承该 ID 并记录 `parent_action_id`，形成完整因果链。跨 async runtime 和 event 触发场景均可追溯。

**Audit Hash Chain + Mandatory Anchor**

每条 Audit 记录包含 `prev_hash = SHA-256(上一条记录全文)`，形成链式不可篡改日志。`apex doctor` 自动校验 hash chain 完整性。

**Daily Anchor（必选）**：每日生成 `daily_anchor = SHA-256(当日最后一条 audit 的 hash)`，写入方式按优先级：

1. OS keychain 签名存储（最安全）
2. Git tag 写入项目仓库（`apex-audit-anchor-{date}`）
3. 独立 append-only 文件 `~/.claude/audit/anchors.jsonl`（chmod 400）
4. 打印到 stdout/日志，由用户自行保存

启动时 `apex doctor` 校验最近 anchor 与链上 hash 一致性。anchor 缺失或不匹配 → 报告潜在篡改。

Policy 变更审计：`policy.yaml`、`connector spec`、`snapshot_exclude.txt` 的任何修改 → 写入 `policy_change` 事件 + 文件 checksum。

所有字段经 Redaction Pipeline 过滤后写入 `audit/{date}.jsonl`（append-only，hash chain，可选加密）。

**Live Dashboard** `~/.claude/runtime/dashboard.md`（`watch` 查看）。可选 TUI：`apex status [--filter BLOCKED,FAILED]`，含 Invalidation 原因链可视化（trace_id 串联）、TUI 审批（y/n/m + diff + rollback_quality + sandbox_level + 只读安全字段 + Modify 二次确认）。

**Run Manifest** `runs/{run_id}/manifest.json`：model/profile/connector 版本、input hash、platform、platform_capabilities、sandbox_level、cost、duration、trace_id。`apex run diff {id1} {id2}` 对比。

**Metrics Export（可选）**

Daemon 模式下可选导出 metrics，支持长期趋势分析：

```bash
apex metrics export --format prometheus  # 输出 Prometheus 格式
apex metrics export --format jsonl       # 输出 JSONL 格式
```

指标包括：DAG 吞吐/延迟/失败率、Token 消耗/预算使用率、Memory 条目增长/冲突率、Connector 熔断次数/恢复时间、Sandbox 使用分布、System Health Level 变化。

---

### 2.10 Tool Connector Spec

```yaml
name: grafana
type: http_api
spec_version: "1.2"              # Connector Spec 版本号
api_version: "v1"                # 目标 API 版本
base_url: https://grafana.internal/api
auth: { type: bearer_token, env_var: GRAFANA_TOKEN }  # Agent 仅见 <GRAFANA_TOKEN_REF>
circuit_breaker: { failure_threshold: 5, cooldown_seconds: 60 }
rate_limit_group: k8s_internal   # 可选，共享 rate limit pool
endpoints:
  query_metrics:
    path: /ds/query
    method: POST
    params: { required: [query, datasource_uid], optional: [start, end, step] }
    timeout: 10s
    retry: { max_attempts: 3, backoff: exponential }
    idempotency_support: none
    reconciliation: { type: none }
  create_alert_rule:
    path: /alerts/rules
    method: POST
    idempotency_support: header
    idempotency_key_name: "X-Idempotency-Key"
    reconciliation:
      type: query_by_correlation
      correlation_field: action_id
      query_endpoint: get_alert
      response_exists_field: "$.data.id"
allowed_agents: [incident_responder, devops]
risk_level: LOW
```

Run Manifest 中记录使用的 `spec_version` + `api_version`，方便回溯。

---

### 2.11 Runtime State DB

路径：`~/.claude/runtime/runtime.db`（SQLite WAL）

**运行时环境预检 + Platform Capability Matrix**

启动时检测并写入结构化能力声明：

```yaml
platform_capabilities:
  sandbox: [docker, bwrap, unshare, ulimit]  # 按可用性排序
  fs_reflink: true | false
  fs_type: ext4 | btrfs | apfs | nfs | ...
  flock: ok | no
  fsync_semantics: strong | weak
  container_backend: [colima, lima, docker_desktop, none]
```

NFS/CIFS/同步盘（Dropbox/OneDrive）→ 拒绝启动 + 提示 `export CLAUDE_RUNTIME_DIR=/path/to/local`。所有策略（fail-closed、risk escalate、plugin load、snapshot 方式）基于此矩阵数据驱动，而非代码 if-else。

**Schema Migration**

`PRAGMA user_version` 管控。迁移脚本版本化（forward-only + 迁移前自动备份）。daemon 与 CLI 启动时校验 schema version → 不匹配则拒绝写入。

**runtime.db 表**

| 表名 | 内容 |
|------|------|
| `dag_tasks` | 节点状态、attempt、checkpoint、invalidation_buckets、normalized_inputs_hash、replan_source_node_id |
| `event_queue` | 待处理事件（优先级、去重 hash、causality_id） |
| `artifact_index` | Artifact 索引（checksum/normalized_checksum/compression_policy） |
| `actions` | 幂等记录（action_id/trace_id/status/result）；>30d 归档到 `actions_archive` |
| `async_tasks` | 后台任务状态与 checkpoint |
| `staging_memories` | Memory 暂存区（含 nli_status、entity_version、resolution_reason、staging_state） |
| `memories` | 正式记忆表（含 vec_sync_status: PENDING/SYNCED/VEC_FAILED） |
| `cross_project_links` | 跨项目实体关联 |
| `connector_circuits` | Connector 熔断状态 |
| `snapshots` | 快照元数据（id/type/method/size/status/rollback_quality） |
| `system_health` | 各组件降级状态 + 全局 Health Level |

**vectors.db（独立 SQLite 文件）**

| 表名 | 内容 |
|------|------|
| `vec_memories` | Memory embedding + HNSW 索引 |
| `vec_artifacts` | Artifact embedding（可选） |
| `vec_meta` | 索引元数据、embedding 模型版本 |

vectors.db corruption 不影响 runtime.db，反之亦然。vectors.db 损坏时降级为纯关键词检索，后台重建索引。

**分层锁 + Lock Ordering Protocol**

```
锁获取顺序（严格不可逆）:
  1. 全局锁: ~/.claude/runtime/apex.lock（flock LOCK_EX | LOCK_NB）
     管控: global_memory、connectors circuit、tokens、schema migration
  2. Workspace 锁: {workspace}/.claude/runtime/ws.lock（flock）
     管控: 该 workspace 的 dag_tasks、artifact_index、project memory
     不同 workspace 可并行写入

Lock Ordering:
  - 需要全局锁 + workspace 锁: 必须先全局后 workspace
  - 需要多个 workspace 锁: 按 workspace_name 字典序依次获取
  - 跨 workspace 操作（reconcile-cross-project）: 持有全局锁 + 排队执行，
    避免同时持有多个 workspace 锁
  - 锁元数据: 每个锁文件写入 {pid, timestamp, lock_order_position, lock_version}
  - 旧版 CLI 遇到高于自身的 lock_version → fail-fast 并提示升级
  - doctor 输出: 当前锁持有者 PID、持有时长、阻塞队列长度、乱序检测

[F9/I9] Lock Ordering 运行时强制:
  - 锁获取函数内置 runtime assertion:
    acquire_lock(target):
      held = current_process.held_locks()  # 线程局部变量维护已持有锁列表
      for h in held:
        assert ordering(h) < ordering(target),
          "Lock ordering violation: holding {h} while acquiring {target}"
      # ordering: global=0, workspace=1+字典序位
  - 违规时: fail-fast + 写 Audit（LOCK_ORDER_VIOLATION 事件）+ ESCALATED
  - 这将 Lock Ordering 从"约定级"提升为"强制级"，Invariant I9 保障

可选 daemon: apex daemon start → 持有全局锁 + IPC unix socket
  其他 CLI → 通过 socket 发请求

崩溃恢复: flock 自动释放 + 启动时校验 PID 存活性
```

**DB Writer Queue**

单 Writer 线程 → 50-100ms 批量合并 → 单事务落盘。背压：队列上限 1000，超限阻塞调用方。Writer 崩溃 → 自动重启（最多 3 次，1s 间隔，均失败触发 Kill Switch）。读连接池最多 4 只读连接（WAL 并行无阻塞）。

**SQLite 运维**

Checkpoint 策略：正常运行 `PRAGMA wal_checkpoint(PASSIVE)`（不阻塞）；gc 时 `PRAGMA wal_checkpoint(TRUNCATE)`（回收 WAL）。DB 内数据归档：actions/dag_tasks 已完成 >30d → archive 表。VACUUM 仅在 daemon idle 时执行（或使用 `VACUUM INTO` 非阻塞重建）。

**Action Outbox 协议**

消除 "副作用已发生但 WAL 未记录" 的灰区：

```
严格顺序:
  1. WAL append STARTED (fsync)              — 此刻起崩溃可识别"可能有副作用"
  2. runtime.db action status=STARTED        — 单 writer 事务
  3. 执行 Connector（带 idempotency key / correlation id）env 注入
  4. runtime.db status=COMPLETED + result_ref — 同一 batch 含 artifact_index + dag_tasks
  5. Artifact: 写入目标目录下的 .tmp/ 子目录 → fsync → rename（原子落盘）
  6. WAL append COMPLETED (fsync)
  7. audit（含 hash chain + anchor check）/ manifest（可异步）
```

> **[F9] 步骤 5 原子写入约束**：`mktemp` 必须在**目标目录下的 `.tmp/` 子目录**创建临时文件（如 `~/.claude/artifacts/.tmp/`），**而非系统 `/tmp`**。这确保 `rename(2)` 在同一 mount point 内执行，保证原子性。如果 `rename` 返回 `EXDEV`（跨设备），视为**不可恢复错误** → ESCALATED + Audit（不降级为 copy+delete，因为非原子写入违反 Outbox 协议的原子性假设）。启动预检验证 `~/.claude/artifacts/.tmp/` 与 `~/.claude/artifacts/` 在同一 mount point（`stat` 比较 `st_dev`）。

```
启动对账:
  1. DB integrity_check → 失败从备份恢复
  2. WAL STARTED 无 COMPLETED → 副作用可能已发生
     → 有 Connector 对账 → 自动查询确认
     → 无对账能力 → NEEDS_HUMAN
     COMPLETED 无 DB 记录 → 同步回 actions 表
  3. Artifact: 文件存在 DB 无记录 → <1h 重新索引 / >7d 标记 orphan
              DB 有记录文件不存在 → 标记 MISSING + 通知依赖节点
  4. 孤儿进程: active_pids.json vs 实际 PID → 清理
  5. Audit hash chain 完整性校验 + anchor 比对
  6. vectors.db integrity_check → 失败标记降级，后台重建
  7. [F6] Memory 双库对账: 扫描 vec_sync_status=PENDING → vectors.db 补写
```

**Correctness Invariants**

故障注入验证的形式化基准：

```
I1: WAL.COMPLETED(action_id) 存在
    → DB.actions.status ∈ {COMPLETED} 最终可达（或 MISSING 明确标注）
I2: DB.actions.status=COMPLETED
    → result_ref 指向的 Artifact 存在，或 MISSING + 依赖节点 INVALIDATED
I3: 任何 STARTED 最终进入终态集 {COMPLETED, NEEDS_HUMAN, RETRIABLE, FORBIDDEN,
    CANCELLED, ESCALATED}（无"悬挂"状态）
    注: SUSPENDED 和 RESUMING 是受保护瞬态，有最大停留时间（30s/60s），
    超时自动转为 ESCALATED，因此不计入终态集但保证不悬挂。
    [F2] 相对 v10.0: 新增 CANCELLED 和 ESCALATED 为合法终态。
I4: 同一 action_id 的副作用最多执行一次
    （依赖 idempotency key / reconciliation 保障）
I5: trace_id 链路完整
    → 每个 Action 可追溯到 root_trace_id，无孤立节点
I6: Audit hash chain 连续
    → 任意相邻记录 prev_hash 校验通过，断链即报告篡改
I7: Daily anchor 一致
    → 每日 anchor hash 与链上对应位置的 hash 匹配
I8: Memory 双库最终一致（新增）
    → runtime.db 中 vec_sync_status=PENDING 的条目在有限时间内
      最终达到 SYNCED 或 VEC_FAILED（通过补偿机制或启动对账）
    → vectors.db 中不存在 runtime.db 中无对应 COMMITTED 记忆的孤立 embedding
    [F6] runtime.db 是 source of truth，vectors.db 是可重建的派生索引
I9: Lock Ordering 无违规（新增）
    → 运行时不存在任何进程在持有高序锁的同时获取低序锁
    → 任何 Lock Ordering 违规在 acquire_lock 时被 runtime assertion 拦截
    [F7] 将 Lock Ordering 从约定级提升为强制级
```

启动对账逐条映射到 I1-I9。Phase 8 故障注入（1k 次随机 SIGKILL）后自动验证全部 invariants。

备份：每次 Run 前备份（保留 5 份）+ 每小时在线备份（`sqlite3 .backup`，runtime.db 和 vectors.db 分别备份）+ integrity_check + 启动对账。CRITICAL 操作前额外执行 mini-backup。

文件写入规范：`mktemp（同 mount point .tmp/ 子目录）→ write → fsync → mv`（原子），`flock -x` 并发控制。

---

### 2.12 Plugin System

扩展点：Profiles / Connectors / Reasoning Protocols / Aggregators

plugin.yaml 声明 sandbox（`allowed_read/write_paths` + `network`），通过 OS 级隔离强制执行（共用 Sandbox 降级链）。降级到 ulimit 时插件**直接拒绝加载**。

**签名校验（分级强制）**：LOW 风险插件 SHA-256 即可；MEDIUM+ 插件必须签名（至少本地自签），否则 fail-closed 不加载。可选 GPG 签名。

生命周期：`scan` → `install` → `enable` → `disable`

---

### 2.13 Maintenance Subsystem

```bash
apex doctor    # 健康检查: DB integrity（runtime.db + vectors.db）、schema version、
               #           WAL 解析、锁状态（含持有者/时长/乱序检测）、
               #           FS 类型 + Capability Matrix、Audit hash chain + anchor 校验、
               #           孤儿进程/artifact、配额使用、Connector 熔断、
               #           UNVERIFIED 记忆数 + retraction 队列、daemon 状态、
               #           System Health Level 综合评估、
               #           Memory 双库 vec_sync_status 对账、
               #           tmpdir mount point 一致性检查

apex gc [--dry-run]  # 垃圾回收: 无引用 artifact(>7d)、超配额快照(LRU)、
                     #           audit 归档(>90d)、WAL 轮转(>7d)、
                     #           DB 数据归档(>30d) + VACUUM(idle)、
                     #           deprecated embeddings（vectors.db）、
                     #           rejected/expired staging(>30d)、
                     #           Memory 自动淘汰（confidence<0.3 且 30d 未访问）、
                     #           vec_sync_status=VEC_FAILED 超 7d 的重试或清理
```

推荐节奏：每日 doctor / 每周 gc --dry-run / 每月 gc。Daemon 模式下 Run 完成后自动轻量 doctor，配额 >80% 自动 gc。

---

### 2.14 System Health Level — 全局降级状态机

各组件独立降级可能导致"名义上在运行，但保障已大幅削弱"。System Health Level 综合多组件状态，提供全局降级决策。

**Health Level 定义**

| Level | 条件 | 行为 |
|-------|------|------|
| GREEN | 所有关键组件正常 | 正常运行 |
| YELLOW | 1-2 个非关键组件降级（如 NLI 旁路、向量检索降级） | 正常运行 + dashboard 告警 |
| RED | 关键组件降级（sandbox 降级、Docker 不可用、DB 备份失败） | 仅允许 LOW 风险操作；MEDIUM+ 需人工确认降级风险 |
| CRITICAL | 多个关键组件同时降级，或 DB integrity 异常 | fail-closed 全局，仅允许 `apex doctor` / `apex resume` |

**组件→Health 贡献矩阵**

```yaml
health_components:
  critical:    # 任一降级 → RED
    - sandbox_available    # 至少 unshare 以上可用
    - db_integrity         # runtime.db integrity_check 通过
    - audit_chain          # hash chain + 最近 anchor 校验通过
    - flock_available      # flock 正常工作
  important:   # 任一降级 → YELLOW，2+ 降级 → RED
    - nli_service          # NLI 可用（非旁路模式）
    - vector_search        # vectors.db 正常
    - container_backend    # Docker/Colima 可用
    - credential_store     # keychain / env var 正常
  optional:    # 降级仅记录
    - inotify              # 文件监控
    - metrics_export       # metrics 导出
```

Health Level 变更写入 `system_health` 表 + Audit。`apex status` 和 dashboard 实时展示。

---

## 3. 数据模型

```
Run ──▶ AgentSession ──▶ Artifact ──depends_on──▶ Artifact
             │                │                        │
             ▼                ▼                        │content_addressed
        AuditEntry ◀── MemoryEntry ──superseded_by──▶ MemoryEntry
         (prev_hash      │    (resolution_reason,          │
          trace_id)  cross_proj_link(DB)     superseded_entries)
                              │              vec_sync_status
                        entity_extracted_to ──▶ KGEntity

DAGTask ──depends_on──▶ DAGTask
  ├── requires/produces ──▶ Artifact
  ├── invalidation_buckets: {(source, window) → count}
  ├── replan_source_node_id: 原节点 ID（replan 时继承断路器计数）
  ├── cancellation_propagation: parent CANCELLED → children CANCELLED
  │     └── CANCELLED + has_snapshot → auto rollback（叶子优先）
  └── snapshot_ref ──▶ Snapshot(id, type, method, target_dir, size, status,
                               rollback_quality, verify_result)

Action ──▶ DAGTask, trace_id, parent_action_id,
           result_ref ──▶ Artifact, wal_ref, reconciliation_ref

MemoryEntry: entity_id, property, value, cardinality(single|set|map),
  valid_from/to, observed_at, evidence{type,ref,locator}, nli_status,
  entity_version(monotonic_ts), retracted, confidence,
  resolution_reason, superseded_entries, vec_sync_status

StagingMemory: 继承 MemoryEntry 字段 +
  staging_state(PENDING|VERIFIED|UNVERIFIED|REJECTED|EXPIRED|COMMITTED)
```

---

## 4. 文件系统布局

```
~/.claude/
├── CLAUDE.md
├── governance/
│   ├── policy.yaml          # redaction, sandbox, dag, require_isolation_for,
│   │                        # retraction_window_days, max_retraction_invalidation,
│   │                        # kg_query_depth, rate_limit_groups 等
│   ├── environments.yaml
│   └── snapshot_exclude.txt
├── agent_profiles/          # 含 allowed_read_paths
├── connectors/              # 含 idempotency, reconciliation, circuit_breaker,
│                            # spec_version, api_version, rate_limit_group
├── data_sources/ / plugins/ / global_memory/ / artifacts/
│                                                ├── .tmp/    # Outbox 原子写入临时目录（同 mount point）
│                                                └── ...
├── async_queue/{task_id}/   # output.log + checkpoints/
├── events/ (pending/ + processed/) / event_handlers/
├── audit/
│   ├── {date}.jsonl         # append-only + hash chain
│   ├── anchors.jsonl        # daily anchor hashes (chmod 400)
│   └── archive/
├── costs/log.jsonl / approvals/ / escalations/
├── sessions/{id}/           # memory_context.md + hypothesis_board.json
├── runtime/
│   ├── apex.lock            # 全局锁（含 lock_version 元数据）
│   ├── runtime.db           # SQLite WAL + Writer Queue（核心事务数据）
│   ├── vectors.db           # sqlite-vec HNSW（独立，可重建）
│   ├── runtime.db.hourly    # 备份
│   ├── vectors.db.hourly    # 备份
│   ├── actions_wal.jsonl    # Action Outbox 日志
│   ├── backups/
│   ├── capabilities.yaml    # Platform Capability Matrix（启动时生成）
│   ├── active_pids.json / daemon.pid / daemon.sock
│   ├── tokens/              # OAuth cache
│   └── dashboard.md
├── runs/{run_id}/manifest.json  # 含 trace_id + capabilities + connector versions
├── snapshots/{task_id}/     # chmod 700, CoW/OverlayFS metadata
├── scripts/                 # invoke, async_runner, event_daemon, sandbox_runner,
│                            # snapshot, redaction, reconcile, doctor, gc,
│                            # kill_switch, memory_*, kg_update, credential_injector,
│                            # env_precheck, schema_migrate, verify_rollback,
│                            # audit_chain_verify, anchor_sign, health_check,
│                            # metrics_export, vec_sync_compensator
├── progress/{project}.json / workspace_registry.json / KILL_SWITCH

{project}/.claude/
├── runtime/ws.lock          # Workspace 锁（含 lock_version 元数据）
├── memory/
│   ├── decisions/ facts/ patterns/ incidents/ preferences/ ephemeral/
│   ├── embeddings/
```

---

## 5. 设计原则

| # | 原则 | 要点 |
|---|------|------|
| P1 | 文件系统优先 | 所有状态在 `~/.claude/`（本地磁盘），无外部服务依赖 |
| P2 | 渐进增强 | 每个组件缺失时降级为更简单模式 |
| P3 | Hook 注入 | 新功能通过 hook 注入，不修改已稳定组件 |
| P4 | 结构化通信 | 代理间通过 Artifact Registry + JSON 通信 |
| P5 | 安全默认 | 未知环境 → prod；未知风险 → HIGH；Fail-Closed 无条件优先于环境策略 |
| P6 | 可观测优先 | 审计追踪 + hash chain + anchor 是最高优先级非功能需求 |
| P7 | 白名单工具 | Connector Spec 白名单（含版本），Agent 不可绕过 |
| P8 | 幂等副作用 | 四层：precheck + WAL + 幂等键 + 对账 |
| P9 | 原子状态变更 | temp+rename（同 mount point）+ Writer Queue + Outbox 顺序 |
| P10 | 纵深隔离 | Sandbox 网络+文件+进程，降级时 fail-closed |
| P11 | 分层锁 + 顺序 | 全局→字典序 workspace，跨项目操作全局排队，含版本协商 + 运行时强制 |
| P12 | 微内核编排 | 子模块接口分离，可独立测试和替换 |
| P13 | 零信任凭证 | Agent 不接触真实 Token，脚本场景仅 env var |
| P14 | 环境感知 | Capability Matrix 数据驱动策略选择，MEDIUM+ 执行前 re-check |
| P15 | 信任边界可验证 | Registry Path Security Contract，系统调用级约束 |
| P16 | 审计不可抵赖 | Hash chain + 必选 daily anchor，可检测篡改 |
| P17 | 形式化可验证 | Correctness Invariants（I1-I9），故障注入自动验证 |
| P18 | 全局降级感知 | System Health Level 综合多组件状态，数据驱动降级决策 |
| P19 | 故障域隔离 | 核心事务 DB 与向量索引分离，单点 corruption 不扩散 |
| P20 | 派生数据可重建 | vectors.db 是 runtime.db 的派生索引，双库不一致时以 runtime.db 为准 |

---

## 6. 非功能性需求

| 需求 | 指标 | 需求 | 指标 |
|------|------|------|------|
| 语义检索 | < 500ms (vectors.db) | 关键词检索 | < 100ms |
| Kill Switch | < 1s (+5s SIGKILL) | 子代理冷启动 | < 3s |
| 事件 IMMEDIATE | < 1s | 事件 NORMAL | < 5s |
| Context 压缩 | < 2s (Haiku) | Context Paging | < 200ms |
| Reference 注入 | < 100ms | Path Security | < 5ms overhead |
| Sandbox 标准 | < 200ms | Sandbox bwrap+OverlayFS | < 500ms |
| 快照 git stash | < 1s (<10k 文件) | 快照 reflink | < 500ms (中型) |
| DAG 去抖 | 500ms/2s | 计数衰减 | 10min |
| `apex plan` | < 3s / <10s(LLM) | DB Writer 批次 | < 100ms |
| Redaction | < 10ms/entry | NLI 单次 | < 3s (超时旁路) |
| Layered Lock | < 10ms | 启动对账 | < 5s / <30s(远端) |
| apex doctor | < 10s | apex gc | < 60s |
| 环境预检 | < 1s | Schema 校验 | < 100ms |
| Hash chain 校验 | < 1s/1k entries | Invariant 验证 | < 30s |
| Capability re-check | < 50ms | Health Level 计算 | < 10ms |
| Anchor 校验 | < 100ms | vectors.db 重建 | < 60s/10k entries |
| RESUMING 最大停留 | 30s | REPLANNING 最大停留 | 60s |
| Vec 补偿单条 | < 200ms | Vec 补偿全量扫描 | < 30s |
| Lock ordering assert | < 1ms | Mount point 校验 | < 10ms |

---

## 7. 风险与缓解

| # | 风险 | 级别 | 缓解 |
|---|------|------|------|
| R1 | 递归爆炸 | 高 | max_depth + max_breadth + depth counter |
| R2 | Token 成本失控 | 高 | Cost Engine + Context 预算 + Paging 配额 + 熔断 + QoS 预留 |
| R3 | 记忆中毒 | 中 | Staged Commit + Evidence + NLI(importance routing) + 隔离/撤回 |
| R4 | 级联错误放大 | 高 | normalized checksum + 去抖 + 分桶断路器 + slow start |
| R5 | 回声室 | 中 | Adversarial Review |
| R6 | 上下文漂移 | 中 | 分级压缩 + reference + Paging（含配额） + DAG 持久化 |
| R7 | 生产误操作 | 高 | Governance + Dry-run + TUI 审批 + Rollback(verify) |
| R8 | 协调开销 > 收益 | 中 | 简单任务不自调用；超 4 代理警告 |
| R9 | 工具滥用 | 高 | Connector 白名单 + allowed_agents + rate_limit_group |
| R10 | KG 腐化 | 中 | 实体标准化 + evidence 强制 + schema 迁移 + max_kg_nodes |
| R11 | 脚本破坏宿主机 | 高 | 三档 Sandbox + Container Backend + Fail-Closed + PGID Kill |
| R12 | 并发写损坏 | 高→低 | 分层锁 + Lock Ordering（含版本协商 + 运行时强制） + Writer Queue + 原子写 |
| R13 | 任务失败脏状态 | 中 | Snapshot CoW + rollback verify + quality 分级 + CANCELLED 自动回滚 |
| R14 | 幂等失效重复副作用 | 高 | 四层 + Outbox 顺序 + Invariant I4 |
| R15 | Orchestrator 过重 | 中→低 | 微内核 |
| R16 | DB 损坏全丢 | 高 | backup + integrity_check + Outbox 对账 + CRITICAL mini-backup |
| R17 | Token 明文泄漏 | 高 | 零信任 env-only + Redaction(含 error path) + chmod 600 |
| R18 | Invalidation 循环 | 中→低 | DFS + 分桶断路器（replan 继承计数） |
| R19 | 审计泄漏/篡改 | 中→低 | Redaction + Hash Chain + mandatory anchor + 加密 |
| R20 | 快照含敏感文件 | 中 | snapshot_exclude + chmod 700 + 命中统计 |
| R21 | 多进程死锁 | 高→低 | Lock Ordering Protocol + lock_version 版本协商 + 运行时断言 + doctor 锁诊断 |
| R22 | 崩溃三方不一致 | 高→低 | Outbox + 启动对账 + Invariants I1-I9（形式化可验证） |
| R23 | 非幂等恢复困难 | 中 | Connector 对账 + temporal_attributes fallback |
| R24 | macOS 无隔离 | 中→低 | Container Backend + Fail-Closed 兜底 |
| R25 | URGENT 恢复失配 | 中 | 变更权重 + 错峰恢复 + snapshot 基线比较 |
| R26 | NLI 堆积 | 中→低 | Importance Routing + 旁路降级 + Batch 补检 |
| R27 | 长期资源累积 | 中 | doctor + gc + DB 归档 + Memory 自动淘汰 + 配额触发 |
| R28 | NFS/同步盘锁失效 | 高 | 启动预检 + 热数据强制本地 |
| R29 | Agent 读取越权文件 | 中→低 | Path Security Contract + openat O_NOFOLLOW |
| R30 | Schema 版本不匹配 | 中 | PRAGMA user_version + 版本协商 + 迁移脚本 |
| R31 | 回滚看似完整实则缺失 | 中 | rollback_quality + verify_rollback 自动校验 |
| R32 | Hardlink 快照 inode 穿透 | 高→消除 | CoW reflink / OverlayFS 替代裸 Hardlink |
| R33 | Registry TOCTOU/symlink | 高→低 | realpath + openat + O_NOFOLLOW + (dev,inode) 审计 |
| R34 | 审计日志被篡改 | 中→低 | Hash chain + mandatory daily anchor + doctor 自动校验 |
| R35 | NLI 积压级联失效 | 中→低 | Importance Routing 减少 80%+ NLI 调用量 |
| R36 | 向量检索性能崩溃 | 中→消除 | vectors.db 独立，HNSW 索引，100k 级 < 50ms，corruption 不影响核心 DB |
| R37 | 运行中 capability 失效 | 中→低 | MEDIUM+ 执行前 lightweight re-check（< 50ms） |
| R38 | 多组件同时降级失察 | 中→低 | System Health Level 综合评估，RED 限制操作，CRITICAL fail-closed |
| R39 | Paging 循环耗尽 Token | 中→低 | max_paging_per_task + max_paging_tokens_per_task 配额 |
| R40 | 基础设施级 rate limit 过载 | 中→低 | rate_limit_group 跨 Connector 共享限流 |
| R41 | 旧版 CLI 违反 lock ordering | 中→低 | lock_version 字段，旧版遇新锁 fail-fast |
| R42 | 审计 chain 整体重写 | 中→低 | Mandatory daily anchor 外部锚点，检测从任意点重写 |
| R43 | CANCELLED 节点遗留脏状态 | 中→低 | CANCELLED 自动回滚快照（叶子优先）+ verify_rollback + 失败 ESCALATED |
| R44 | 双库写入不一致 | 中→低 | runtime.db 优先 + vec_sync_status 补偿 + 启动对账 + Invariant I8 |
| R45 | Replan 绕过断路器 | 中→低 | 新节点继承旧节点 invalidation_buckets + replan_source_node_id 审计 |
| R46 | 撤回级联 blast radius 过大 | 中→低 | max_retraction_invalidation 配额 + RETRACTION_OVERFLOW 人工升级 |
| R47 | Outbox 跨 mount rename 失败 | 中→消除 | tmpdir 同 mount point 约束 + 启动预检 st_dev 验证 + EXDEV fail-fast |

---

## 8. 实现路线

```
Phase 1  安全底线    Governance（含 Fail-Closed 优先级）+ Audit(hash chain + mandatory anchor) + Memory 基础版
Phase 2  成本编排    Cost Engine + DAG（含完整状态机） + Agent Pool + Artifact Registry
Phase 3  智能化      语义检索(vectors.db + 双库补偿) + Reasoning Protocols + Aggregation
Phase 4  异步事件    Async Runtime + Event Runtime + Priority Router + Kill Switch 独立 loop
Phase 5  生态扩展    Data Puller + Multi-Workspace + KG + Connectors + Dashboard
```

**Phase 6 子阶段依赖图**

```
Phase 6a 数据可靠
  DB Writer + 分层锁+Ordering（含运行时断言）+ 幂等四层 + Outbox（含同 mount 约束）+
  Invariants I1-I9 + Schema Migration + DB 运维 + 环境预检+Capability +
  双库补偿机制
         │
         ├───────────────┬──────────────────┐
         ▼               ▼                  ▼
Phase 6b 执行安全    Phase 6c 智能质量    Phase 6d 安全合规
  Sandbox +            ContextBuilder +      Redaction(error) +
  Container Backend +  Paging(含配额) +     TUI 审批 +
  Fail-Closed +        Registry +            OAuth keychain +
  PGID Kill +          微内核 +              Plugin 签名分级
  Snapshot CoW +       Staged Commit +
  Path Security +      NLI Routing +
  Credential env +     隔离/撤回 +
  DAG Invalidation     Trace ID + QoS +
  (含 replan 继承) +  URGENT 重规划 +
  Connector 熔断 +    CANCELLED 自动回滚
  slow start
         │               │                  │
         └───────────────┴──────────────────┘
                         │
                         ▼
                  Phase 6e 全局治理
                    System Health Level +
                    Rate Limit Groups +
                    Connector Spec 版本化 +
                    Lock 版本协商 +
                    vectors.db 独立化 +
                    Metrics Export +
                    Paging 配额 +
                    KG 可配深度 +
                    撤回 blast radius 限制
```

```
Phase 7  维护生态    doctor+gc（含双库对账+tmpdir校验）+ Plugin OS 隔离 + Run Manifest +
                     内容寻址 + Artifact GC + 血缘图 + Memory 导入导出 +
                     跨项目对账 + TUI Dashboard + Memory 自动淘汰

Phase 8  可信验证    故障注入测试框架（关键点随机 SIGKILL × 1k 次）+
                     Invariant I1-I9 自动验证 + Hash chain + anchor 完整性验证 +
                     恢复正确性（无重复副作用、无丢失、无幽灵记录、无篡改）+
                     vectors.db 独立 corruption/恢复验证 +
                     双库最终一致性验证 + Lock Ordering 违规注入测试 +
                     CANCELLED 回滚正确性验证 + 跨 mount rename 失败注入
```

---

## 附录 A：术语表

| 术语 | 定义 |
|------|------|
| Action Outbox | 严格顺序的副作用提交协议，消除灰区 |
| Audit Hash Chain | 每条审计含 prev_hash，形成链式不可篡改日志 |
| Capability Matrix | 结构化平台能力声明，策略数据驱动 |
| Capability Re-check | MEDIUM+ 执行前 lightweight 验证后端可用性 |
| Container Backend | macOS/Windows 可选 Docker/Colima 隔离后端 |
| Context Paging | Agent 按需 Fetch 被压缩区段原始内容，含配额限制 |
| Correctness Invariant | 9 条形式化规则（I1-I9），故障注入验证基准 |
| CoW Snapshot | reflink/OverlayFS 替代裸 Hardlink，写时复制隔离 |
| Credential Env-Only | 脚本场景仅通过 env var 注入凭证 |
| Daily Anchor | 每日 audit chain 的外部锚点（必选），检测整体重写 |
| Lock Ordering | 全局→字典序 Workspace 的严格锁获取顺序，含版本协商 + 运行时强制断言 |
| Memory Staged Commit | 分阶段提交：PENDING → VERIFIED/UNVERIFIED/REJECTED/EXPIRED → COMMITTED |
| NLI Importance Routing | 按记忆重要度决定是否走 NLI 全量检测 |
| Path Security Contract | realpath + openat + O_NOFOLLOW 的系统调用级约束 |
| Rate Limit Group | 多 Connector 共享一个 rate limit pool |
| System Health Level | GREEN/YELLOW/RED/CRITICAL 全局降级状态机 |
| sqlite-vec | SQLite 向量搜索扩展，HNSW 索引（独立 vectors.db） |
| Trace ID | root_trace_id + parent_action_id 贯穿全链路因果链 |
| vec_sync_status | Memory 记录在 vectors.db 中的同步状态（PENDING/SYNCED/VEC_FAILED） |
| vectors.db | 独立的向量索引数据库，corruption 不影响核心 runtime.db，可从 runtime.db 重建 |

---

## 附录 B：容量规划

| 规模 | 并发 | Token 预算 | 快照配额 | Artifact 磁盘 | Memory 条目 | KG 实体 |
|------|------|-----------|---------|-------------|------------|---------|
| 小型（单服务） | 2-4 | 30k | 5 × 512MB | 500MB | <5k | <500 |
| 中型（3-5 服务） | 4-6 | 60k | 10 × 2GB | 2GB | <20k | <2k |
| 大型（10+ 服务） | 6-8 | 90k | 15 × 4GB | 5GB | <50k | <5k |
| Monorepo | 8+daemon | 120k | 20 × 8GB | 10GB | <100k | <10k |

---

## 附录 C：故障恢复 Runbook

| 故障 | 恢复 |
|------|------|
| runtime.db 损坏 | sqlite3 .backup 恢复 → 启动对账 → WAL 补全 → Invariant 验证 |
| vectors.db 损坏 | 标记降级（纯关键词检索）→ 后台从 runtime.db memory 数据重建索引 |
| Kill Switch 后 | 删 KILL_SWITCH → doctor → resume |
| Daemon 崩溃 | `apex daemon start`（自动获锁+对账） |
| 多实例冲突 | `apex status` 查看 → `apex attach` 或等待 |
| 死锁排查 | `apex doctor`（锁持有者/时长/阻塞队列/乱序检测）→ 检查 Lock Ordering 违规（运行时已强制拦截） |
| Connector 熔断 | 检查外部服务 → 等待 slow start 恢复 或 `apex connector reset` |
| 快照磁盘满 | `apex gc` → 调整配额 |
| NLI 堆积 | `apex memory verify-pending` → Importance Routing 已减少 80%+ 负载 |
| WAL 不确定 | Outbox STARTED 无 COMPLETED → 有对账自动查询 → 无则 `apex action resolve {id}` |
| NFS/同步盘启动失败 | `export CLAUDE_RUNTIME_DIR=/local/path` → 重启 |
| Schema 版本不匹配 | 升级 CLI → `apex doctor` → 自动迁移 |
| Outbox 崩溃 | 步骤 1-2: 无副作用安全重试; 步骤 3: 对账确认; 步骤 4-6: DB 补全 |
| UNVERIFIED 撤回 | 自动 INVALIDATED 撤回窗口内受影响节点（受 max_retraction_invalidation 限制） → 检查 retraction 事件(trace_id 追溯) |
| 回滚质量不完整 | 检查 rollback_quality → verify_rollback 结果 → PARTIAL/STRUCTURAL 需人工验证 |
| Audit hash chain 断链 | `apex doctor` 报告断点位置 → 校验 anchor → 检查是否被篡改 → 从备份恢复 |
| Audit anchor 不匹配 | 比对 anchor 存储与链上 hash → 确认篡改范围 → 从备份恢复受影响日期段 |
| 运行中 capability 失效 | 自动 re-check 检测 → 降级或 fail-closed → doctor 重新生成 capabilities.yaml |
| System Health RED/CRITICAL | `apex status` 查看降级组件 → 逐一恢复 → Health 自动回升 |
| 整机迁移 | 复制 `~/.claude/` → 目标机 `apex doctor` → 重建 capabilities.yaml + 锁文件 |
| Staging memory 堆积 | `apex doctor` 检查 PENDING/EXPIRED 数量 → gc 清理超时条目 |
| 旧版 CLI 锁冲突 | 升级 CLI 版本 → `apex doctor` → 锁文件自动更新 lock_version |
| CANCELLED 回滚失败 | 检查 `CANCELLED_ROLLBACK_FAILED` 节点 → 手动 verify → `apex snapshot rollback {id}` |
| Memory 双库不一致 | `apex doctor` 检查 vec_sync_status=PENDING/VEC_FAILED → 手动 `apex memory vec-resync` |
| Outbox EXDEV 错误 | 检查 .tmp/ 目录 mount point → `apex doctor` 验证 → 修复 mount 配置后重启 |
| 撤回溢出 | 检查 RETRACTION_OVERFLOW 事件 → `apex memory retraction-review {trace_id}` → 人工确认剩余节点 |
| Lock ordering 违规 | 检查 LOCK_ORDER_VIOLATION 审计事件 → 修复调用代码 → 重启 |
