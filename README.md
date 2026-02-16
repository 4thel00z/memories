<div align="center">

# mem

**Git-powered, file-based memory store with semantic search.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/4thel00z/memories/actions/workflows/ci.yml/badge.svg)](https://github.com/4thel00z/memories/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/4thel00z/memories)](https://goreportcard.com/report/github.com/4thel00z/memories)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Store key-value memories in a Git repository, with branching, history, diffing,
and commit semantics. Optionally add vector-based semantic search via local embeddings.

[Install](#install) · [Quick Start](#quick-start) · [Commands](#commands) · [Public API](#public-api) · [Plugins](#extensibility)

</div>

---

## Install

```bash
go install github.com/4thel00z/memories/cmd/mem@latest
```

Or build from source:

```bash
git clone https://github.com/4thel00z/memories.git
cd memories
make install
```

## Quick Start

```bash
# Initialize a project-scoped memory store
mem init

# Store some memories
mem set project/name "my-project"
mem set project/lang "go"
mem set notes/todo "write documentation"

# Retrieve
mem get project/name
# → my-project

# List all
mem list
# → notes/todo
# → project/lang
# → project/name

# List with prefix filter
mem list project
# → project/lang
# → project/name

# Search by keyword
mem search documentation
# → notes/todo

# Delete
mem del notes/todo

# View history
mem log --oneline

# Branch
mem branch experiments
mem set experiment/idea "try annoy trees=20"
mem branch main
```

## Commands

### Memory CRUD

| Command | Description |
|---------|-------------|
| `mem set <key> <value>` | Create or update a memory (auto-commits) |
| `mem get <key>` | Retrieve a memory's content |
| `mem del <key>` | Delete a memory (auto-commits) |
| `mem list [prefix]` | List memories, optionally filtered by prefix |
| `mem add <key> [content]` | Append content to a memory (reads stdin if no content) |
| `mem edit <key>` | Open a memory in `$EDITOR` (auto-commits on save) |

### Git-like Operations

| Command | Description |
|---------|-------------|
| `mem commit [-m "msg"]` | Commit staged changes (opens `$EDITOR` if no `-m`) |
| `mem status` | Show current branch |
| `mem log [-n N] [--oneline]` | Show commit history |
| `mem diff [ref]` | Show uncommitted changes |

### Branches

| Command | Description |
|---------|-------------|
| `mem branch` | List branches |
| `mem branch <name>` | Create and switch to a new branch |
| `mem branch -d <name>` | Delete a branch |

### Search

| Command | Description |
|---------|-------------|
| `mem search <query>` | Keyword search (content + key matching) |
| `mem search -s <query>` | Semantic search (requires embedder) |

### AI Features

| Command | Description |
|---------|-------------|
| `mem summarize [prefix]` | AI-powered summarization (requires provider) |
| `mem provider list` | List configured LLM providers |
| `mem provider add <name>` | Add an LLM provider |
| `mem provider remove <name>` | Remove a provider |
| `mem provider default <name>` | Set the default provider |

### Index Management

| Command | Description |
|---------|-------------|
| `mem index rebuild` | Rebuild the vector search index |
| `mem index status` | Show index statistics |

### Other

| Command | Description |
|---------|-------------|
| `mem init [--global]` | Initialize a memory store |
| `mem watch [--debounce]` | Watch for file changes, auto-commit |

### Global Flags

| Flag | Description |
|------|-------------|
| `--scope=<global\|project>` | Target scope |
| `--branch=<name>` | Target branch |
| `--json` | JSON output |

## Scopes

`mem` supports two scopes:

- **Project scope** (`./.mem`) — per-project memories, found by walking up from the current directory.
- **Global scope** (`~/.mem`) — user-wide memories.

By default, `mem` uses project scope if a `.mem` directory exists in the current directory or any parent. Otherwise, it falls back to global scope. Use `--scope=global` to target global scope explicitly.

## Configuration

Configuration lives in `.mem/config.yaml`:

```yaml
embeddings:
  backend: gollama
  model: nomic-embed-text-v1.5.Q4_K_M.gguf
  dimension: 768

providers:
  openrouter:
    api_key: sk-or-...
    base_url: https://openrouter.ai/api/v1
    model: anthropic/claude-sonnet-4-20250514

default_provider: openrouter
```

## Public API

Use `mem` as a library in your Go programs:

```go
package main

import (
    "context"
    "fmt"

    mem "github.com/4thel00z/memories/pkg/v1"
)

func main() {
    client, err := mem.New()
    if err != nil {
        panic(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Store
    client.Set(ctx, "my/key", []byte("hello world"))

    // Retrieve
    data, _ := client.Get(ctx, "my/key")
    fmt.Println(string(data)) // hello world

    // List
    memories, _ := client.List(ctx, "my")
    for _, m := range memories {
        fmt.Printf("%s: %s\n", m.Key, m.Content)
    }

    // Delete
    client.Delete(ctx, "my/key")
}
```

## Extensibility

Any executable named `mem-*` in your `$PATH` becomes a subcommand:

```bash
# Install a plugin
cp my-plugin /usr/local/bin/mem-backup
chmod +x /usr/local/bin/mem-backup

# Use it
mem backup
```

External commands receive these environment variables:

| Variable | Example |
|----------|---------|
| `MEM_SCOPE` | `project` |
| `MEM_SCOPE_PATH` | `/home/user/project/.mem` |
| `MEM_ROOT` | `/home/user/project` |
| `MEM_BRANCH` | `main` |
| `MEM_CONFIG` | `/home/user/project/.mem/config.yaml` |
| `MEM_VERSION` | `1.0.0` |
| `MEM_BIN` | `/usr/local/bin/mem` |

<details>
<summary>Example Plugin (Shell)</summary>

```bash
#!/bin/bash
# /usr/local/bin/mem-backup
set -e
OUTPUT="${1:-memories-backup-$(date +%Y%m%d).tar.gz}"
echo "Backing up $MEM_SCOPE_PATH to $OUTPUT..."
tar -czf "$OUTPUT" -C "$(dirname "$MEM_SCOPE_PATH")" "$(basename "$MEM_SCOPE_PATH")"
echo "Done: $OUTPUT"
```

</details>

<details>
<summary>Example Plugin (Go)</summary>

```go
package main

import (
    "context"
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

</details>

## Storage Layout

```
.mem/
├── objects/          # Git object storage
├── refs/
│   └── heads/
│       └── main     # Default branch
├── HEAD             # Current branch ref
├── index            # Git staging area
├── config.yaml      # Mem configuration
└── vectors/
    ├── index.ann    # Annoy vector index
    └── mapping.json # Key-to-ID mapping
```

## Claude Code Skill

mem ships with a [Claude Code skill](https://docs.anthropic.com/en/docs/claude-code/skills) that teaches Claude to recall and store memories automatically during coding sessions.

### Install the skill

```bash
# From your project root (where .claude/ lives)
mkdir -p .claude/skills
cp -r skills/using-mem .claude/skills/using-mem
```

Or symlink it so it stays in sync with upstream:

```bash
mkdir -p .claude/skills
ln -s "$(pwd)/skills/using-mem" .claude/skills/using-mem
```

Once installed, Claude Code will automatically recall relevant memories at session start and store decisions at milestones. Run `/skill using-mem` to invoke it manually.

## Development

```bash
make help          # Show all targets
make build         # Build mem binary
make test          # Run all tests
make test-cover    # Run tests with coverage report
make lint          # Run golangci-lint
make install-hooks # Install git hooks (lefthook)
```

## License

MIT
