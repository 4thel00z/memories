package main

import (
	"testing"
)

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd("1.0.0")

	if cmd == nil {
		t.Fatal("NewRootCmd returned nil")
	}

	if cmd.Use != "mem" {
		t.Errorf("expected Use='mem', got %q", cmd.Use)
	}

	if cmd.Version != "1.0.0" {
		t.Errorf("expected Version='1.0.0', got %q", cmd.Version)
	}
}

func TestRootCmdHasFlags(t *testing.T) {
	cmd := NewRootCmd("1.0.0")

	flags := []string{"scope", "branch", "json"}
	for _, name := range flags {
		f := cmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("expected persistent flag %q to exist", name)
		}
	}
}

func TestRootCmdVersion(t *testing.T) {
	versions := []string{"dev", "1.0.0", "2.3.4-beta"}

	for _, v := range versions {
		cmd := NewRootCmd(v)
		if cmd.Version != v {
			t.Errorf("expected version %q, got %q", v, cmd.Version)
		}
	}
}
