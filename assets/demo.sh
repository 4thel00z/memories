#!/bin/bash
# Demo script for asciinema recording
# Uses a temp dir to avoid polluting the real project

set +e

DEMO_DIR=$(mktemp -d)
cd "$DEMO_DIR"

# Typing simulator
type_cmd() {
    echo ""
    printf "\033[1;32m❯\033[0m "
    for ((i=0; i<${#1}; i++)); do
        printf "%s" "${1:$i:1}"
        sleep 0.04
    done
    echo ""
    sleep 0.3
    eval "$1"
    sleep 1
}

clear
echo ""
echo "  ┌─────────────────────────────────────┐"
echo "  │  mem — Git-powered memory store     │"
echo "  │  with semantic search & git hooks   │"
echo "  └─────────────────────────────────────┘"
echo ""
sleep 2

# Init
type_cmd "mem init"

# Store memories
type_cmd "mem set arch/storage 'Git-backed flat files under .mem/ with go-git'"
type_cmd "mem set bugs/nil-ptr 'LoadConfig panics on empty file. Fix: nil check after unmarshal.'"
type_cmd "mem set ctx/stack 'Go 1.25, Cobra CLI, purego bindings for llama.cpp'"
type_cmd "mem set ctx/embedding 'nomic-embed-text-v1.5 via gollama.cpp purego bindings'"
type_cmd "mem set patterns/tdd 'Always write failing test first, then minimal implementation'"

# List
type_cmd "mem list"

# Get
type_cmd "mem get arch/storage"

# Keyword search
type_cmd "mem search config"
type_cmd "mem search purego"
type_cmd "mem search 'test first'"

# Semantic search (before index — shows fallback behavior)
type_cmd "mem search -s 'how does the embedding model work' -n 3"
type_cmd "mem search -s 'what causes crashes' -n 3"

# Rebuild index
type_cmd "mem index rebuild"

# Semantic search again after index
type_cmd "mem search -s 'how does the embedding model work' -n 3"

# History
type_cmd "mem log --oneline"

# Branch
type_cmd "mem branch experiments"
type_cmd "mem set experiment/idea 'Try annoy trees=20 for better recall'"
type_cmd "mem list"
type_cmd "mem branch main"

# Install hook
type_cmd "mem install --strategy extract"

# Uninstall
type_cmd "mem uninstall"

echo ""
echo "  ✓ Done! See github.com/4thel00z/memories"
echo ""
sleep 2

# Cleanup
rm -rf "$DEMO_DIR"
