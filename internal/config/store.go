package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Credentials struct {
	Token   string `json:"token,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

type Store struct {
	Dir string
}

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
	return &Store{Dir: dir}
}

func (s *Store) CredentialsPath() string {
	return filepath.Join(s.Dir, "credentials.json")
}

func (s *Store) LoadCredentials() (Credentials, error) {
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

func (s *Store) SaveCredentials(credentials Credentials) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.CredentialsPath(), data, 0o600)
}

func (s *Store) ClearCredentials() error {
	err := os.Remove(s.CredentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
