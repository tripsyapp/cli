package config

import "testing"

func TestCredentialsRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	in := Credentials{Token: "token", BaseURL: "https://example.com"}

	if err := store.SaveCredentials(in); err != nil {
		t.Fatal(err)
	}
	out, err := store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("credentials = %#v, want %#v", out, in)
	}

	if err := store.ClearCredentials(); err != nil {
		t.Fatal(err)
	}
	out, err = store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if out.Token != "" || out.BaseURL != "" {
		t.Fatalf("credentials after clear = %#v, want empty", out)
	}
}
