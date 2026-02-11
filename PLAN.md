# Memories - Implementation Plan

A Git-powered, file-based memory abstraction with semantic search for CLI usage.

## Project Overview

| Aspect | Choice                                  |
|--------|-----------------------------------------|
| Language | Go 1.25+                                |
| CLI | Cobra                                   |
| Git Backend | go-git/v5 (custom `.mem` storage)       |
| Vector Index | goannoy (Approximate Nearest Neighbors) |
| Local Embeddings | gollama.cpp (embeddinggemma-300m)       |
| LLM Provider | charmbracelet/fantasy                   |
| Config | YAML                                    |

## Directory Structure

```
memories/
├── cmd/
│   └── mem/
│       ├── main.go
│       ├── root.go
│       ├── init.go
│       ├── set.go
│       ├── get.go
│       ├── del.go
│       ├── list.go
│       ├── add.go
│       ├── commit.go
│       ├── status.go
│       ├── log.go
│       ├── diff.go
│       ├── branch.go
│       ├── search.go
│       ├── provider.go
│       ├── index.go
│       ├── summarize.go
│       ├── edit.go
│       ├── watch.go
│       └── external.go
│
├── pkg/
│   └── v1/
│       ├── client.go
│       ├── types.go
│       └── options.go
│
├── internal/
│   ├── memory.go
│   ├── branch.go
│   ├── scope.go
│   ├── vector.go
│   ├── llm.go
│   ├── service.go
│   ├── gogit.go
│   ├── annoy.go
│   ├── gollama.go
│   ├── fantasy.go
│   ├── config.go
│   ├── ignore.go
│   ├── download.go
│   ├── hardware.go
│   └── external.go          # mem-* command dispatch (git-style)
│
├── go.mod
├── go.sum
├── Makefile
├── LICENSE
└── README.md
```

---

## Phase 1: Project Bootstrap

### 1.1 Initialize Go Module
- [ ] Create `go.mod` with module path `github.com/4thel00z/memories`
- [ ] Set Go version to 1.25
- [ ] Add initial dependencies

**File: `go.mod`**
```go
module github.com/4thel00z/memories

go 1.25

require (
    github.com/spf13/cobra v1.8.1
    github.com/go-git/go-git/v5 v5.12.0
    github.com/go-git/go-billy/v5 v5.5.0
    github.com/mariotoffia/goannoy v0.1.0
    github.com/dianlight/gollama.cpp v0.5.0
    charm.land/fantasy v0.1.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### 1.2 Create Makefile
- [ ] Build targets for all platforms
- [ ] Install target
- [ ] Test target
- [ ] Lint target

**File: `Makefile`**
```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test lint clean

build:
	go build $(LDFLAGS) -o bin/mem ./cmd/mem

