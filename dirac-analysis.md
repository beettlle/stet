# Dirac Project: Deep Technical Analysis

> **Repository**: https://github.com/dirac-run/dirac  
> **Website**: https://dirac.run  
> **License**: Apache 2.0  
> **Stars**: ~1,173 | **Created**: April 5, 2026  
> **Author**: Max Trivedi (Dirac Delta Labs)  
> **Origin**: Fork of [Cline](https://github.com/cline/cline)

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Complete Project Structure](#2-complete-project-structure)
3. [How Dirac Achieves Token Efficiency](#3-how-dirac-achieves-token-efficiency)
4. [How Dirac Achieves Accuracy](#4-how-dirac-achieves-accuracy)
5. [Prompt Engineering Strategies](#5-prompt-engineering-strategies)
6. [Context Management Strategies](#6-context-management-strategies)
7. [Novel Architectural Patterns](#7-novel-architectural-patterns)
8. [Detailed Core Algorithm Analysis](#8-detailed-core-algorithm-analysis)
9. [Benchmark Results](#9-benchmark-results)
10. [Key Takeaways](#10-key-takeaways)

---

## 1. Project Overview

Dirac is an open-source AI coding agent that claims — and benchmarks show — **64.8% lower API costs** compared to competing agents (Cline, Kilo, Ohmypi, Opencode, Pimono, Roo) while achieving **100% accuracy** (8/8 tasks) on complex real-world refactoring tasks. It topped the **Terminal-Bench-2 leaderboard** with a **65.2%** score using `gemini-3-flash-preview`.

The fundamental thesis behind Dirac is:

> "It is a well studied phenomenon that any given model's reasoning ability degrades with the context length. If we can keep context tightly curated, we improve both accuracy and cost while making larger changes tractable in a single task."

Dirac is available as:
- **VS Code Extension** (via VS Code Marketplace)
- **CLI** (`npm install -g dirac-cli`)

It supports all major LLM providers (Anthropic, OpenAI, Gemini, Groq, Mistral, x.ai, HuggingFace, AWS Bedrock, Google Vertex AI, and any OpenAI-compatible endpoint).

---

## 2. Complete Project Structure

```
dirac/
├── AGENTS.md                          # Agent guide for Dirac development
├── CONTRIBUTING.md                    # Contribution guidelines
├── README.md                          # Project documentation
├── LICENSE                            # Apache 2.0
├── biome.jsonc                        # Linter/formatter config
├── buf.yaml                           # Protocol Buffer config
├── .nvmrc                             # Node.js version (v22 LTS)
│
├── src/                               # Main VS Code extension source
│   ├── extension.ts                   # Extension entry point
│   │
│   ├── core/                          # Core business logic
│   │   ├── api/                       # LLM API providers
│   │   │   ├── index.ts               # API factory (createHandlerForProvider)
│   │   │   ├── providers/             # Provider implementations
│   │   │   │   ├── anthropic.ts
│   │   │   │   ├── gemini.ts
│   │   │   │   ├── openai.ts
│   │   │   │   ├── bedrock.ts
│   │   │   │   └── ...
│   │   │   └── transform/             # Stream transformers → internal ApiStream
│   │   │
│   │   ├── task/                      # Task execution loop (CRITICAL)
│   │   │   ├── index.ts               # Main task logic (72KB!)
│   │   │   ├── ApiConversationManager.ts
│   │   │   ├── ContextLoader.ts       # Context gathering
│   │   │   ├── ToolExecutor.ts        # Tool dispatch
│   │   │   ├── ResponseProcessor.ts   # LLM response processing
│   │   │   ├── StreamResponseHandler.ts
│   │   │   ├── StreamChunkCoordinator.ts
│   │   │   ├── LifecycleManager.ts    # Task lifecycle
│   │   │   ├── TaskMessenger.ts       # UI messaging
│   │   │   ├── TaskState.ts           # State management
│   │   │   ├── EnvironmentManager.ts
│   │   │   ├── HookManager.ts
│   │   │   ├── multifile-diff.ts      # Multi-file diff viewer
│   │   │   ├── message-state.ts
│   │   │   ├── tools/                 # Tool implementations (handlers)
│   │   │   │   └── utils/
│   │   │   │       └── SymbolContextResolver.ts
│   │   │   └── types/
│   │   │
│   │   ├── prompts/                   # Prompt engineering (CRITICAL)
│   │   │   ├── commands.ts            # Slash command prompts
│   │   │   ├── contextManagement.ts   # Context window management prompts
│   │   │   ├── responses.ts           # Response formatting
│   │   │   ├── tool-examples.ts       # Tool call examples
│   │   │   └── system-prompt/         # System prompt system
│   │   │       ├── index.ts           # getSystemPrompt() entry
│   │   │       ├── template.ts        # SYSTEM_PROMPT template
│   │   │       ├── spec.ts            # Tool spec → multi-provider conversion
│   │   │       ├── types.ts           # SystemPromptContext types
│   │   │       ├── constants.ts
│   │   │       ├── registry/          # Prompt registry system
│   │   │       │   ├── PromptRegistry.ts
│   │   │       │   ├── PromptBuilder.ts
│   │   │       │   └── DiracToolSet.ts
│   │   │       ├── sections/
│   │   │       │   └── editing-files.ts   # EDITING FILES instructions (CRITICAL)
│   │   │       ├── templates/
│   │   │       │   ├── TemplateEngine.ts
│   │   │       │   └── placeholders.ts
│   │   │       └── tools/             # Native tool specifications
│   │   │           ├── edit_file.ts        # Hash-anchored editing tool
│   │   │           ├── read_file.ts        # Multi-file read tool
│   │   │           ├── get_function.ts     # AST function extraction
│   │   │           ├── get_file_skeleton.ts# AST skeleton extraction
│   │   │           ├── replace_symbol.ts   # AST symbol replacement
│   │   │           ├── rename_symbol.ts    # Symbol renaming
│   │   │           ├── find_symbol_references.ts
│   │   │           ├── execute_command.ts  # Shell/script execution
│   │   │           ├── search_files.ts
│   │   │           ├── list_files.ts
│   │   │           ├── write_to_file.ts
│   │   │           ├── browser_action.ts
│   │   │           ├── diagnostics_scan.ts
│   │   │           ├── subagent.ts
│   │   │           ├── ask_followup_question.ts
│   │   │           ├── attempt_completion.ts
│   │   │           ├── plan_mode_respond.ts
│   │   │           └── ...
│   │   │
│   │   ├── context/                   # Context management (CRITICAL)
│   │   │   ├── context-management/
│   │   │   │   ├── ContextManager.ts      # Main context manager (14.7KB)
│   │   │   │   ├── context-error-handling.ts
│   │   │   │   └── context-window-utils.ts
│   │   │   ├── context-tracking/
│   │   │   │   ├── FileContextTracker.ts   # File staleness detection
│   │   │   │   ├── ModelContextTracker.ts  # Model context window tracking
│   │   │   │   ├── EnvironmentContextTracker.ts
│   │   │   │   └── ContextTrackerTypes.ts
│   │   │   └── instructions/              # Custom instruction loading
│   │   │
│   │   ├── controller/               # Extension coordination
│   │   ├── ignore/                    # DiracIgnoreController
│   │   ├── slash-commands/            # Slash command definitions
│   │   └── storage/                   # State persistence
│   │
│   ├── utils/                         # Utilities (CRITICAL)
│   │   ├── .hash_anchors              # 1,700+ single-token anchor words
│   │   ├── AnchorStateManager.ts      # Stateful anchor management + Myers Diff
│   │   ├── ASTAnchorBridge.ts         # AST ↔ Anchor integration
│   │   ├── line-hashing.ts            # Line hashing utilities
│   │   ├── cost.ts                    # Token cost calculation
│   │   ├── git.ts / git-worktree.ts   # Git integration
│   │   └── ...
│   │
│   ├── services/                      # Shared services
│   │   └── tree-sitter/               # Tree-sitter AST parsing
│   │       ├── index.ts               # parseFile() - skeleton extraction
│   │       └── languageParser.ts      # Multi-language parser loading
│   │
│   ├── integrations/                  # Terminal, Browser, Editor APIs
│   ├── shared/                        # Cross-component types
│   │   ├── tools.ts                   # DiracDefaultTool enum, tool registry
│   │   ├── api.ts                     # Model metadata, pricing
│   │   └── utils/
│   │       └── line-hashing.ts        # Shared line-hashing utilities
│   │
│   └── hosts/                         # Host abstraction (VSCode/CLI)
│
├── cli/                               # CLI application (TypeScript + Ink)
│   ├── src/
│   │   ├── agent/
│   │   │   ├── DiracAgent.ts          # CLI agent orchestrator (36KB)
│   │   │   ├── messageTranslator.ts   # Message format translation (24KB)
│   │   │   └── DiracSessionEmitter.ts
│   │   ├── acp/                       # Agent Communication Protocol
│   │   ├── commands/                  # CLI commands (auth, task, history, etc.)
│   │   └── components/               # Ink React components for CLI UI
│   ├── package.json
│   └── esbuild.mts                    # CLI build config
│
├── webview-ui/                        # React-based VS Code webview
├── agent-registry/                    # Agent registry metadata
├── evals/                             # Benchmark evaluation results
├── docs/                              # Documentation
│   └── providers/
│       └── README.md                  # Provider configuration guide
└── proto/
    └── dirac/                         # Protocol Buffer definitions
```

---

## 3. How Dirac Achieves Token Efficiency

Dirac employs **six primary techniques** to achieve its 64.8% cost reduction:

### 3.1 Single-Token Hash Anchors (Primary Innovation)

**The Problem**: Traditional agents (Claude Code, Gemini CLI, Codex) use search-and-replace for code edits, requiring `O(S+R)` output tokens where S = search block size and R = replacement size. The model must repeat the entire old code verbatim to locate the edit target.

**Dirac's Solution**: Hash-anchored edits reduce output to `O(R)` — only the replacement text is needed.

**How it works**:
1. Every line read by the LLM is prefixed with a **single-token anchor word** + `§` delimiter
2. The LLM references lines by their anchor (`Apple§ def process(data):`) instead of reproducing the entire search block
3. ~1,700 curated single-token words (from the `o200k_base` tiktoken encoding) are shipped as an asset in `src/utils/.hash_anchors`
4. Each anchor costs only **2 tokens** per line overhead (1 for the word, 1 for `§`)

**Example read output**:
```
Apple§ def process(param1, param2):
Brave§     total = 0
Cider§     for item in items:
Delta§         if item.price > 0:
Eagle§             total += item.price
Fox§     return total
```

**Example edit (instead of full search-and-replace)**:
```json
{
  "edit_type": "replace",
  "anchor": "Brave§ total = 0",
  "end_anchor": "Eagle§     total += item.price",
  "text": "    total = sum(item.price for item in items if item.price > 0)"
}
```

The savings are particularly dramatic for deletions (where traditional agents must output the entire deleted block as the "search" term).

### 3.2 Multi-File Batching

The `edit_file` tool accepts a `files` array parameter, allowing the LLM to edit multiple files in a **single tool call**. The `read_file`, `get_function`, `get_file_skeleton`, `search_files`, `list_files`, and `execute_command` tools all accept array parameters for batch operations.

```json
{
  "files": [
    { "path": "src/a.ts", "edits": [...] },
    { "path": "src/b.py", "edits": [...] }
  ]
}
```

This dramatically reduces LLM round-trips. The system prompt explicitly says:

> "MINIMIZE THE NUMBER OF ROUND TRIPS NEEDED TO DO THIS. BATCH TOOL CALLS TOGETHER TO AVOID MULTIPLE ROUND TRIPS."

And for parallel tool calling:

> "When refactoring a single file, multiple edits to different sections of the file are considered INDEPENDENT operations because we have stable hash anchors."

### 3.3 AST-Based Surgical Context Loading

Instead of reading entire files, Dirac provides three granularity levels:

1. **`get_file_skeleton`** — Returns only the structural outline (class/function/method definition lines), stripping all implementation. Uses Tree-sitter AST parsing. Optionally includes call graphs (line counts and cross-references).

2. **`get_function`** — Extracts only the specified function(s) by name from file(s), returning their complete implementation with anchors. Supports dot-separated paths (`ClassName.methodName`).

3. **`read_file`** with line ranges — Supports `start_line`/`end_line` for partial reads.

The `read_file` tool description itself says: *"Consider using surgical tools like get_file_skeleton or get_function over this."*

### 3.4 Stateful Anchor Management with Myers Diff Reconciliation

The `AnchorStateManager` is the heart of the system. Key design decisions:

- **Stateful, not stateless**: Anchors are tracked per-file, per-task in memory. This is a deliberate departure from the original hash-anchor concept (which was stateless).
- **Myers Diff reconciliation**: When a file changes (via edit or external modification), the system uses the `diff` library's `diffArrays` on FNV-1a hash arrays to identify unchanged lines. Unchanged lines **keep their anchors**; only new lines get new anchors.
- **No invalidation cascade**: Unlike line-number-based approaches, editing line 1 does NOT invalidate all subsequent anchors. Only actually-changed lines get new anchors.
- **LRU eviction**: Tracks up to 1,024 files per task and 50 tasks simultaneously with LRU eviction.
- **Pool exhaustion handling**: When single-word anchors (1,700+) are exhausted for a file, the system generates random 2-word combinations; if those run out, 3-word combinations.

### 3.5 AST-Native Symbol Operations

The `replace_symbol` tool targets AST nodes directly. Instead of using anchor-based editing for function replacements, this tool:
- Locates symbols by dot-separated path (e.g., `ClassName.methodName`)
- Includes associated JSDoc, decorators, and export keywords in the replacement range
- Walks the AST parent chain to resolve full qualified names

The `rename_symbol` tool propagates renames across entire directories without requiring the LLM to enumerate every reference.

### 3.6 Command/Script Tool for Bulk Updates

The `execute_command` tool accepts both `commands` (array of shell commands) and `script` (multi-line scripts in bash/python/node). The system prompt explicitly recommends:

> "Use `execute_command` with commands like grep/awk/sed/find etc for bulk updates. CHEAPEST to execute and very useful for updating files in bulk. You can update the files without necessarily reading them."

---

## 4. How Dirac Achieves Accuracy

### 4.1 Context Curation Over Bloat

Dirac's core thesis is that keeping context tight improves model reasoning. This is implemented through:

- Encouraging `get_file_skeleton` → `get_function` → `edit_file` workflow instead of reading entire files
- The `PRIME DIRECTIVES` in the system prompt: "LOAD INTO CONTEXT ONLY WHAT IS NECESSARY"
- "ONCE YOU READ A FILE OR A FUNCTION, DO NOT TRY TO READ IT AGAIN, ASSUME THAT IT HASN'T CHANGED SINCE YOUR LAST READ UNLESS YOU CHANGED IT"

### 4.2 File Staleness Detection

The `FileContextTracker` uses `chokidar` file watchers to detect when files are modified outside of Dirac. When a user edits a file externally, Dirac is informed that it needs to re-read before making changes. This prevents stale-context bugs where the LLM tries to edit a file based on outdated information.

### 4.3 Anchor Validation

The `edit_file` tool validates anchors by doing a full-line string match. The LLM must provide the complete anchored line (`Apple§ def process(data):`) — not just the anchor word. If the anchor doesn't match the current file state, the tool returns an error, forcing the LLM to re-read the file.

### 4.4 AST-Guaranteed Structural Correctness

The `replace_symbol` tool uses Tree-sitter to locate the exact byte range of a symbol's definition (including decorators, comments, export wrappers). It walks up the AST to include `export_statement`, `decorated_definition`, etc., ensuring structurally correct replacements.

### 4.5 Detailed Editing Instructions with Examples

The `editing-files.ts` section provides comprehensive instructions to the LLM with explicit examples showing:
- How anchors work (before/after)
- How to handle multi-file batching
- How to handle single-line replacements, deletions, and insertions
- Common error patterns ("THE MOST COMMON error type is not balancing braces/indents")

### 4.6 Native Tool Calling Only (No MCP)

Dirac explicitly avoids MCP (Model Context Protocol) in favor of native tool calling. This ensures maximum reliability by using each provider's first-party tool-calling mechanism rather than text-based tool parsing.

---

## 5. Prompt Engineering Strategies

### 5.1 System Prompt Architecture

The system prompt uses a **registry-based modular architecture**:

```
PromptRegistry → PromptBuilder → TemplateEngine → sections + tools → final prompt
```

- `PromptRegistry` is a singleton that assembles the full system prompt
- `DiracToolSet` manages which tools are available based on context (mode, capabilities)
- Tool specs are defined per-tool in `src/core/prompts/system-prompt/tools/`
- Sections like `editing-files.ts` are assembled into the prompt
- Template placeholders (`{{OS}}`, `{{SHELL}}`, `{{CWD}}`, `{{AVAILABLE_CORES}}`, etc.) are filled at runtime

### 5.2 Prime Directives (Minimalist Philosophy)

The system prompt has three prime directives:

```
PRIME DIRECTIVES

1. ACCOMPLISH THE TASK HUMAN GIVES YOU.
2. MINIMIZE THE NUMBER OF ROUND TRIPS NEEDED TO DO THIS. BATCH TOOL CALLS TOGETHER...
3. LOAD INTO CONTEXT ONLY WHAT IS NECESSARY.
```

This is deliberately "bare minimum prompting" — the project philosophy is:

> "Optimize for bang-for-the-buck on tooling with bare minimum prompting instead of going blindly minimalistic."

### 5.3 Tool Description Design

Tool descriptions serve double duty as both API documentation and behavioral guidance:

- `read_file`: "Consider using surgical tools like get_file_skeleton or get_function over this."
- `edit_file`: Contains detailed anchor rules, batching rules, and multi-file examples inline in the description
- `execute_command`: "CHEAPEST to execute and very useful for updating files in bulk"
- `replace_symbol`: "More robust and token-efficient than edit_file because it targets specific AST nodes directly"

### 5.4 Multi-Provider Tool Spec System

The `spec.ts` file contains a sophisticated tool spec converter:

- `DiracToolSpec` → OpenAI `ChatCompletionTool` (with optional strict mode)
- `DiracToolSpec` → Anthropic `Tool` (input_schema format)
- `DiracToolSpec` → Google Gemini `FunctionDeclaration`
- `DiracToolSpec` → OpenAI Response API format

This means tools are defined once and automatically adapted to each provider's expected schema.

### 5.5 Context Condensation

When approaching context limits, Dirac has two mechanisms:

1. **`summarize_task`** — Creates an exhaustive summary capturing all progress, then continues with just the summary as context
2. **`condense`** — Similar but maintains the same conversation thread

The `contextManagement.ts` prompt instructs: "Your summary must be exhaustive, capturing the 'whole nine yards'"

### 5.6 Custom Instructions Integration

Dirac loads custom instructions from multiple sources:
- `.diracrules/` directory
- `.cursorrules` file
- `.cursor/rules/` directory
- `.windsurfrules` file
- `AGENTS.md` / `.agents/` / `.ai/` / `.claude/` directories

These are appended to the system prompt under `# USER'S CUSTOM INSTRUCTIONS`.

---

## 6. Context Management Strategies

### 6.1 FileContextTracker (Staleness Detection)

Each file that Dirac reads or edits gets a `chokidar` file watcher. The tracker maintains:
- `dirac_read_date`: When Dirac last read the file
- `dirac_edit_date`: When Dirac last edited the file
- `user_edit_date`: When the user last edited the file externally
- `record_state`: "active" or "stale"

When a user modifies a file externally, the watcher fires and marks it in `recentlyModifiedFiles`. Before the next tool call, Dirac is informed that the file's context may be stale.

### 6.2 ModelContextTracker

Tracks the model's context window usage to know when condensation/summarization is needed.

### 6.3 EnvironmentContextTracker

Tracks environment context (OS, shell, cwd) for the system prompt.

### 6.4 Task Summarization for Context Window Management

When the context window fills up, the system forces the LLM to call `summarize_task`, creating a comprehensive summary that becomes the starting context for a new conversation continuation.

### 6.5 Plan Mode vs Act Mode

- **Plan Mode**: Read-only tools + `plan_mode_respond`. Goal is to gather information and present a plan.
- **Act Mode**: Full tool access. Execute the plan.

This separation prevents the LLM from making premature changes before understanding the problem.

---

## 7. Novel Architectural Patterns

### 7.1 Single-Token Anchor Dictionary

The `.hash_anchors` file contains ~1,700 carefully curated English words, each of which tokenizes to exactly **one token** in the `o200k_base` encoding. This was generated by iteratively filtering the tiktoken vocabulary for single-token, single-word, human-readable anchors.

This means each line's anchor overhead is exactly **2 tokens**: one for the anchor word and one for the `§` delimiter. Compare this to the original hash-anchor approach (`101:x9|`) which costs 4-5 tokens per line.

### 7.2 FNV-1a Hash-Based Myers Diff

Instead of running text-based diff on file contents (which would be expensive for large files), the `AnchorStateManager` first computes FNV-1a hashes for each line into a `Uint32Array`, then runs Myers Diff on the integer arrays. This is significantly faster and more memory-efficient.

```typescript
private static computeHashes(lines: string[]): Uint32Array {
    const hashes = new Uint32Array(lines.length)
    for (let i = 0; i < lines.length; i++) {
        let h = 2166136261 // FNV-1a offset basis
        for (let j = 0; j < line.length; j++) {
            h = Math.imul(h ^ line.charCodeAt(j), 16777619) // FNV-1a prime
        }
        hashes[i] = h >>> 0
    }
    return hashes
}
```

### 7.3 AST-Anchor Bridge

The `ASTAnchorBridge` class unifies Tree-sitter AST parsing with the anchor state system:

- `getFileSkeleton()`: Parses AST for definitions, then overlays anchors from `AnchorStateManager`
- `getFunctions()`: Locates functions by name via AST, extracts their byte range, then maps to anchored lines
- `getSymbolRange()`: Finds the exact byte range of a symbol for `replace_symbol`, walking up the AST for decorators/exports

This means the AST tools (`get_function`, `get_file_skeleton`, `replace_symbol`) all integrate with the same anchor state, so anchors returned by `get_file_skeleton` can be used directly in `edit_file` calls.

### 7.4 SymbolContextResolver

When extracting a function via `get_function`, Dirac includes **context** above the function — imports, type definitions, or other symbols that the function depends on. This is done via `SymbolContextResolver.resolve()`, which analyzes the AST to find relevant context.

### 7.5 Function Hash for Change Detection

Each extracted function gets a content hash (`contentHash(defText)`), enabling the LLM to detect whether a function has changed since it was last read without re-reading the entire file:

```
src/main.ts::processData
[Function Hash: a1b2c3d4]
```

### 7.6 Parallel Independent Edits

Because hash anchors are stable across edits (unchanged lines keep their anchors), Dirac declares that multiple edits to different sections of the same file are **independent operations**. This enables the LLM to batch them in parallel within a single response, even though they target the same file.

### 7.7 No MCP — Native Tool Calling

Dirac exclusively uses native function/tool calling APIs from each provider. No text-based tool parsing, no MCP protocol overhead. This is a deliberate architectural decision for reliability and performance.

---

## 8. Detailed Core Algorithm Analysis

### 8.1 Anchor Reconciliation Algorithm (AnchorStateManager.reconcile)

```
Input: absolutePath, currentLines[], taskId
Output: anchors[] (one per line)

1. Compute FNV-1a hashes for all currentLines → currentHashes (Uint32Array)
2. Look up tracked state for this file+task

3. FAST PATH: If hashes are identical to cached → return cached anchors

4. FIRST TIME: If no prior state:
   a. Shuffle the 1,700-word dictionary
   b. Assign one unique word per line
   c. Store {hashes, anchors, usedWords, availablePool}
   d. Return anchors

5. RECONCILIATION (file changed since last seen):
   a. Run diff.diffArrays(oldHashes, currentHashes) → change list
   b. For each change:
      - "added": Assign NEW unique words from pool
      - "removed": Skip (advance old index)
      - "unchanged": CARRY OVER exact same anchor word
   c. Update stored state with new anchors
   d. Return new anchors
```

The key insight: by running Myers Diff on integer hash arrays instead of strings, the reconciliation is O(N+D) where D is the number of differences, and uses minimal memory.

### 8.2 AST Skeleton Extraction (ASTAnchorBridge.getFileSkeleton)

```
Input: absolutePath, diracIgnoreController, taskId, options
Output: Formatted skeleton string or null

1. Load language-specific Tree-sitter parser for the file extension
2. Parse file content into AST
3. Run Tree-sitter query to find all definition captures:
   - "name.definition.function"
   - "name.definition.method"
   - "name.definition.class"
   - "name.definition.interface"
   - etc.
4. Optionally collect call graph:
   - Track definedNames (functions/methods defined in this file)
   - Track allReferences (function calls within definition bodies)
   - Map each function → Set of local functions it calls
5. Reconcile file with AnchorStateManager to get anchors
6. Format output: "│{anchor}§{definition line}" with "│----" separators
7. Optionally append "# Lines: N" and "# Calls: [fn1, fn2]" metadata
```

### 8.3 Function Extraction (ASTAnchorBridge.getFunctions)

```
Input: absolutePath, relPath, functionNames[], diracIgnoreController, taskId
Output: {formattedContent, foundNames[]}

1. Parse file with Tree-sitter
2. Build nodeToMatch map for all definition captures
3. For each match with a name.definition capture:
   a. Walk up AST parent chain to compute fully-qualified name
   b. Check if fullName matches any requested functionNames
   c. If match:
      i.  Get extended range (include export_statement, decorators, comments)
      ii. Map to anchored lines from AnchorStateManager
      iii. Resolve context via SymbolContextResolver
      iv. Compute content hash
      v.  Format: "path::fullName\n[Function Hash: ...]\n{context}\n{anchored lines}"
```

### 8.4 Edit Application

The `edit_file` handler processes edits as follows:

```
Input: files[{path, edits[{edit_type, anchor, end_anchor, text}]}]

For each file:
  1. Parse anchor strings to extract anchor word + content
  2. Validate anchors exist in the file's anchor state
  3. Validate the content portion matches the actual file line
  4. Sort edits by line position (bottom-up to prevent offset shifts)
  5. Apply each edit:
     - replace: Replace lines from anchor to end_anchor (inclusive) with text
     - insert_after: Insert text after anchor line
     - insert_before: Insert text before anchor line
  6. Write modified file
  7. Reconcile anchors via AnchorStateManager (new lines get new anchors)
  8. Return updated anchors to LLM for subsequent edits
```

### 8.5 Tool Spec Multi-Provider Conversion

```
DiracToolSpec {
  id, name, description, instruction?,
  contextRequirements?, parameters[]
}

→ toolSpecFunctionDefinition()  → OpenAI ChatCompletionTool
→ toolSpecInputSchema()         → Anthropic Tool
→ toolSpecFunctionDeclarations() → Google Gemini FunctionDeclaration
→ toOpenAIResponseTools()       → OpenAI Response API format
→ openAIToolToAnthropic()       → OpenAI → Anthropic converter
```

Each converter handles provider-specific quirks:
- OpenAI strict mode: recursively sets `additionalProperties: false`
- Google: maps JSON Schema types to `GoogleToolParamType` enums
- Anthropic: uses `input_schema` format

---

## 9. Benchmark Results

All tasks used `gemini-3-flash-preview` with thinking set to `high`.

| Task | Files | Cline | Kilo | Ohmypi | Opencode | Pimono | Roo | **Dirac** |
|------|-------|-------|------|--------|----------|--------|-----|-----------|
| DynamicCache (transformers) | 8 | $0.37 | N/A | $0.24 | $0.20 | $0.34 | $0.49 | **$0.13** |
| IOverlayWidget (vscode) | 21 | $0.67 | $0.78 | $0.63 | $0.40 | $0.48 | $0.58 | **$0.23** |
| addLogging (vscode) | 12 | $0.42 | $0.70 | $0.64 | $0.32 | $0.25 | $0.45 | **$0.16** |
| datadict (django) | 14 | $0.36 | $0.42 | $0.32 | $0.24 | $0.24 | $0.17 | **$0.08** |
| extensionswb_service (vscode) | 3 | N/A | $0.71 | $0.43 | $0.53 | $0.50 | $0.36 | **$0.17** |
| latency (transformers) | 25 | $0.87 | $1.51 | $0.94 | $0.90 | $0.52 | $1.44 | **$0.34** |
| sendRequest (vscode) | 13 | $0.51 | $0.77 | $0.74 | $0.67 | $0.45 | $1.05 | **$0.25** |
| stoppingcriteria (transformers) | 3 | $0.25 | $0.19 | $0.17 | $0.26 | $0.23 | $0.29 | **$0.12** |
| **Total Correct** | | 5/8 | 5/8 | 6/8 | 8/8 | 6/8 | 6/8 | **8/8** |
| **Avg Cost** | | $0.49 | $0.73 | $0.51 | $0.44 | $0.38 | $0.60 | **$0.18** |

**Key observations**:
- Dirac achieves **100% accuracy** (8/8) while being the cheapest
- Cost reduction ranges from 30% (vs Pimono on task 3) to **86%** (vs Roo on task 6)
- The savings scale with task complexity — the 25-file `latency` task shows the greatest absolute savings
- Only Dirac and Opencode achieved 8/8 accuracy; Dirac did so at 59% lower cost

---

## 10. Key Takeaways

### What Makes Dirac Novel

1. **Single-token anchors** — The use of tiktoken-curated single-word anchors instead of line-number:hash pairs reduces per-line overhead from 4-5 tokens to 2 tokens.

2. **Stateful anchor management** — Breaking from stateless designs to maintain per-file anchor maps, enabling stable references across edits without full-file re-reads.

3. **FNV-1a + Myers Diff for reconciliation** — Using integer-hash-based diffing for anchor reconciliation is far more efficient than text-based approaches.

4. **AST-anchor unification** — The `ASTAnchorBridge` seamlessly bridges structural code understanding (Tree-sitter) with the hash-anchor editing system, so `get_file_skeleton` output anchors can be used directly in `edit_file`.

5. **Multi-file batching across all tools** — Not just editing, but reading, searching, and skeleton extraction all support array parameters for batch operations.

6. **Hierarchical context loading** — The skeleton → function → full-file pipeline gives the LLM the right granularity at each step.

7. **No MCP** — Deliberately avoiding the overhead and fragility of Model Context Protocol in favor of native tool calling.

### Design Philosophy

Dirac's approach can be summarized as: **"Give the LLM powerful, precise tools and minimal but actionable context."** Rather than dumping entire files into context and hoping the model figures it out, Dirac provides:
- Surgical tools that operate at the right granularity
- Anchors that eliminate redundant output tokens
- Batch operations that minimize round-trips
- AST understanding that ensures structural correctness

The result is a system where both cost AND accuracy improve simultaneously — not the typical tradeoff — because tighter context genuinely helps LLM reasoning.
