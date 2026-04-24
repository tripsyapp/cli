package config

import (
	"os"
	"strings"
	"testing"
)

type fakeSecretStore struct {
	values map[string]string
}

func (f *fakeSecretStore) Name() string {
	return "keychain"
}

func (f *fakeSecretStore) Get(service, account string) (string, error) {
	value, ok := f.values[service+"/"+account]
	if !ok {
		return "", errSecretNotFound
	}
	return value, nil
}

func (f *fakeSecretStore) Set(service, account, secret string) error {
	f.values[service+"/"+account] = secret
	return nil
}

func (f *fakeSecretStore) Delete(service, account string) error {
	key := service + "/" + account
	if _, ok := f.values[key]; !ok {
		return errSecretNotFound
	}
	delete(f.values, key)
	return nil
}

func TestCredentialsRoundTrip(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
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

func TestSaveCredentialsRepairsFilePermissions(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.Dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.CredentialsPath(), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveCredentials(Credentials{Token: "token"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.CredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credentials mode = %#o, want 0600", got)
	}
	dirInfo, err := os.Stat(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("config dir mode = %#o, want 0700", got)
	}
}

func TestKeychainBackendStoresTokenOutsideCredentialsFile(t *testing.T) {
	secrets := &fakeSecretStore{values: map[string]string{}}
	store := &Store{Dir: t.TempDir(), AuthBackend: "keychain", secrets: secrets}

	in := Credentials{Token: "secret-token", BaseURL: "https://example.com"}
	if err := store.SaveCredentials(in); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(store.CredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-token") {
		t.Fatalf("credentials file contains token: %s", raw)
	}
	out, err := store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("credentials = %#v, want %#v", out, in)
	}
}

func TestLoadCredentialsMigratesLegacyTokenToKeychain(t *testing.T) {
	dir := t.TempDir()
	legacy := &Store{Dir: dir, AuthBackend: "file"}
	if err := legacy.SaveCredentials(Credentials{Token: "legacy-token", BaseURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}

	secrets := &fakeSecretStore{values: map[string]string{}}
	store := &Store{Dir: dir, AuthBackend: "keychain", secrets: secrets}
	out, err := store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if out.Token != "legacy-token" || out.BaseURL != "https://example.com" {
		t.Fatalf("credentials = %#v, want migrated legacy token", out)
	}
	raw, err := os.ReadFile(store.CredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "legacy-token") {
		t.Fatalf("credentials file still contains migrated token: %s", raw)
	}
	if got := secrets.values[tokenService+"/"+tokenAccount]; got != "legacy-token" {
		t.Fatalf("keychain token = %q, want legacy-token", got)
	}
}

func TestClearCredentialsDeletesKeychainTokenAndFile(t *testing.T) {
	secrets := &fakeSecretStore{values: map[string]string{}}
	store := &Store{Dir: t.TempDir(), AuthBackend: "keychain", secrets: secrets}
	if err := store.SaveCredentials(Credentials{Token: "secret-token", BaseURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearCredentials(); err != nil {
		t.Fatal(err)
	}
	if _, ok := secrets.values[tokenService+"/"+tokenAccount]; ok {
		t.Fatal("keychain token was not deleted")
	}
	if _, err := os.Stat(store.CredentialsPath()); !os.IsNotExist(err) {
		t.Fatalf("credentials file stat error = %v, want not exist", err)
	}
}