install:
	go install $(LDFLAGS) ./cmd/mem

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
```

### 1.3 Create Entry Point
- [ ] Create `cmd/mem/main.go` with basic structure
- [ ] Wire up dependency injection skeleton

---

## Phase 2: Domain Layer (internal/)

### 2.1 Memory Entity & Repository Port
**File: `internal/memory.go`**

- [ ] Define `Key` value object with validation
- [ ] Define `Memory` entity with fields: Key, Content, Metadata, CreatedAt, UpdatedAt
- [ ] Define `Metadata` struct: Tags, MimeType
- [ ] Define `MemoryRepository` interface:
  - `Get(ctx, key) (*Memory, error)`
  - `Save(ctx, *Memory) error`
  - `Delete(ctx, key) error`
  - `List(ctx, prefix) ([]*Memory, error)`
  - `Exists(ctx, key) (bool, error)`
- [ ] Define `ErrNotFound`, `ErrExists` sentinel errors

### 2.2 Branch Entity & Repository Port
**File: `internal/branch.go`**

- [ ] Define `Branch` entity: Name, Head (commit hash), CreatedAt
- [ ] Define `Commit` entity: Hash, Message, Author, Timestamp, Parents
- [ ] Define `BranchRepository` interface:
  - `Current(ctx) (*Branch, error)`
  - `List(ctx) ([]*Branch, error)`
  - `Create(ctx, name) (*Branch, error)`
  - `Switch(ctx, name) error`
  - `Delete(ctx, name) error`
- [ ] Define `HistoryRepository` interface:
  - `Commit(ctx, message) (*Commit, error)`
  - `Log(ctx, limit) ([]*Commit, error)`
  - `Diff(ctx, ref) (string, error)`
  - `Show(ctx, ref) (*Commit, error)`
  - `Revert(ctx, ref) error`

### 2.3 Scope Entity & Resolver
**File: `internal/scope.go`**

- [ ] Define `ScopeType` enum: `global`, `project`
- [ ] Define `Scope` struct: Type, Path, MemPath
- [ ] Helper methods: `VectorPath()`, `ConfigPath()`
- [ ] Define `ScopeResolver` struct
- [ ] Implement `Global() Scope`
- [ ] Implement `Project() (Scope, bool)`
- [ ] Implement `Resolve(explicit string) Scope`
- [ ] Implement `Cascade() []Scope` (project → global lookup order)
- [ ] Implement `EnvVars(scope, branch) map[string]string`

### 2.4 Vector Types & Index Port
**File: `internal/vector.go`**

- [ ] Define `Embedding` struct: Vector []float32, Dimension, Model
- [ ] Constructor `NewEmbedding(vec, model) Embedding`
- [ ] Define `SearchResult` struct: Key, Score (0-1)
- [ ] Define `VectorIndex` interface:
  - `Add(ctx, key, emb) error`
  - `Remove(ctx, key) error`
  - `Search(ctx, query, k) ([]SearchResult, error)`
  - `Build(ctx, numTrees) error`
  - `Save(ctx) error`
  - `Load(ctx) error`
  - `Contains(ctx, key) bool`

### 2.5 LLM Ports
**File: `internal/llm.go`**

- [ ] Define `Embedder` interface:
  - `Embed(ctx, text) ([]float32, error)`
  - `EmbedBatch(ctx, texts) ([][]float32, error)`
  - `Dimension() int`
  - `Device() string`
  - `Close() error`
- [ ] Define `Provider` interface:
  - `Complete(ctx, prompt) (string, error)`
  - `GenerateObject(ctx, prompt, target) error`
  - `Stream(ctx, prompt) (<-chan string, error)`
- [ ] Define structured output types:
  - `Summary`: Title, Overview, KeyPoints, Tags
  - `AutoTag`: Tags, Category, Confidence

---

## Phase 3: Infrastructure Layer (internal/)

### 3.1 Hardware Detection
**File: `internal/hardware.go`**

- [ ] Implement `detectHardware() string`
  - Return "mps" for Apple Silicon (darwin + arm64)
  - Return "cuda" for NVIDIA GPU (check /dev/nvidia0 or nvidia-smi)
  - Return "cpu" as fallback

### 3.2 Model Downloader
**File: `internal/download.go`**

- [ ] Define constants: ModelURL, ModelFilename, ModelSize
- [ ] Implement `ensureModel(cacheDir) (string, error)`
  - Check if model exists
  - Download with progress indicator if not
  - Use temp file + rename for atomic writes
- [ ] Implement `progressWriter` for download progress

### 3.3 Configuration
**File: `internal/config.go`**

- [ ] Define `Config` struct:
  - Embeddings: Backend, Model, Dimension
  - Providers: map[string]ProviderConfig
  - DefaultProvider: string
- [ ] Define `ProviderConfig`: APIKey, BaseURL, Model
- [ ] Implement `LoadConfig(scope) (*Config, error)`
- [ ] Implement `SaveConfig(scope, *Config) error`
- [ ] Default config location: `~/.mem/config.yaml` or `./.mem/config.yaml`

### 3.4 Ignore Matcher
**File: `internal/ignore.go`**

- [ ] Implement `.memignore` parser (gitignore-compatible patterns)
- [ ] Use `go-git/v5/plumbing/format/gitignore` for parsing
- [ ] Implement `IgnoreMatcher` struct with `MatchKey(key) bool`
- [ ] Block add/edit operations for keys matching ignore patterns

### 3.5 External Command Dispatch (git-style)
**File: `internal/external.go`**

Core extensibility mechanism - `mem foo` dispatches to `mem-foo` in PATH.

#### Dispatch Flow

```
mem foo arg1 arg2
     │
     ▼
