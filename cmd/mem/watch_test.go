package main

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestShouldIgnoreEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   fsnotify.Event
		memPath string
		want    bool
	}{
		{
			name:    "write outside .mem",
			event:   fsnotify.Event{Name: "/project/data.txt", Op: fsnotify.Write},
			memPath: "/project/.mem",
			want:    false,
		},
		{
			name:    "write inside .mem",
			event:   fsnotify.Event{Name: "/project/.mem/objects/abc", Op: fsnotify.Write},
			memPath: "/project/.mem",
			want:    true,
		},
		{
			name:    "chmod event ignored",
			event:   fsnotify.Event{Name: "/project/data.txt", Op: fsnotify.Chmod},
			memPath: "/project/.mem",
			want:    true,
		},
		{
			name:    "create outside .mem",
			event:   fsnotify.Event{Name: "/project/new.txt", Op: fsnotify.Create},
			memPath: "/project/.mem",
			want:    false,
		},
		{
			name:    "remove outside .mem",
			event:   fsnotify.Event{Name: "/project/old.txt", Op: fsnotify.Remove},
			memPath: "/project/.mem",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnoreEvent(tt.event, tt.memPath)
			if got != tt.want {
				t.Errorf("shouldIgnoreEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
