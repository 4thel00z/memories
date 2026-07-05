package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/4thel00z/memories/internal"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func NewWatchCmd(commitUC *internal.CommitUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for changes and auto-commit",
		Long:  `Watch the memory store for file changes and automatically commit them.`,
		RunE:  makeWatchRunner(commitUC),
	}

	cmd.Flags().Duration("debounce", 500*time.Millisecond, "Debounce window for batching changes")
	return cmd
}

func makeWatchRunner(commitUC *internal.CommitUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")
		debounce, _ := cmd.Flags().GetDuration("debounce")

		resolver := internal.NewScopeResolver()
		scope := resolver.Resolve(scopeHint)

		if _, err := os.Stat(scope.MemPath); os.IsNotExist(err) {
			return fmt.Errorf("not initialized: %s", scope.MemPath)
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("create watcher: %w", err)
		}
		defer func() { _ = watcher.Close() }()

		if err := addWatchDirs(watcher, scope.MemPath); err != nil {
			return fmt.Errorf("add watch dirs: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Watching %s for changes...\n", scope.MemPath)

		// Start with one debounce window pending so changes made while watch
		// was not running are committed on startup instead of sitting dirty
		// until the next event.
		timer := time.NewTimer(debounce)
		pending := true

		for {
			select {
			case <-cmd.Context().Done():
				return nil
			case event, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if shouldIgnoreEvent(event, scope.MemPath) {
					continue
				}
				// fsnotify does not watch recursively; pick up new key
				// directories as they appear.
				if event.Op&fsnotify.Create != 0 {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = watcher.Add(event.Name)
					}
				}
				if !pending {
					timer.Reset(debounce)
					pending = true
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "watch error: %v\n", err)
			case <-timer.C:
				pending = false
				out, commitErr := commitUC.Execute(cmd.Context(), internal.CommitInput{
					Message: "auto: watch commit", Scope: scopeHint,
				})
				if errors.Is(commitErr, internal.ErrNothingToCommit) {
					continue
				}
				if commitErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch commit: %v\n", commitErr)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", internal.ShortHash(out.Hash), out.Message)
			}
		}
	}
}

// addWatchDirs registers the store's key directories, skipping git internals
// and the vector index.
func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			base := filepath.Base(path)
			if path != root && (strings.HasPrefix(base, ".") || path == filepath.Join(root, "vectors")) {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}

// shouldIgnoreEvent filters events down to memory content, using the same
// store-metadata definition as status, diff, and commit.
func shouldIgnoreEvent(event fsnotify.Event, memPath string) bool {
	rel, err := filepath.Rel(memPath, event.Name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return true
	}
	if internal.IsStoreMetadata(filepath.ToSlash(rel)) {
		return true
	}

	return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0
}