┌─────────────────────────────────┐
│ 1. Is "foo" a built-in command? │
│    YES → execute built-in       │
│    NO  → continue               │
└─────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────┐
│ 2. Look for "mem-foo" in $PATH  │
│    FOUND → execute it           │
│    NOT FOUND → error            │
└─────────────────────────────────┘
```

#### Implementation Tasks

- [ ] Define constants:
  ```go
  const ExternalPrefix = "mem-"
  ```

- [ ] Implement `FindExternal(name string) (string, error)`
  - Use `exec.LookPath("mem-" + name)`
  - Return full path or error if not found

- [ ] Implement `ListExternalCommands() []string`
  - Scan all directories in $PATH
  - Find all executables matching `mem-*` pattern
  - Deduplicate (first in PATH wins)
  - Return command names without prefix

- [ ] Implement `ExecuteExternal(ctx, name, args, env) error`
  - Find binary via `FindExternal`
  - Build `exec.Cmd` with inherited environment + MEM_* vars
  - Connect stdin/stdout/stderr
  - Execute and wait
  - Return exit code as error if non-zero

- [ ] Implement `BuildExternalEnv(scope Scope, branch, version string) map[string]string`
  - Get current executable path via `os.Executable()`
  - Build complete environment map

#### Environment Variables for External Commands

| Variable | Description | Example |
|----------|-------------|---------|
| `MEM_SCOPE` | Scope type | `project` or `global` |
| `MEM_SCOPE_PATH` | Path to `.mem` directory | `/home/user/project/.mem` |
| `MEM_ROOT` | Working directory root | `/home/user/project` |
| `MEM_BRANCH` | Current branch name | `main` |
| `MEM_CONFIG` | Path to config.yaml | `/home/user/project/.mem/config.yaml` |
| `MEM_VERSION` | mem CLI version | `1.0.0` |
| `MEM_BIN` | Path to mem binary | `/usr/local/bin/mem` |

#### Reference Implementation

```go
// internal/external.go
package internal

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

const ExternalPrefix = "mem-"

// FindExternal looks for mem-<name> in PATH
func FindExternal(name string) (string, error) {
    binary := ExternalPrefix + name
    path, err := exec.LookPath(binary)
    if err != nil {
        return "", fmt.Errorf("unknown command %q: %s not found in PATH", name, binary)
    }
    return path, nil
}

// ListExternalCommands finds all mem-* executables in PATH
func ListExternalCommands() []string {
    var commands []string
    seen := make(map[string]bool)

    pathEnv := os.Getenv("PATH")
    for _, dir := range filepath.SplitList(pathEnv) {
        entries, err := os.ReadDir(dir)
        if err != nil {
            continue
        }
        for _, entry := range entries {
            if entry.IsDir() {
                continue
            }
            name := entry.Name()
            if !strings.HasPrefix(name, ExternalPrefix) {
                continue
            }
            // Check if executable
            path := filepath.Join(dir, name)
            info, err := os.Stat(path)
            if err != nil {
                continue
            }
            if info.Mode()&0111 == 0 {
                continue // not executable
            }
            cmdName := strings.TrimPrefix(name, ExternalPrefix)
            if !seen[cmdName] {
                seen[cmdName] = true
                commands = append(commands, cmdName)
            }
        }
    }
    return commands
}

// ExecuteExternal runs an external mem-* command
func ExecuteExternal(ctx context.Context, name string, args []string, env map[string]string) error {
    binaryPath, err := FindExternal(name)
    if err != nil {
        return err
    }

    cmd := exec.CommandContext(ctx, binaryPath, args...)

    // Inherit current environment
    cmd.Env = os.Environ()

    // Add MEM_* variables
    for k, v := range env {
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
    }

    // Connect stdio
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}

// BuildExternalEnv creates the MEM_* environment for external commands
func BuildExternalEnv(scope Scope, branch, version string) map[string]string {
    memBin, _ := os.Executable()

    return map[string]string{
        "MEM_SCOPE":      string(scope.Type),
        "MEM_SCOPE_PATH": scope.MemPath,
        "MEM_ROOT":       scope.Path,
        "MEM_BRANCH":     branch,
        "MEM_CONFIG":     scope.ConfigPath(),
        "MEM_VERSION":    version,
        "MEM_BIN":        memBin,
    }
}
```

#### Example External Commands

**Shell script: `mem-backup`**
```bash
#!/bin/bash
# /usr/local/bin/mem-backup
set -e
OUTPUT="${1:-memories-backup-$(date +%Y%m%d).tar.gz}"
echo "Backing up $MEM_SCOPE_PATH to $OUTPUT..."
tar -czf "$OUTPUT" -C "$(dirname "$MEM_SCOPE_PATH")" "$(basename "$MEM_SCOPE_PATH")"
echo "Done: $OUTPUT"
```

**Go binary: `mem-stats`**
```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
)

