package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCmd(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	memPath := filepath.Join(tmpDir, ".mem")
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		t.Error(".mem directory not created")
	}

	vectorsPath := filepath.Join(memPath, "vectors")
	if _, err := os.Stat(vectorsPath); os.IsNotExist(err) {
		t.Error("vectors directory not created")
	}

	configPath := filepath.Join(memPath, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config.yaml not created")
	}
}

func TestInitCmdAlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cmd := NewInitCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for already initialized")
	}
}

func TestInitCmdGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	cmd := NewInitCmd()
	cmd.SetArgs([]string{"--global"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	memPath := filepath.Join(tmpDir, ".mem")
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		t.Error("global .mem directory not created")
	}
}
