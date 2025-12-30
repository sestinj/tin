package storage

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danieladler/tin/internal/model"
)

const (
	TinDir      = ".tin"
	HeadFile    = "HEAD"
	ConfigFile  = "config"
	IndexFile   = "index.json"
	ThreadsDir  = "threads"
	CommitsDir  = "commits"
	RefsDir     = "refs"
	HeadsDir    = "heads"
)

var (
	ErrNotARepository = errors.New("not a tin repository (or any of the parent directories)")
	ErrAlreadyExists  = errors.New("tin repository already exists")
	ErrNotFound       = errors.New("not found")
)

// RemoteConfig represents a configured remote repository
type RemoteConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Config holds tin configuration
type Config struct {
	Version int            `json:"version"`
	Remotes []RemoteConfig `json:"remotes,omitempty"`
}

// Index represents the staging area
type Index struct {
	Staged []model.ThreadRef `json:"staged"`
}

// Repository represents a tin repository
type Repository struct {
	RootPath string
	TinPath  string
	IsBare   bool
}

// Init initializes a new tin repository in the given path
func Init(path string) (*Repository, error) {
	tinPath := filepath.Join(path, TinDir)

	// Check if already exists
	if _, err := os.Stat(tinPath); err == nil {
		return nil, ErrAlreadyExists
	}

	// Ensure git is initialized
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = path
		if err := cmd.Run(); err != nil {
			return nil, errors.New("failed to initialize git repository: " + err.Error())
		}
	}

	// Create directory structure
	dirs := []string{
		tinPath,
		filepath.Join(tinPath, ThreadsDir),
		filepath.Join(tinPath, CommitsDir),
		filepath.Join(tinPath, RefsDir),
		filepath.Join(tinPath, RefsDir, HeadsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	repo := &Repository{
		RootPath: path,
		TinPath:  tinPath,
	}

	// Write initial config
	config := Config{Version: 1}
	if err := repo.writeJSON(ConfigFile, config); err != nil {
		return nil, err
	}

	// Write initial HEAD (pointing to main branch)
	if err := repo.WriteHead("main"); err != nil {
		return nil, err
	}

	// Write empty index
	if err := repo.WriteIndex(&Index{Staged: []model.ThreadRef{}}); err != nil {
		return nil, err
	}

	return repo, nil
}

// Open opens an existing tin repository
func Open(path string) (*Repository, error) {
	// Search up the directory tree for .tin
	current := path
	for {
		tinPath := filepath.Join(current, TinDir)
		if _, err := os.Stat(tinPath); err == nil {
			return &Repository{
				RootPath: current,
				TinPath:  tinPath,
			}, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return nil, ErrNotARepository
		}
		current = parent
	}
}

// writeJSON writes a JSON file to the tin directory
func (r *Repository) writeJSON(name string, v interface{}) error {
	path := filepath.Join(r.TinPath, name)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// readJSON reads a JSON file from the tin directory
func (r *Repository) readJSON(name string, v interface{}) error {
	path := filepath.Join(r.TinPath, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteHead writes the current branch name to HEAD
func (r *Repository) WriteHead(branchName string) error {
	path := filepath.Join(r.TinPath, HeadFile)
	return os.WriteFile(path, []byte(branchName), 0644)
}

// ReadHead reads the current branch name from HEAD
func (r *Repository) ReadHead() (string, error) {
	path := filepath.Join(r.TinPath, HeadFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteIndex writes the staging area
func (r *Repository) WriteIndex(index *Index) error {
	return r.writeJSON(IndexFile, index)
}

// ReadIndex reads the staging area
func (r *Repository) ReadIndex() (*Index, error) {
	var index Index
	if err := r.readJSON(IndexFile, &index); err != nil {
		if os.IsNotExist(err) {
			return &Index{Staged: []model.ThreadRef{}}, nil
		}
		return nil, err
	}
	return &index, nil
}

// GetCurrentGitHash returns the current git HEAD hash
func (r *Repository) GetCurrentGitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		// Might be an empty repo with no commits yet
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// GitCheckout checks out a specific git commit
func (r *Repository) GitCheckout(hash string) error {
	cmd := exec.Command("git", "checkout", hash)
	cmd.Dir = r.RootPath
	return cmd.Run()
}

// GitAdd stages files for commit
func (r *Repository) GitAdd(files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Use -A to handle additions, modifications, and deletions
	args := append([]string{"add", "-A", "--"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("git add failed: " + string(output))
	}
	return nil
}

// GitCommit creates a git commit with the given message
func (r *Repository) GitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("git commit failed: " + string(output))
	}
	return nil
}

// GitHasStagedChanges checks if there are staged changes ready to commit
func (r *Repository) GitHasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = r.RootPath
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means there are differences (staged changes exist)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, err
	}
	// Exit code 0 means no differences (no staged changes)
	return false, nil
}

// InitBare initializes a bare tin repository (no git, no working tree)
// Bare repositories are used as remote repositories
func InitBare(path string) (*Repository, error) {
	// For bare repos, the path IS the tin directory (not .tin subdirectory)
	tinPath := path

	// Check if already exists
	configPath := filepath.Join(tinPath, ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		return nil, ErrAlreadyExists
	}

	// Create directory structure
	dirs := []string{
		tinPath,
		filepath.Join(tinPath, ThreadsDir),
		filepath.Join(tinPath, CommitsDir),
		filepath.Join(tinPath, RefsDir),
		filepath.Join(tinPath, RefsDir, HeadsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	repo := &Repository{
		RootPath: path,
		TinPath:  tinPath,
		IsBare:   true,
	}

	// Write initial config
	config := Config{Version: 1}
	if err := repo.writeJSON(ConfigFile, config); err != nil {
		return nil, err
	}

	// Write initial HEAD (pointing to main branch)
	if err := repo.WriteHead("main"); err != nil {
		return nil, err
	}

	// Write empty index
	if err := repo.WriteIndex(&Index{Staged: []model.ThreadRef{}}); err != nil {
		return nil, err
	}

	return repo, nil
}

// OpenBare opens an existing bare tin repository
func OpenBare(path string) (*Repository, error) {
	configPath := filepath.Join(path, ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, ErrNotARepository
	}

	return &Repository{
		RootPath: path,
		TinPath:  path,
		IsBare:   true,
	}, nil
}

// ReadConfig reads the repository configuration
func (r *Repository) ReadConfig() (*Config, error) {
	var config Config
	if err := r.readJSON(ConfigFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// WriteConfig writes the repository configuration
func (r *Repository) WriteConfig(config *Config) error {
	return r.writeJSON(ConfigFile, config)
}

// AddRemote adds a remote to the repository
func (r *Repository) AddRemote(name, url string) error {
	config, err := r.ReadConfig()
	if err != nil {
		return err
	}

	// Check if remote already exists
	for _, remote := range config.Remotes {
		if remote.Name == name {
			return errors.New("remote already exists: " + name)
		}
	}

	config.Remotes = append(config.Remotes, RemoteConfig{
		Name: name,
		URL:  url,
	})

	return r.WriteConfig(config)
}

// GetRemote returns a remote by name
func (r *Repository) GetRemote(name string) (*RemoteConfig, error) {
	config, err := r.ReadConfig()
	if err != nil {
		return nil, err
	}

	for _, remote := range config.Remotes {
		if remote.Name == name {
			return &RemoteConfig{Name: remote.Name, URL: remote.URL}, nil
		}
	}

	return nil, ErrNotFound
}

// ListRemotes returns all configured remotes
func (r *Repository) ListRemotes() ([]RemoteConfig, error) {
	config, err := r.ReadConfig()
	if err != nil {
		return nil, err
	}
	return config.Remotes, nil
}

// RemoveRemote removes a remote by name
func (r *Repository) RemoveRemote(name string) error {
	config, err := r.ReadConfig()
	if err != nil {
		return err
	}

	found := false
	remotes := make([]RemoteConfig, 0, len(config.Remotes))
	for _, remote := range config.Remotes {
		if remote.Name != name {
			remotes = append(remotes, remote)
		} else {
			found = true
		}
	}

	if !found {
		return ErrNotFound
	}

	config.Remotes = remotes
	return r.WriteConfig(config)
}