func main() {
    scopePath := os.Getenv("MEM_SCOPE_PATH")
    var count int
    var totalSize int64
    filepath.Walk(scopePath, func(path string, info os.FileInfo, err error) error {
        if err == nil && !info.IsDir() {
            count++
            totalSize += info.Size()
        }
        return nil
    })
    fmt.Printf("Scope: %s\nFiles: %d\nSize: %d bytes\n",
        os.Getenv("MEM_SCOPE"), count, totalSize)
}
```

#### Tests

- [ ] `internal/external_test.go`:
  - Test `FindExternal` with mock PATH
  - Test `ListExternalCommands` with temp directory containing mem-* scripts
  - Test `ExecuteExternal` with simple echo script
  - Test environment variables are passed correctly
  - Test exit code propagation

---

## Phase 4: Adapters - Storage (internal/)

### 4.1 Go-Git Storage Adapter
**File: `internal/gogit.go`**

- [ ] Define `GitRepository` struct
  - Fields: repo, worktree, rootPath, memPath
- [ ] Implement `NewGitRepository(scope) (*GitRepository, error)`
  - Use custom filesystem storage pointing to `.mem` instead of `.git`
  - Use `go-billy` for filesystem abstraction
- [ ] Implement `InitRepository(scope) error`
  - Create `.mem` directory structure
  - Initialize git repository with custom storage
  - Create initial commit
- [ ] Implement `MemoryRepository` interface:
  - `Get`: Read file from worktree
  - `Save`: Write file to worktree, stage it
  - `Delete`: Remove file, stage deletion
  - `List`: Walk worktree with prefix filter
  - `Exists`: Check file existence
- [ ] Implement `BranchRepository` interface:
  - `Current`: Get HEAD reference
  - `List`: List refs/heads/*
  - `Create`: Create new branch
  - `Switch`: Checkout branch
  - `Delete`: Delete branch ref
- [ ] Implement `HistoryRepository` interface:
  - `Commit`: Create commit with staged changes
  - `Log`: Iterate commit history
  - `Diff`: Show diff against ref
  - `Show`: Get commit details
  - `Revert`: Reset to ref

### 4.2 Custom Storage Backend
Within `internal/gogit.go`:

- [ ] Configure go-git to use `.mem` directory:
  ```go
  fs := osfs.New(memPath)
  storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
  wt := osfs.New(rootPath)
  repo, _ := git.Open(storage, wt)
  ```

---

## Phase 5: Adapters - Vector Index (internal/)

### 5.1 Annoy Index Adapter
**File: `internal/annoy.go`**

- [ ] Define `AnnoyIndex` struct:
  - Fields: mu (mutex), idx, dimension, keyToID, idToKey, nextID, paths, built
- [ ] Implement `NewAnnoyIndex(basePath, dimension) (*AnnoyIndex, error)`
  - Create vectors/ directory
  - Initialize Annoy with Angular distance
  - Use multi-worker policy and mmap allocator
- [ ] Implement `VectorIndex` interface:
  - `Add`: Assign ID to key, add vector to index
  - `Remove`: Delete from mappings (mark dirty for rebuild)
  - `Search`: Query k nearest neighbors, convert IDs to keys, compute scores
  - `Build`: Build index with n trees
  - `Save`: Persist index.ann + mapping.json
  - `Load`: Load index and mappings
  - `Contains`: Check key in mapping

---

## Phase 6: Adapters - Embeddings (internal/)

### 6.1 Local Embedder (gollama.cpp)
**File: `internal/gollama.go`**

- [ ] Define `LocalEmbedder` struct:
  - Fields: mu (mutex), model, ctx, dimension, device
- [ ] Implement `NewLocalEmbedder(cacheDir, dimension) (*LocalEmbedder, error)`
  - Ensure model is downloaded
  - Initialize gollama backend
  - Detect hardware
  - Load model with appropriate params
  - Create context with embeddings enabled
  - Set GPU layers based on device
- [ ] Implement `Embedder` interface:
  - `Embed`: Tokenize → Batch → Decode → Get embeddings → Normalize
  - `EmbedBatch`: Loop over texts (TODO: true batching later)
  - `Dimension`: Return configured dimension
  - `Device`: Return detected device string
  - `Close`: Free context, model, backend
- [ ] Implement L2 normalization helper

---

## Phase 7: Adapters - LLM Provider (internal/)

### 7.1 Fantasy Provider Adapter
**File: `internal/fantasy.go`**

- [ ] Define `FantasyProvider` struct:
  - Fields: model (fantasy.LanguageModel), name
- [ ] Define `FantasyConfig`: Provider, APIKey, BaseURL, Model
- [ ] Implement `NewFantasyProvider(ctx, cfg) (*FantasyProvider, error)`
  - Switch on provider type (openrouter, openai, anthropic, ollama)
  - Initialize appropriate fantasy provider
  - Get language model
- [ ] Implement `Provider` interface:
  - `Complete`: Create agent, call Generate
  - `GenerateObject`: Use schema.For(), call GenerateObject
  - `Stream`: Create agent, call Stream, return channel

---

## Phase 8: Application Services (internal/)

### 8.1 Memory Service
**File: `internal/service.go`**

- [ ] Define `MemoryService` struct:
  - Fields: resolver, repoFor, indexFor, embedder
- [ ] Implement `NewMemoryService(...) *MemoryService`
- [ ] Implement `Set(ctx, key, content, scope) error`
  - Validate key
  - Resolve scope
  - Save to repository
  - Generate embedding and index (if embedder available)
- [ ] Implement `Get(ctx, key, scope) (*Memory, error)`
  - Cascade through scopes if not explicit
- [ ] Implement `Delete(ctx, key, scope) error`
  - Delete from repo and index
- [ ] Implement `List(ctx, prefix, scope) ([]*Memory, error)`

### 8.2 History Service
Within `internal/service.go`:

- [ ] Define `HistoryService` struct
- [ ] Implement `Commit(ctx, message, scope) (*Commit, error)`
- [ ] Implement `Log(ctx, limit, scope) ([]*Commit, error)`
- [ ] Implement `Diff(ctx, ref, scope) (string, error)`
- [ ] Implement `Revert(ctx, ref, scope) error`

### 8.3 Branch Service
Within `internal/service.go`:

- [ ] Define `BranchService` struct
- [ ] Implement `Current(ctx, scope) (*Branch, error)`
- [ ] Implement `List(ctx, scope) ([]*Branch, error)`
- [ ] Implement `Create(ctx, name, scope) error`
- [ ] Implement `Switch(ctx, name, scope) error`
- [ ] Implement `Delete(ctx, name, scope) error`

### 8.4 Search Service
Within `internal/service.go`:

- [ ] Define `SearchService` struct
- [ ] Implement `Keyword(ctx, query, scope) ([]*Memory, error)`
  - Simple substring/regex match
- [ ] Implement `Semantic(ctx, query, k, scope) ([]SearchResult, error)`
  - Embed query
  - Search vector index
- [ ] Implement `RebuildIndex(ctx, scope) error`
  - List all memories
  - Batch embed
  - Add all to index
  - Build index

### 8.5 Summarize Service
Within `internal/service.go`:

- [ ] Define `SummarizeService` struct
- [ ] Implement `Summarize(ctx, prefix, scope) (*Summary, error)`
  - List memories
  - Build prompt
  - Call provider.GenerateObject
- [ ] Implement `AutoTag(ctx, key, scope) (*AutoTag, error)`

### 8.6 Provider Management Service
Within `internal/service.go`:

- [ ] Define `ProviderService` struct
- [ ] Implement `List() []string`
- [ ] Implement `Add(name, cfg) error`
- [ ] Implement `Remove(name) error`
- [ ] Implement `SetDefault(name) error`
- [ ] Implement `Test(ctx, name) error`

---

## Phase 9: CLI Commands (cmd/mem/)

### 9.1 Root Command
**File: `cmd/mem/root.go`**

- [ ] Create root command with:
  - Use: "mem"
  - Short description
  - Version flag
- [ ] Add persistent flags: `--scope`, `--branch`, `--json`
- [ ] Add all subcommands
- [ ] Set up external command fallback

### 9.2 Init Command
**File: `cmd/mem/init.go`**

- [ ] `mem init` - Initialize project scope (./.mem)
- [ ] `mem init --global` - Initialize global scope (~/.mem)
- [ ] Create directory structure
- [ ] Initialize git repository
- [ ] Create default config

### 9.3 Set Command
**File: `cmd/mem/set.go`**

- [ ] `mem set <key> <value>` - Create/update memory
- [ ] Flags: `--scope`, `--message`
- [ ] Auto-commit after set

### 9.4 Get Command
**File: `cmd/mem/get.go`**

- [ ] `mem get <key>` - Retrieve memory content
- [ ] Flags: `--scope`, `--json`
- [ ] Cascade through scopes

### 9.5 Del Command
**File: `cmd/mem/del.go`**

- [ ] `mem del <key>` - Delete memory
- [ ] Flags: `--scope`
- [ ] Auto-commit after delete

### 9.6 List Command
**File: `cmd/mem/list.go`**

- [ ] `mem list [prefix]` - List memories
- [ ] Flags: `--scope`, `--json`

### 9.7 Add Command
**File: `cmd/mem/add.go`**

- [ ] `mem add <key> [content]` - Append content to a memory
- [ ] Read from stdin if no content provided
- [ ] Creates historical node (git commit)
- [ ] Flags: `--scope`, `-m` for commit message

### 9.8 Commit Command
**File: `cmd/mem/commit.go`**

- [ ] `mem commit` - Commit staged changes
- [ ] Flags: `-m` for message
- [ ] Open $EDITOR if no message provided

### 9.9 Status Command
**File: `cmd/mem/status.go`**

- [ ] `mem status` - Show working tree status
- [ ] Show staged, unstaged, untracked

### 9.10 Log Command
**File: `cmd/mem/log.go`**

- [ ] `mem log` - Show commit history
- [ ] Flags: `-n` for limit, `--oneline`

### 9.11 Diff Command
**File: `cmd/mem/diff.go`**

- [ ] `mem diff [ref]` - Show changes
- [ ] Default: show unstaged changes

### 9.12 Branch Command
**File: `cmd/mem/branch.go`**

- [ ] `mem branch` - List branches
- [ ] `mem branch <name>` - Create and switch
- [ ] `mem branch -d <name>` - Delete

### 9.13 Search Command
**File: `cmd/mem/search.go`**

- [ ] `mem search <query>` - Keyword search
- [ ] `mem search --semantic <query>` - Semantic search
- [ ] Flags: `-n` for limit, `--json`

### 9.14 Provider Command
**File: `cmd/mem/provider.go`**

- [ ] `mem provider list` - List providers
- [ ] `mem provider add <name>` - Add provider (interactive)
- [ ] `mem provider remove <name>` - Remove provider
- [ ] `mem provider default <name>` - Set default
- [ ] `mem provider test [name]` - Test connectivity

### 9.15 Index Command
**File: `cmd/mem/index.go`**

- [ ] `mem index rebuild` - Rebuild semantic search index
- [ ] `mem index status` - Show index stats

### 9.16 Summarize Command
**File: `cmd/mem/summarize.go`**

- [ ] `mem summarize [prefix]` - Summarize memories
- [ ] Flags: `--provider`, `--json`

### 9.17 Edit Command
**File: `cmd/mem/edit.go`**

- [ ] `mem edit <key>` - Open in $EDITOR
- [ ] Create if doesn't exist
- [ ] Auto-commit on save

### 9.18 Watch Command
**File: `cmd/mem/watch.go`**

- [ ] `mem watch` - Watch for changes, auto-commit
- [ ] Flags: `--debounce` for batch window
- [ ] Use fsnotify

### 9.19 External Command Integration in Root
**Uses: `internal/external.go`**

Wire external command dispatch into the root command (implementation in Phase 3.5).

- [ ] Import external package from internal/
- [ ] Set `root.RunE` to handle unknown commands via `ExecuteExternal`
- [ ] Override help to list discovered `mem-*` commands in PATH:
  ```go
  root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
      cmd.Println(cmd.UsageString())

      externals := internal.ListExternalCommands()
      if len(externals) > 0 {
          cmd.Println("\nExternal commands (mem-*):")
          for _, name := range externals {
              cmd.Printf("  %s\n", name)
          }
      }
  })
  ```
- [ ] Propagate exit codes from external commands

### 9.20 Main Entry Point
**File: `cmd/mem/main.go`**

- [ ] Parse version from ldflags
- [ ] Initialize all dependencies
- [ ] Create services
- [ ] Build and execute CLI

---

## Phase 10: Public API (pkg/v1/)

### 10.1 Client
**File: `pkg/v1/client.go`**

- [ ] Define `Client` struct
- [ ] Implement `New(opts ...Option) (*Client, error)`
- [ ] Implement `Set(ctx, key, value) error`
- [ ] Implement `Get(ctx, key) ([]byte, error)`
- [ ] Implement `Delete(ctx, key) error`
- [ ] Implement `List(ctx, prefix) ([]Memory, error)`
- [ ] Implement `Search(ctx, query, limit) ([]SearchResult, error)`
- [ ] Implement `Close() error`

### 10.2 Types
**File: `pkg/v1/types.go`**

- [ ] Define `Memory` struct: Key, Content, Tags
- [ ] Define `SearchResult` struct: Key, Score
- [ ] Define `Commit` struct: Hash, Message, Timestamp

### 10.3 Options
**File: `pkg/v1/options.go`**

- [ ] Define `Option` type
- [ ] Implement `WithCacheDir(dir) Option`
- [ ] Implement `WithDimension(dim) Option`
- [ ] Implement `WithScope(scope) Option`

---

## Phase 11: Testing

### 11.1 Unit Tests
- [ ] `internal/memory_test.go` - Key validation, Memory creation
- [ ] `internal/scope_test.go` - Scope resolution
- [ ] `internal/annoy_test.go` - Index operations
- [ ] `internal/service_test.go` - Service logic with mocks

### 11.2 Integration Tests
- [ ] `internal/gogit_test.go` - Git operations with temp directories
- [ ] `internal/gollama_test.go` - Embedding generation (skip if no model)

### 11.3 E2E Tests
- [ ] `cmd/mem/e2e_test.go` - Full CLI workflow tests

---

## Phase 12: Documentation

### 12.1 README
- [ ] Project overview
- [ ] Installation instructions
- [ ] Quick start guide
- [ ] Command reference
- [ ] Configuration guide
- [ ] Extensibility (mem-* plugins)

### 12.2 Examples
- [ ] Example mem-summarize plugin (shell script)
- [ ] Example mem-export plugin (Go)
- [ ] Claude Code skill integration example

---

## Implementation Order

Execute phases in this order for incremental, testable progress:

```
Phase 1  → Project Bootstrap (foundation)
    ↓
