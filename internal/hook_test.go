package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookMarker(t *testing.T) {
	script := HookScript("post-commit")
	assert.Contains(t, script, "#!/bin/sh")
	assert.Contains(t, script, HookMarker)
	assert.Contains(t, script, "mem hook run post-commit")
}

func TestIsManagedHook(t *testing.T) {
	assert.True(t, IsManagedHook(HookScript("post-commit")))
	assert.False(t, IsManagedHook("#!/bin/sh\necho hello"))
	assert.False(t, IsManagedHook(""))
}

func TestFindGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	found, err := FindGitDir(dir)
	assert.NoError(t, err)
	assert.Equal(t, gitDir, found)

	// non-git dir
	noGit := t.TempDir()
	_, err = FindGitDir(noGit)
	assert.Error(t, err)
}
