# Phase 4 Context Builder Design

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | MVP — token budget + compression + memory integration | No Paging, no Artifact Registry dependency |
| Compression | Pure rule-based, no LLM calls | Zero extra API cost, deterministic, testable |
| Token estimation | `len([]rune(text)) / 3` | Simple approximation for mixed CJK/Latin text |
| Summarizable fallback | Extractive (headings + first paragraph + key lines) | No LLM needed, good enough for MVP |

## Architecture

```
Task (DAG Node)
    |
    v
+-----------------------------+
|      Context Builder        |
|                             |
|  1. Token Budget (60k)      |
|  2. Memory Retrieval        |<-- search.Engine (Phase 3)
|  3. File Collector          |<-- filesystem direct read
|  4. Compression Pipeline    |
|     exact -> structural ->  |
|     summarizable -> ref     |
|  5. Prompt Assembly         |
+-------------+---------------+
              |
              v
        claude --print
```

## Core Data Structure

```go
type ContentBlock struct {
    ID       string            // e.g. "memory:facts/redis.md"
    Source   string            // "memory" | "file" | "task"
    Path     string            // file path (relative)
    RawText  string            // original content
    Policy   CompressionPolicy // exact|structural|summarizable|reference
    Priority int               // higher = more important, last to degrade
    Tokens   int               // estimated token count
}
```

## Compression Strategies

| Policy | Behavior | Typical Source |
|--------|----------|----------------|
| exact | Keep full text | Task description, API schema |
| structural | Keep function signatures/structure skeleton, truncate bodies | .go/.py/.js code files |
| summarizable | Extract headings + first paragraph + key lines (rule-based) | .md docs, reports |
| reference | Keep path + size + first line only | Large files, low priority |

## Compression Pipeline

1. Collect all ContentBlocks, apply default compression per Policy
2. Calculate total tokens; if within budget, done
3. If over budget, degrade lowest Priority blocks first:
   - summarizable -> reference
   - structural -> summarizable -> reference
4. exact blocks never degrade; if exact alone exceeds budget, return error

## Priority Defaults

| Source | Default Priority |
|--------|-----------------|
| Task description | 100 |
| Memory results | 80 |
| Code files | 60 |
| Documentation | 40 |

## Integration Point

`cmd/apex/run.go` executeDAGNode() calls `contextbuilder.Build()` before executor, replacing current raw prompt concatenation.

## Error Handling

- Memory search fails -> skip memory context, use files + task only (graceful degradation)
- File read fails -> skip that file, log warning
- All exact blocks exceed budget -> return error

## Config

New `ContextConfig` in config.yaml:

```yaml
context:
  token_budget: 60000
```

## Deliverables

| File | Description |
|------|-------------|
| `internal/context/builder.go` | Context Builder core: Build() function |
| `internal/context/compress.go` | Four-tier compression implementations |
| `internal/context/token.go` | Token estimation utility |
| `internal/context/builder_test.go` | Unit tests (~8-10) |
| `internal/config/config.go` | Add ContextConfig struct |
| `cmd/apex/run.go` | Integrate Context Builder into execution |

## Testing

- Each compression strategy tested independently
- Budget overflow + degradation logic tested
- Memory integration tested with mock embedder
- Graceful degradation (missing memory/files) tested