Phase 2  → Domain Layer (types & interfaces)
    ↓
Phase 3.1-3.4 → Infrastructure (hardware, download, config, ignore)
    ↓
Phase 3.5 → External Command Dispatch (mem-* in PATH)
    ↓
Phase 4  → Storage Adapter (go-git)
    ↓
Phase 9.1 → Root Command (with external dispatch wired in)
    ↓
Phase 9.2-9.6 → Basic CLI (init, set, get, del, list)
    ↓
[MILESTONE: Basic memory CRUD + external dispatch working]
    ↓
Phase 9.7-9.12 → Git CLI (add, commit, status, log, diff, branch)
    ↓
[MILESTONE: Full git-like workflow]
    ↓
Phase 5  → Vector Index (annoy)
    ↓
Phase 6  → Local Embeddings (gollama)
    ↓
Phase 9.13 → Search Command
    ↓
[MILESTONE: Semantic search working]
    ↓
Phase 7  → LLM Provider (fantasy)
    ↓
Phase 8.5 → Summarize Service
    ↓
Phase 9.14-9.16 → Provider & Summarize CLI
    ↓
[MILESTONE: AI features working]
    ↓
Phase 9.17-9.18 → Edit, Watch
    ↓
Phase 10 → Public API (pkg/v1)
    ↓
