package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sestinj/tin/internal/model"
)

const (
	TinDir            = ".tin"
	HeadFile          = "HEAD"
	ConfigFile        = "config"
	IndexFile         = "index.json"
	ThreadsDir        = "threads"
	ThreadVersionsDir = "thread-versions"
	CommitsDir        = "commits"
	RefsDir           = "refs"
	HeadsDir          = "heads"
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

// CredentialEntry represents stored credentials for a host
type CredentialEntry struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

// Config holds tin configuration
type Config struct {
	Version       int               `json:"version"`
	Remotes       []RemoteConfig    `json:"remotes,omitempty"`
	CodeHostURL   string            `json:"code_host_url,omitempty"`
	ThreadHostURL string            `json:"thread_host_url,omitempty"` // Base URL for tin web viewer (e.g., https://tin.example.com)
	AuthToken     string            `json:"auth_token,omitempty"`      // Deprecated: use Credentials instead
	Credentials   []CredentialEntry `json:"credentials,omitempty"`     // Per-host authentication tokens
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
		filepath.Join(tinPath, ThreadVersionsDir),
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

// GitCheckoutBranch switches to an existing git branch
func (r *Repository) GitCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %s", string(output))
	}
	return nil
}

// GitCreateBranch creates a new git branch at the current commit (without switching)
func (r *Repository) GitCreateBranch(name string) error {
	cmd := exec.Command("git", "branch", name)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch failed: %s", string(output))
	}
	return nil
}

// GitCreateAndCheckoutBranch creates a new git branch and switches to it
func (r *Repository) GitCreateAndCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %s", string(output))
	}
	return nil
}

// GitBranchExists checks if a git branch exists
func (r *Repository) GitBranchExists(name string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = r.RootPath
	return cmd.Run() == nil
}

// GitDeleteBranch deletes a git branch
func (r *Repository) GitDeleteBranch(name string) error {
	cmd := exec.Command("git", "branch", "-d", name)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch -d failed: %s", string(output))
	}
	return nil
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

// GitCommitEmpty creates an empty git commit (no file changes) with the given message
func (r *Repository) GitCommitEmpty(message string) error {
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", message)
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

// GitHasUnstagedChanges returns true if there are modified files in the working directory
// that are NOT staged for commit.
func (r *Repository) GitHasUnstagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	cmd.Dir = r.RootPath
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, err
	}
	return false, nil
}

// GitGetChangedFiles returns all modified, untracked, and staged files.
// Uses git status --porcelain which respects .gitignore.
// Excludes .tin/ directory files.
func (r *Repository) GitGetChangedFiles() ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 3 {
			continue
		}
		// git status --porcelain format: XY filename
		// X = staging area status, Y = working tree status
		// Filename starts at position 3
		file := line[3:]
		// Handle renamed files: "R  old -> new"
		if idx := strings.Index(file, " -> "); idx != -1 {
			file = file[idx+4:]
		}
		// Git wraps filenames with special characters in quotes - strip them
		if len(file) >= 2 && file[0] == '"' && file[len(file)-1] == '"' {
			file = file[1 : len(file)-1]
		}
		// Exclude .tin/ directory
		if !strings.HasPrefix(file, ".tin/") && file != ".tin" {
			files = append(files, file)
		}
	}
	return files, nil
}

// GitPush runs git push with the given remote and branch
func (r *Repository) GitPush(remote, branch string, force bool) error {
	args := []string{"push", remote, branch}
	if force {
		args = []string{"push", "--force", remote, branch}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %s", string(output))
	}
	return nil
}

// GitPull runs git pull with the given remote and branch
func (r *Repository) GitPull(remote, branch string) error {
	cmd := exec.Command("git", "pull", remote, branch)
	cmd.Dir = r.RootPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %s", string(output))
	}
	return nil
}

