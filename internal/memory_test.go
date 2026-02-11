package internal

import (
	"testing"
)

func TestNewKeyValid(t *testing.T) {
	valid := []string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"project.config",
		"my-key",
		"my_key",
		"a",
		"A1",
		"config/db/host",
		"user.preferences.theme",
	}

	for _, s := range valid {
		key, err := NewKey(s)
		if err != nil {
			t.Errorf("NewKey(%q) returned error: %v", s, err)
			continue
		}
		if key.String() != s {
			t.Errorf("expected key %q, got %q", s, key.String())
		}
	}
}

func TestNewKeyInvalid(t *testing.T) {
	invalid := []string{
		"",
		"-start-with-dash",
		".start-with-dot",
		"/start-with-slash",
		"_start-with-underscore",
		"has spaces",
		"has\ttab",
		"has\nnewline",
		"special!char",
		"special@char",
		"special#char",
	}

	for _, s := range invalid {
		_, err := NewKey(s)
		if err != ErrInvalidKey {
			t.Errorf("NewKey(%q) expected ErrInvalidKey, got %v", s, err)
		}
	}
}

func TestKeyString(t *testing.T) {
	key, _ := NewKey("test/key")
	if key.String() != "test/key" {
		t.Errorf("expected 'test/key', got %q", key.String())
	}
}
