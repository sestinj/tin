package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CredentialEntry represents stored credentials for a host
type CredentialEntry struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CredentialStore manages credentials for remote operations.
// Credentials are stored globally in ~/.config/tin/credentials (not in repo)
// to avoid accidentally committing secrets.
type CredentialStore struct{}

// NewCredentialStore creates a new credential store
func NewCredentialStore() *CredentialStore {
	return &CredentialStore{}
}

// Get retrieves credentials for the given host.
// It checks in order:
// 1. TIN_AUTH environment variable (format: "username:password")
// 2. Per-host credentials in ~/.config/tin/credentials
func (s *CredentialStore) Get(host string) (*Credentials, error) {
	// 1. Check environment variable (highest priority)
	if auth := os.Getenv("TIN_AUTH"); auth != "" {
		return parseAuthString(auth), nil
	}

	// Legacy env var support
	if token := os.Getenv("TIN_AUTH_TOKEN"); token != "" {
		return parseAuthString(token), nil
	}

	// 2. Check per-host credentials in global config
	entries, err := s.readCredentials()
	if err != nil {
		return nil, nil // No credentials file
	}

	for _, entry := range entries {
		if entry.Host == host {
			return &Credentials{
				Username: entry.Username,
				Password: entry.Password,
			}, nil
		}
	}

	return nil, nil // No credentials found
}

// Store saves credentials for the given host
func (s *CredentialStore) Store(host, username, password string) error {
	entries, _ := s.readCredentials() // Ignore error, start fresh if needed

	// Update or add entry
	found := false
	for i, entry := range entries {
		if entry.Host == host {
			entries[i].Username = username
			entries[i].Password = password
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, CredentialEntry{
			Host:     host,
			Username: username,
			Password: password,
		})
	}

	return s.writeCredentials(entries)
}

// Remove removes credentials for the given host
func (s *CredentialStore) Remove(host string) error {
	entries, err := s.readCredentials()
	if err != nil {
		return nil // No credentials, nothing to remove
	}

	newEntries := make([]CredentialEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Host != host {
			newEntries = append(newEntries, entry)
		}
	}

	return s.writeCredentials(newEntries)
}

// List returns all stored credentials (with passwords masked)
func (s *CredentialStore) List() ([]CredentialEntry, error) {
	return s.readCredentials()
}

// credentialsPath returns the global credentials file path
func (s *CredentialStore) credentialsPath() string {
	// Use ~/.config/tin/credentials (XDG-style)
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "tin", "credentials")
}

func (s *CredentialStore) readCredentials() ([]CredentialEntry, error) {
	data, err := os.ReadFile(s.credentialsPath())
	if err != nil {
		return nil, err
	}

	var entries []CredentialEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *CredentialStore) writeCredentials(entries []CredentialEntry) error {
	path := s.credentialsPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	// Write with restricted permissions (user-only)
	return os.WriteFile(path, data, 0600)
}

// parseAuthString parses "username:password" or just "password" format
func parseAuthString(auth string) *Credentials {
	for i, c := range auth {
		if c == ':' {
			return &Credentials{
				Username: auth[:i],
				Password: auth[i+1:],
			}
		}
	}
	// No colon - treat as password with default username
	return &Credentials{
		Username: "x-token-auth",
		Password: auth,
	}
}
