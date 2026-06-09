package main

import (
	"os"
	"os/exec"
	"strings"
)

// editorCommand builds a command to open file in the user's editor. It honors
// $VISUAL then $EDITOR (falling back to vi) and splits the value into fields so
// editors configured with arguments — "code --wait", "vim -u NONE",
// "emacsclient -c" — are invoked correctly instead of being treated as a single
// executable name.
func editorCommand(file string) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	args := append(parts[1:], file)
	return exec.Command(parts[0], args...)
}