Phase 11 → Testing
    ↓
Phase 12 → Documentation
    ↓
[RELEASE v1.0.0]
```

---

## Storage Layout

```
~/.mem/                              # Global scope
├── objects/                         # Git object storage
├── refs/
│   └── heads/
│       └── main                     # Default branch
├── HEAD                             # Current branch ref
├── index                            # Git staging area
├── config.yaml                      # Mem configuration
└── vectors/
    ├── index.ann                    # Annoy index
    └── mapping.json                 # Key ↔ ID mapping

~/.cache/mem/
└── models/
    └── embeddinggemma-300M-Q4_K_M.gguf

./.mem/                              # Project scope (same structure)
├── objects/
├── refs/
├── HEAD
├── index
├── config.yaml
└── vectors/
    ├── index.ann
    └── mapping.json

.memignore                           # Gitignore-style patterns
```

---

## CLI Reference (Final)

```bash
# Initialization
mem init                    # Init project scope
mem init --global           # Init global scope

# Memory CRUD
mem set <key> <value>       # Set memory (auto-commits)
mem get <key>               # Get memory
mem del <key>               # Delete memory
mem list [prefix]           # List memories
mem edit <key>              # Edit in $EDITOR

# Git-like operations
mem add <path>...           # Stage files
mem commit [-m "msg"]       # Commit staged
mem status                  # Show status
mem log [-n N]              # Show history
mem diff [ref]              # Show changes

