package main

import (
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
		defer watcher.Close()

		if err := addWatchDirs(watcher, scope.Path); err != nil {
			return fmt.Errorf("add watch dirs: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Watching %s for changes...\n", scope.Path)

		timer := time.NewTimer(0)
		if !timer.Stop() {
			<-timer.C
		}
		pending := false

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
				if commitErr != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", out.Hash[:7], out.Message)
			}
		}
	}
}

func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}

func shouldIgnoreEvent(event fsnotify.Event, memPath string) bool {
	if strings.HasPrefix(event.Name, memPath) {
		return true
	}

	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return true
	}

	return false
}
