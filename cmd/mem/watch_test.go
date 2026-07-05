package main

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestShouldIgnoreEvent(t *testing.T) {
	const memPath = "/project/.mem"

	tests := []struct {
		name  string
		event fsnotify.Event
		want  bool
	}{
		{
			name:  "memory file write triggers",
			event: fsnotify.Event{Name: "/project/.mem/notes/todo", Op: fsnotify.Write},
			want:  false,
		},
		{
			name:  "memory file create triggers",
			event: fsnotify.Event{Name: "/project/.mem/new-key", Op: fsnotify.Create},
			want:  false,
		},
		{
			name:  "memory file remove triggers",
			event: fsnotify.Event{Name: "/project/.mem/old-key", Op: fsnotify.Remove},
			want:  false,
		},
		{
			name:  "git internals ignored",
			event: fsnotify.Event{Name: "/project/.mem/.git/objects/abc", Op: fsnotify.Write},
			want:  true,
		},
		{
			name:  "vector index ignored",
			event: fsnotify.Event{Name: "/project/.mem/vectors/index.ann", Op: fsnotify.Write},
			want:  true,
		},
		{
			name:  "config ignored",
			event: fsnotify.Event{Name: "/project/.mem/config.yaml", Op: fsnotify.Write},
			want:  true,
		},
		{
			name:  "atomic-write temp file ignored",
			event: fsnotify.Event{Name: "/project/.mem/notes/todo.tmp", Op: fsnotify.Create},
			want:  true,
		},
		{
			name:  "file outside the store ignored",
			event: fsnotify.Event{Name: "/project/data.txt", Op: fsnotify.Write},
			want:  true,
		},
		{
			name:  "chmod ignored",
			event: fsnotify.Event{Name: "/project/.mem/notes/todo", Op: fsnotify.Chmod},
			want:  true,
		},
		{
			name:  "nested key containing vectors segment triggers",
			event: fsnotify.Event{Name: "/project/.mem/notes/vectors/idea", Op: fsnotify.Write},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreEvent(tt.event, memPath)
			if got != tt.want {
				t.Errorf("shouldIgnoreEvent(%q) = %v, want %v", tt.event.Name, got, tt.want)
			}
		})
	}
}