# Branches
mem branch                  # List branches
mem branch <name>           # Create & switch
mem branch -d <name>        # Delete

# Search
mem search <query>          # Keyword search
mem search -s <query>       # Semantic search

# AI features
mem summarize [prefix]      # AI summary
mem provider list           # List LLM providers
mem provider add <name>     # Add provider
mem provider default <name> # Set default

# Index management
mem index rebuild           # Rebuild vector index
mem index status            # Index stats

# Watch mode
mem watch                   # Auto-commit on changes

# Global flags
--scope=<global|project>    # Target scope
--branch=<name>             # Target branch
--json                      # JSON output

# External commands (git-style dispatch)
mem <anything>              # Dispatches to mem-<anything> in $PATH
mem backup                  # → executes mem-backup
mem export --format=json    # → executes mem-export --format=json
```

---

## External Command Protocol

Any executable named `mem-*` in PATH becomes a subcommand:

```bash
# Install a plugin
cp my-plugin /usr/local/bin/mem-myplugin
chmod +x /usr/local/bin/mem-myplugin

# Use it
mem myplugin arg1 arg2
```

**Dispatch flow:**
```
mem foo arg1 arg2
  │
  ├─ Is "foo" built-in? → Execute built-in
  │
  └─ Look for "mem-foo" in $PATH → Execute with MEM_* env vars
