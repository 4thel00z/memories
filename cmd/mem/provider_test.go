package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func setupProviderTest(t *testing.T) (
	*internal.ProviderListUseCase,
	*internal.ProviderAddUseCase,
	*internal.ProviderRemoveUseCase,
	*internal.ProviderSetDefaultUseCase,
	*internal.ProviderTestUseCase,
) {
	t.Helper()
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	scope := internal.Scope{
		Type:    internal.ScopeProject,
		Path:    tmpDir,
		MemPath: filepath.Join(tmpDir, ".mem"),
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	if err := internal.InitRepository(scope); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	cfg := internal.DefaultConfig()
	if err := internal.SaveConfig(scope, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	resolver := internal.NewScopeResolver()
	return internal.NewProviderListUseCase(resolver),
		internal.NewProviderAddUseCase(resolver),
		internal.NewProviderRemoveUseCase(resolver),
		internal.NewProviderSetDefaultUseCase(resolver),
		internal.NewProviderTestUseCase(resolver)
}

func TestProviderListEmpty(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	cmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	cmd.SetArgs([]string{"list"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "No providers") {
		t.Errorf("expected 'No providers' message, got %q", out.String())
	}
}

func TestProviderAddAndList(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	// Add a provider
	addCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	addCmd.SetArgs([]string{"add", "openai", "--api-key", "sk-test", "--model", "gpt-4"})
	var addOut bytes.Buffer
	addCmd.SetOut(&addOut)

	if err := addCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	if !strings.Contains(addOut.String(), "Added provider openai") {
		t.Errorf("unexpected add output: %q", addOut.String())
	}

	// List should show it
	listCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	listCmd.SetArgs([]string{"list"})
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}

	if !strings.Contains(listOut.String(), "openai") {
		t.Errorf("expected 'openai' in list, got %q", listOut.String())
	}
}

func TestProviderListShowsMaskedDetails(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	addCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	addCmd.SetArgs([]string{
		"add", "openrouter",
		"--api-key", "sk-secretkey1234567890",
		"--base-url", "https://openrouter.ai/api/v1",
		"--model", "anthropic/claude-sonnet-4",
	})
	var addOut bytes.Buffer
	addCmd.SetOut(&addOut)
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	listCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	listCmd.SetArgs([]string{"list"})
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}

	got := listOut.String()
	if !strings.Contains(got, "https://openrouter.ai/api/v1") {
		t.Errorf("expected base_url in list output, got %q", got)
	}
	if !strings.Contains(got, "anthropic/claude-sonnet-4") {
		t.Errorf("expected model in list output, got %q", got)
	}
	if strings.Contains(got, "sk-secretkey1234567890") {
		t.Errorf("api key must be masked, but full key leaked in %q", got)
	}
	if !strings.Contains(got, maskSecret("sk-secretkey1234567890")) {
		t.Errorf("expected masked api key in list output, got %q", got)
	}
}

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "(not set)"},
		{"short", "****"},
		{"12345678", "****"},
		{"sk-secretkey1234567890", "sk-s...7890"},
	}
	for _, c := range cases {
		if got := maskSecret(c.in); got != c.want {
			t.Errorf("maskSecret(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProviderRemove(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	// Add then remove
	addCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	addCmd.SetArgs([]string{"add", "todelete", "--api-key", "x"})
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	rmCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	rmCmd.SetArgs([]string{"remove", "todelete"})
	var rmOut bytes.Buffer
	rmCmd.SetOut(&rmOut)

	if err := rmCmd.Execute(); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if !strings.Contains(rmOut.String(), "Removed provider todelete") {
		t.Errorf("unexpected remove output: %q", rmOut.String())
	}
}

func TestProviderPrompterFillsMissingFields(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("https://api.example.com/v1\ngpt-4o\n"))
	var out bytes.Buffer
	p := providerPrompter{
		in:         in,
		out:        &out,
		readSecret: func(string) (string, error) { return "sk-from-prompt", nil },
	}

	got, err := p.fill(internal.ProviderConfig{})
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	if got.BaseURL != "https://api.example.com/v1" {
		t.Errorf("base url: got %q", got.BaseURL)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("model: got %q", got.Model)
	}
	if got.APIKey != "sk-from-prompt" {
		t.Errorf("api key: got %q", got.APIKey)
	}
}

func TestProviderPrompterKeepsProvidedFields(t *testing.T) {
	p := providerPrompter{
		in:  bufio.NewReader(strings.NewReader("")),
		out: &bytes.Buffer{},
		readSecret: func(string) (string, error) {
			t.Fatal("should not prompt for api key when already provided")
			return "", nil
		},
	}

	in := internal.ProviderConfig{
		APIKey:  "sk-flag",
		BaseURL: "https://flag.example.com",
		Model:   "flag-model",
	}
	got, err := p.fill(in)
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	if got != in {
		t.Errorf("expected config unchanged, got %+v", got)
	}
}

func TestProviderSetDefault(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	// Add a provider first
	addCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	addCmd.SetArgs([]string{"add", "myp", "--api-key", "x"})
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Set as default
	defCmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	defCmd.SetArgs([]string{"default", "myp"})
	var defOut bytes.Buffer
	defCmd.SetOut(&defOut)

	if err := defCmd.Execute(); err != nil {
		t.Fatalf("default: %v", err)
	}

	if !strings.Contains(defOut.String(), "Default provider set to myp") {
		t.Errorf("unexpected default output: %q", defOut.String())
	}
}

func TestProviderSetDefaultNonexistent(t *testing.T) {
	listUC, addUC, removeUC, setDefUC, testUC := setupProviderTest(t)

	cmd := NewProviderCmd(listUC, addUC, removeUC, setDefUC, testUC)
	cmd.SetArgs([]string{"default", "nonexistent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}
