# Apex CLI Live Scenario Testing — Design

**Goal:** Systematically test Apex CLI with real-world scenarios to identify capability boundaries.

**Method:** Progressive stress testing (L1→L5), each level increasing complexity.

**Safety:** No git push, no commits to project repo, all output to `/tmp/apex-tests/`.

---

## Scoring Dimensions

| Dimension | Description |
|-----------|-------------|
| Completion | Task finished? (0% / 50% / 100%) |
| Plan Quality | DAG decomposition reasonable? |
| Output Quality | Result accurate and usable? |
| Safety Mechanisms | Audit/snapshot/manifest working? |
| Duration | Total execution time |

---

## L1 — Single-Step Tasks (Baseline)

| # | Command | Validates |
|---|---------|-----------|
| L1.1 | `apex run "列出当前目录下所有 Go 文件的行数统计"` | Shell execution + result return |
| L1.2 | `apex run "写一个 Python 函数计算斐波那契数列，保存到 /tmp/apex-tests/fib.py"` | File creation + code quality |
| L1.3 | `apex run "分析 internal/dag/dag.go 的代码结构，给出中文总结"` | File reading + Chinese reasoning |

## L2 — Multi-Step DAG Tasks

| # | Command | Validates |
|---|---------|-----------|
| L2.1 | `apex run "先创建一个 /tmp/apex-tests/multi 目录，然后在里面创建 3 个文件 a.txt b.txt c.txt，最后写一个 index.txt 列出所有文件"` | DAG decomposition + dependency order |
| L2.2 | `apex run "读取 PROGRESS.md 的内容，统计完成了多少个 Phase，然后生成一个 JSON 格式的摘要保存到 /tmp/apex-tests/progress-summary.json"` | Multi-step reasoning + file I/O |
| L2.3 | `apex plan "设计一个简单的 Go HTTP 服务器，包含 /health 和 /echo 两个端点，写测试，然后运行测试"` | Plan quality only (dry-run) |

## L3 — Research & Analysis

| # | Command | Validates |
|---|---------|-----------|
| L3.1 | `apex run "调查 Go 1.25 相比 Go 1.24 有哪些重要的新特性，写一份中文调研报告保存到 /tmp/apex-tests/go125-report.md"` | Knowledge retrieval + report generation |
| L3.2 | `apex run "分析本项目 internal/ 下所有包的依赖关系，生成一个 Mermaid 格式的依赖图保存到 /tmp/apex-tests/deps.md"` | Large-scale code analysis + structured output |
| L3.3 | `apex run "对比 SQLite WAL 模式和 DELETE 模式的优缺点，结合本项目的使用场景给出选型建议，保存到 /tmp/apex-tests/sqlite-analysis.md"` | Technical depth + project context awareness |

## L4 — Code Generation Projects

| # | Command | Validates |
|---|---------|-----------|
| L4.1 | `apex run "在 /tmp/apex-tests/calc 目录创建一个 Go CLI 计算器程序，支持 add/sub/mul/div 四则运算，包含 main.go 和 calc_test.go，确保测试通过"` | Full project generation + runnable tests |
| L4.2 | `apex run "在 /tmp/apex-tests/todo 目录创建一个基于 SQLite 的 Go TODO 应用，支持 add/list/done/delete 命令，包含测试"` | Multi-file project + DB interaction |

## L5 — Boundary Challenges

| # | Command | Validates |
|---|---------|-----------|
| L5.1 | `apex run "重构本项目的 internal/dag 包，提取所有 magic number 为命名常量，确保所有测试通过"` | Real code modification (snapshot protected) |
| L5.2 | `apex run "为本项目编写一份完整的 API 文档，覆盖所有 cmd/apex/ 下的命令，保存到 /tmp/apex-tests/apex-api-docs.md"` | Wide code reading + long document generation |
| L5.3 | `apex run "分析本项目最近 20 个 commit 的代码变更模式，生成代码质量评估报告到 /tmp/apex-tests/code-quality.md"` | Git history analysis + comprehensive reasoning |

---

## Execution Protocol

1. Pre-flight: `apex doctor` to confirm system health
2. Execute L1→L5 sequentially
3. After each test: check manifest, verify output, record score
4. After each Level: summarize findings to `/tmp/apex-tests/results.md`
5. Stop condition: 2 consecutive 0% completions within a Level

## Safety Constraints

- **No git push** — no code leaves the local machine
- **No git commit** — project repo stays clean
- **All output to `/tmp/apex-tests/`** — no project pollution
- **L5.1 snapshot** — verify snapshot before execution, rollback immediately after
- **Kill switch** available via `apex kill-switch` at any time
