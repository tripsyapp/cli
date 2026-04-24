package terminal

import (
	"strings"
	"testing"
)

func TestReadPasswordFromNonTerminalReader(t *testing.T) {
	password, hidden, err := ReadPassword(strings.NewReader("secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	if hidden {
		t.Fatal("hidden = true, want false for non-terminal reader")
	}
	if password != "secret" {
		t.Fatalf("password = %q, want secret", password)
	}
}
