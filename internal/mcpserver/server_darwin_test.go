package mcpserver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRequestTokenOnlyDoesNotAccessKeychain(t *testing.T) {
	dir := t.TempDir()
	credentialsPath := filepath.Join(dir, "credentials.json")
	credentials := []byte(`{"token":"legacy-token","base_url":"https://example.com"}`)
	if err := os.WriteFile(credentialsPath, credentials, 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	keychainLog := filepath.Join(binDir, "keychain-called")
	script := "#!/bin/sh\nprintf 'called\\n' > \"$TRIPSY_TEST_KEYCHAIN_LOG\"\nprintf 'keychain-token\\n'\n"
	if err := os.WriteFile(filepath.Join(binDir, "security"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("TRIPSY_TEST_KEYCHAIN_LOG", keychainLog)
	t.Setenv("TRIPSY_AUTH_BACKEND", "keychain")

	_, info, err := New(Options{ConfigDir: dir, RequestTokenOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if info.HasToken {
		t.Fatal("request-only startup must not use stored tokens")
	}
	if _, err := os.Stat(keychainLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("request-only startup must not call Keychain")
	}
	after, err := os.ReadFile(credentialsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, credentials) {
		t.Fatal("request-only startup must leave legacy credentials unchanged")
	}
}
