package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRebuildModelRendersProgress(t *testing.T) {
	m := newRebuildModel()

	updated, _ := m.Update(rebuildProgressMsg{done: 1, total: 3})
	view := updated.View()

	if !strings.Contains(view.Content, "1/3") {
		t.Errorf("expected view to show 1/3, got %q", view.Content)
	}
}

func TestRebuildModelQuitsWhenDone(t *testing.T) {
	m := newRebuildModel()

	updated, cmd := m.Update(rebuildDoneMsg{})
	if cmd == nil {
		t.Fatal("expected a quit command on done")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", cmd())
	}

	// After finishing, the view clears so the caller can print its own summary.
	if got := updated.View().Content; got != "" {
		t.Errorf("expected empty view after done, got %q", got)
	}
}

func TestRebuildModelQuitsOnCtrlC(t *testing.T) {
	m := newRebuildModel()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", cmd())
	}
}
