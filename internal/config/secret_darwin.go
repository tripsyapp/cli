package config

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type keychainStore struct{}

func newOSSecretStore() (secretStore, bool) {
	if _, err := exec.LookPath("security"); err != nil {
		return nil, false
	}
	return keychainStore{}, true
}

func (keychainStore) Name() string {
	return "keychain"
}

func (keychainStore) Get(service, account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", account, "-s", service, "-w")
	out, err := cmd.Output()
	if err != nil {
		if isKeychainNotFound(err) {
			return "", errSecretNotFound
		}
		return "", fmt.Errorf("read macOS Keychain token: %w", err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (keychainStore) Set(service, account, secret string) error {
	cmd := exec.Command("security", "add-generic-password", "-a", account, "-s", service, "-w", secret, "-U")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write macOS Keychain token: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (keychainStore) Delete(service, account string) error {
	cmd := exec.Command("security", "delete-generic-password", "-a", account, "-s", service)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isKeychainNotFound(err) || strings.Contains(string(out), "could not be found") {
			return errSecretNotFound
		}
		return fmt.Errorf("delete macOS Keychain token: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isKeychainNotFound(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 44
}