```

**Environment passed to external commands:**
```bash
MEM_SCOPE=project           # or "global"
MEM_SCOPE_PATH=/path/.mem   # .mem directory
MEM_ROOT=/path              # working directory
MEM_BRANCH=main             # current branch
MEM_CONFIG=/path/.mem/config.yaml
MEM_VERSION=1.0.0
MEM_BIN=/usr/local/bin/mem
```

**Example plugin (shell):**
```bash
#!/bin/bash
# /usr/local/bin/mem-hello
echo "Hello from scope: $MEM_SCOPE"
echo "Branch: $MEM_BRANCH"
```

**Example plugin (Go):**
```go
package main

import (
    "fmt"
    "os"

    memclient "github.com/4thel00z/memories/pkg/v1"
)

func main() {
    client, _ := memclient.New()
    defer client.Close()

    memories, _ := client.List(context.Background(), "")
    fmt.Printf("Found %d memories in %s scope\n",
        len(memories), os.Getenv("MEM_SCOPE"))
}
```

---

## Environment Variables

```bash
# Passed to external commands (mem-*)
MEM_SCOPE=project|global
MEM_SCOPE_PATH=/path/to/.mem
MEM_ROOT=/path/to/working/dir
MEM_BRANCH=main

# Configuration overrides
MEM_LLM_PROVIDER=anthropic
MEM_LLM_MODEL=claude-sonnet-4-20250514
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
```

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| github.com/spf13/cobra | v1.8.1 | CLI framework |
| github.com/go-git/go-git/v5 | v5.12.0 | Git operations |
| github.com/go-git/go-billy/v5 | v5.5.0 | Filesystem abstraction |
| github.com/mariotoffia/goannoy | v0.1.0 | Vector index (ANN) |
| github.com/dianlight/gollama.cpp | v0.5.0 | Local embeddings |
| charm.land/fantasy | v0.1.0 | LLM provider |
| gopkg.in/yaml.v3 | v3.0.1 | Config parsing |
| github.com/fsnotify/fsnotify | v1.7.0 | File watching |