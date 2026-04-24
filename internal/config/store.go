package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Credentials struct {
	Token   string `json:"token,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

type Store struct {
	Dir         string
	AuthBackend string
	secrets     secretStore
}

var errSecretNotFound = errors.New("secret not found")

type secretStore interface {
	Name() string
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

const (
	authBackendAuto     = "auto"
	authBackendFile     = "file"
	authBackendKeychain = "keychain"
	tokenService        = "tripsy-cli"
	tokenAccount        = "token"
)

func DefaultDir() string {
	if dir := os.Getenv("TRIPSY_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "tripsy-cli")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".tripsy-cli"
	}
	return filepath.Join(home, ".config", "tripsy-cli")
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Store{Dir: dir, AuthBackend: os.Getenv("TRIPSY_AUTH_BACKEND")}
}

func (s *Store) CredentialsPath() string {
	return filepath.Join(s.Dir, "credentials.json")
}

func (s *Store) AuthBackendName() string {
	backend, secrets, err := s.resolveAuthBackend()
	if err != nil {
		return strings.TrimSpace(s.AuthBackend)
	}
	if secrets != nil {
		return secrets.Name()
	}
	return backend
}

func (s *Store) LoadCredentials() (Credentials, error) {
	credentials, err := s.loadFileCredentials()
	if err != nil {
		return credentials, err
	}

	backend, secrets, err := s.resolveAuthBackend()
	if err != nil {
		return credentials, err
	}
	if backend == authBackendFile {
		return credentials, nil
	}

	token, err := secrets.Get(tokenService, tokenAccount)
	if err == nil {
		if credentials.Token != "" {
			credentials.Token = ""
			if err := s.saveFileCredentials(credentials, false); err != nil {
				return credentials, err
			}
		}
		credentials.Token = token
		return credentials, nil
	}
	if !errors.Is(err, errSecretNotFound) {
		return credentials, err
	}
	if credentials.Token == "" {
		return credentials, nil
	}

	token = credentials.Token
	if err := secrets.Set(tokenService, tokenAccount, token); err != nil {
		return credentials, fmt.Errorf("migrate token to %s: %w", secrets.Name(), err)
	}
	credentials.Token = ""
	if err := s.saveFileCredentials(credentials, false); err != nil {
		return credentials, err
	}
	credentials.Token = token
	return credentials, nil
}

func (s *Store) SaveCredentials(credentials Credentials) error {
	backend, secrets, err := s.resolveAuthBackend()
	if err != nil {
		return err
	}
	if backend == authBackendFile {
		return s.saveFileCredentials(credentials, true)
	}
	if credentials.Token != "" {
		if err := secrets.Set(tokenService, tokenAccount, credentials.Token); err != nil {
			return err
		}
	}
	return s.saveFileCredentials(credentials, false)
}

func (s *Store) ClearCredentials() error {
	_, secrets, backendErr := s.resolveAuthBackend()
	if backendErr == nil && secrets != nil {
		if err := secrets.Delete(tokenService, tokenAccount); err != nil && !errors.Is(err, errSecretNotFound) {
			return err
		}
	}
	err := os.Remove(s.CredentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) loadFileCredentials() (Credentials, error) {
	var credentials Credentials
	data, err := os.ReadFile(s.CredentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return credentials, nil
	}
	if err != nil {
		return credentials, err
	}
	if err := json.Unmarshal(data, &credentials); err != nil {
		return credentials, err
	}
	return credentials, nil
}

func (s *Store) saveFileCredentials(credentials Credentials, includeToken bool) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return err
	}
	if !includeToken {
		credentials.Token = ""
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.CredentialsPath(), data, 0o600); err != nil {
		return err
	}
	return os.Chmod(s.CredentialsPath(), 0o600)
}

func (s *Store) resolveAuthBackend() (string, secretStore, error) {
	backend := strings.ToLower(strings.TrimSpace(s.AuthBackend))
	if backend == "" {
		backend = authBackendAuto
	}
	switch backend {
	case authBackendAuto:
		if secrets, ok := s.osSecretStore(); ok {
			return authBackendKeychain, secrets, nil
		}
		return authBackendFile, nil, nil
	case authBackendFile:
		return authBackendFile, nil, nil
	case authBackendKeychain:
		secrets, ok := s.osSecretStore()
		if !ok {
			return "", nil, errors.New("keychain auth backend is not available on this system")
		}
		return authBackendKeychain, secrets, nil
	default:
		return "", nil, fmt.Errorf("unsupported TRIPSY_AUTH_BACKEND %q; expected auto, keychain, or file", backend)
	}
}

func (s *Store) osSecretStore() (secretStore, bool) {
	if s.secrets != nil {
		return s.secrets, true
	}
	return newOSSecretStore()
}