// GetGitRemoteURL returns the URL of a git remote
func (r *Repository) GetGitRemoteURL(name string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", name)
	cmd.Dir = r.RootPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCodeHostURL returns the code host URL, checking config first then git remote
func (r *Repository) GetCodeHostURL() string {
	// Check config first (explicit override)
	if config, err := r.ReadConfig(); err == nil && config.CodeHostURL != "" {
		return config.CodeHostURL
	}
	// Fall back to git remote (only works for non-bare repos)
	if !r.IsBare {
		if url, err := r.GetGitRemoteURL("origin"); err == nil {
			return url
		}
	}
	return ""
}

// GitGetAuthor returns the git author in "Name <email>" format
func (r *Repository) GitGetAuthor() string {
	nameCmd := exec.Command("git", "config", "user.name")
	nameCmd.Dir = r.RootPath
	nameOutput, nameErr := nameCmd.Output()

	emailCmd := exec.Command("git", "config", "user.email")
	emailCmd.Dir = r.RootPath
	emailOutput, emailErr := emailCmd.Output()

	name := strings.TrimSpace(string(nameOutput))
	email := strings.TrimSpace(string(emailOutput))

	if nameErr != nil && emailErr != nil {
		return ""
	}
	if name == "" && email == "" {
		return ""
	}
	if email == "" {
		return name
	}
	if name == "" {
		return fmt.Sprintf("<%s>", email)
	}
	return fmt.Sprintf("%s <%s>", name, email)
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
		filepath.Join(tinPath, ThreadVersionsDir),
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

// GetThreadHostURL returns the base URL for the tin web viewer.
// Checks config first, then derives from origin remote URL.
func (r *Repository) GetThreadHostURL() string {
	// Check explicit config first
	if config, err := r.ReadConfig(); err == nil && config.ThreadHostURL != "" {
		return strings.TrimSuffix(config.ThreadHostURL, "/")
	}

	// Try to derive from origin remote
	remote, err := r.GetRemote("origin")
	if err != nil {
		return ""
	}

	return deriveThreadHostURL(remote.URL)
}

// deriveThreadHostURL converts a tin remote URL to a web viewer base URL
// Example: "localhost:2323/myproject.tin" -> "http://localhost:2323"
// Example: "tin.example.com:2323/repos/project.tin" -> "https://tin.example.com:2323"
func deriveThreadHostURL(remoteURL string) string {
	// Handle tin:// scheme
	if strings.HasPrefix(remoteURL, "tin://") {
		remoteURL = strings.TrimPrefix(remoteURL, "tin://")
	}

	// Find the path separator to get host:port
	slashIdx := strings.Index(remoteURL, "/")
	if slashIdx == -1 {
		return ""
	}

	hostPort := remoteURL[:slashIdx]

	// Use http for localhost, https otherwise
	scheme := "https"
	if strings.HasPrefix(hostPort, "localhost") || strings.HasPrefix(hostPort, "127.0.0.1") {
		scheme = "http"
	}

	return scheme + "://" + hostPort
}

// BuildCommitURL constructs a URL to view a tin commit in the web viewer.
// Returns empty string if no thread host is configured or derivable.
func (r *Repository) BuildCommitURL(commitID string) string {
	baseURL := r.GetThreadHostURL()
	if baseURL == "" {
		return ""
	}

	// Get the repo path from the origin remote
	remote, err := r.GetRemote("origin")
	if err != nil {
		return ""
	}

	repoPath := extractRepoPath(remote.URL)
	if repoPath == "" {
		return ""
	}

	return fmt.Sprintf("%s/repo/%s/commit/%s", baseURL, repoPath, commitID)
}

// BuildThreadURL constructs a URL to view a thread in the web viewer.
// Returns empty string if no thread host is configured or derivable.
func (r *Repository) BuildThreadURL(threadID, contentHash string) string {
	baseURL := r.GetThreadHostURL()
	if baseURL == "" {
		return ""
	}

	// Get the repo path from the origin remote
	remote, err := r.GetRemote("origin")
	if err != nil {
		return ""
	}

	repoPath := extractRepoPath(remote.URL)
	if repoPath == "" {
		return ""
	}

	url := fmt.Sprintf("%s/repo/%s/thread/%s", baseURL, repoPath, threadID)
	if contentHash != "" {
		url += "?version=" + contentHash
	}
	return url
}

// extractRepoPath gets the repository path from a remote URL
// Example: "localhost:2323/myproject.tin" -> "myproject.tin"
func extractRepoPath(remoteURL string) string {
	// Handle tin:// scheme
	if strings.HasPrefix(remoteURL, "tin://") {
		remoteURL = strings.TrimPrefix(remoteURL, "tin://")
	}

	// Find the path after host:port
	slashIdx := strings.Index(remoteURL, "/")
	if slashIdx == -1 {
		return ""
	}

	path := remoteURL[slashIdx+1:]
	return strings.TrimPrefix(path, "/")
}
