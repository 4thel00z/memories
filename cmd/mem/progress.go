package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/4thel00z/memories/internal"
	"github.com/charmbracelet/x/term"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
)

// rebuildProgressMsg reports reindex progress from the worker goroutine.
type rebuildProgressMsg struct{ done, total int }

// rebuildDoneMsg signals the reindex finished (successfully or not).
type rebuildDoneMsg struct{}

type rebuildModel struct {
	bar      progress.Model
	done     int
	total    int
	finished bool
}

func newRebuildModel() rebuildModel {
	return rebuildModel{
		bar: progress.New(progress.WithDefaultBlend(), progress.WithWidth(40)),
	}
}

func (m rebuildModel) Init() tea.Cmd { return nil }

func (m rebuildModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case rebuildProgressMsg:
		m.done, m.total = msg.done, msg.total
		return m, nil
	case rebuildDoneMsg:
		m.finished = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if s := msg.String(); s == "ctrl+c" || s == "q" {
			m.finished = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m rebuildModel) View() tea.View {
	// Clear the line on exit; the caller prints the final summary.
	if m.finished {
		return tea.NewView("")
	}
	// Render the bar statically from the known ratio (determinate task), so there
	// is no animation lag racing the done signal.
	var pct float64
	if m.total > 0 {
		pct = float64(m.done) / float64(m.total)
	}
	return tea.NewView(fmt.Sprintf("Reindexing %d/%d  %s\n", m.done, m.total, m.bar.ViewAs(pct)))
}

// runRebuild executes a reindex, rendering a live progress bar when stdout is a
// terminal and falling back to a quiet run (with a single summary line) when it
// is not — so piped/CI output stays clean.
func runRebuild(ctx context.Context, rebuildUC *internal.RebuildIndexUseCase, in internal.RebuildIndexInput, out io.Writer) error {
	f, isFile := out.(*os.File)
	if !isFile || !term.IsTerminal(f.Fd()) {
		if err := rebuildUC.Execute(ctx, in); err != nil {
			return err
		}
		fmt.Fprintln(out, "Index rebuilt successfully.")
		return nil
	}

	p := tea.NewProgram(newRebuildModel(), tea.WithContext(ctx), tea.WithOutput(f))

	in.OnProgress = func(done, total int) {
		p.Send(rebuildProgressMsg{done: done, total: total})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- rebuildUC.Execute(ctx, in)
		p.Send(rebuildDoneMsg{})
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	if err := <-errCh; err != nil {
		return err
	}
	fmt.Fprintln(out, "Index rebuilt successfully.")
	return nil
}
