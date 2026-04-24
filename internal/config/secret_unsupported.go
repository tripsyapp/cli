//go:build !darwin

package config

func newOSSecretStore() (secretStore, bool) {
	return nil, false
}
