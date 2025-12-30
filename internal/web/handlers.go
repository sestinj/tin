package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/danieladler/tin/internal/git"
	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

// Page data structures

// IndexPageData contains data for the landing page
type IndexPageData struct {
	Title    string
	RootPath string
	Repos    []RepoInfo
}

// BranchInfo contains branch metadata for display
type BranchInfo struct {
	Name      string
	IsCurrent bool
	CommitID  string
}

// RepoPageData contains data for the repository page
type RepoPageData struct {
	Title          string
	RepoPath       string
	RepoName       string
	Branches       []BranchInfo
	SelectedBranch string
	Commits        []*model.TinCommit
	CodeHostURL    *git.CodeHostURL
}

// CommitPageData contains data for the commit detail page
type CommitPageData struct {
	Title       string
	RepoPath    string
	RepoName    string
	Commit      *model.TinCommit
	Threads     []*model.Thread
	CodeHostURL *git.CodeHostURL
}

// handleIndex handles the landing page showing all repositories
func (s *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	repos, err := DiscoverRepos(s.rootPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := IndexPageData{
		Title:    "Repositories",
		RootPath: s.rootPath,
		Repos:    repos,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleRepo routes repository requests to the appropriate handler
func (s *WebServer) handleRepo(w http.ResponseWriter, r *http.Request) {
	// Parse: /repo/{repo-path} or /repo/{repo-path}/commit/{id}
	path := strings.TrimPrefix(r.URL.Path, "/repo/")
	path = strings.TrimSuffix(path, "/")

	// Check for /commit/ segment
	if idx := strings.Index(path, "/commit/"); idx != -1 {
		repoPath := path[:idx]
		commitID := path[idx+8:]
		s.handleCommit(w, r, repoPath, commitID)
		return
	}

	// Handle repo page
	s.handleRepoPage(w, r, path)
}

// handleRepoPage displays a repository's branches and commits
func (s *WebServer) handleRepoPage(w http.ResponseWriter, r *http.Request, repoPath string) {
	absPath := filepath.Join(s.rootPath, repoPath)

	repo, err := openRepo(absPath)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Get branch from query or HEAD
	selectedBranch := r.URL.Query().Get("branch")
	if selectedBranch == "" {
		selectedBranch, _ = repo.ReadHead()
	}

	// Get branches
	branchNames, _ := repo.ListBranches()
	branches := make([]BranchInfo, len(branchNames))
	for i, name := range branchNames {
		commitID, _ := repo.ReadBranch(name)
		branches[i] = BranchInfo{
			Name:      name,
			IsCurrent: name == selectedBranch,
			CommitID:  commitID,
		}
	}

	// Get commits for selected branch
	var commits []*model.TinCommit
	branchCommitID, _ := repo.ReadBranch(selectedBranch)
	if branchCommitID != "" {
		commits, _ = repo.GetCommitHistory(branchCommitID, 50)
	}

	// Try to detect code host URL from config or git remote
	var codeHostURL *git.CodeHostURL
	if remoteURL := repo.GetCodeHostURL(); remoteURL != "" {
		codeHostURL = git.ParseGitRemoteURL(remoteURL)
	}

	data := RepoPageData{
		Title:          repoPath,
		RepoPath:       repoPath,
		RepoName:       filepath.Base(repoPath),
		Branches:       branches,
		SelectedBranch: selectedBranch,
		Commits:        commits,
		CodeHostURL:    codeHostURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderTemplate(w, "repo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleCommit displays a single commit with its full conversation
func (s *WebServer) handleCommit(w http.ResponseWriter, r *http.Request, repoPath, commitID string) {
	absPath := filepath.Join(s.rootPath, repoPath)

	repo, err := openRepo(absPath)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	commit, err := repo.LoadCommit(commitID)
	if err != nil {
		http.Error(w, "Commit not found", http.StatusNotFound)
		return
	}

	// Load all threads referenced in commit
	var threads []*model.Thread
	for _, ref := range commit.Threads {
		if thread, err := repo.LoadThread(ref.ThreadID); err == nil {
			threads = append(threads, thread)
		}
	}

	// Try to detect code host URL from config or git remote
	var codeHostURL *git.CodeHostURL
	if remoteURL := repo.GetCodeHostURL(); remoteURL != "" {
		codeHostURL = git.ParseGitRemoteURL(remoteURL)
	}

	data := CommitPageData{
		Title:       "Commit " + commitID[:7],
		RepoPath:    repoPath,
		RepoName:    filepath.Base(repoPath),
		Commit:      commit,
		Threads:     threads,
		CodeHostURL: codeHostURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderTemplate(w, "commit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// openRepo tries to open a repository at the exact path given
func openRepo(path string) (*storage.Repository, error) {
	// First check if this is a bare repository (has HEAD, refs/, threads/ directly)
	// We check bare first because storage.Open walks UP the directory tree,
	// which could find a parent .tin directory instead of this repo
	if isBareRepoPath(path) {
		return storage.OpenBare(path)
	}

	// Check if this directory has a .tin subdirectory
	tinPath := filepath.Join(path, ".tin")
	if info, err := os.Stat(tinPath); err == nil && info.IsDir() {
		return storage.Open(path)
	}

	return nil, storage.ErrNotARepository
}

// isBareRepoPath checks if the path looks like a bare tin repository
func isBareRepoPath(path string) bool {
	// Check for config file (required by OpenBare)
	configPath := filepath.Join(path, "config")
	if _, err := os.Stat(configPath); err != nil {
		return false
	}

	// Check for refs directory
	refsPath := filepath.Join(path, "refs")
	if info, err := os.Stat(refsPath); err != nil || !info.IsDir() {
		return false
	}

	return true
}
