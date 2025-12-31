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

// ThreadWithContext wraps a thread with its continuation info
type ThreadWithContext struct {
	Thread       *model.Thread
	ParentThread *model.Thread   // Thread this continues from (if any)
	ChildThreads []*model.Thread // Threads that continue from this one
	ContentHash  string          // Content hash of this version (for linking)
	IsVersioned  bool            // True if this is a specific version (not latest)
	LatestCount  int             // Message count in the latest version
}

// CommitPageData contains data for the commit detail page
type CommitPageData struct {
	Title       string
	RepoPath    string
	RepoName    string
	Commit      *model.TinCommit
	Threads     []ThreadWithContext
	CodeHostURL *git.CodeHostURL
}

// ThreadVersionInfo describes a version of a thread and which commits reference it
type ThreadVersionInfo struct {
	ContentHash  string
	MessageCount int
	Commits      []*model.TinCommit
	IsCurrent    bool // True if this is the currently displayed version
	IsLatest     bool // True if this is the latest version
}

// ThreadPageData contains data for the thread detail page
type ThreadPageData struct {
	Title          string
	RepoPath       string
	RepoName       string
	Thread         *model.Thread
	ParentThread   *model.Thread   // Thread this continues from (if any)
	ChildThreads   []*model.Thread // Threads that continue from this one
	CodeHostURL    *git.CodeHostURL
	CurrentVersion string            // Content hash of currently displayed version (empty = latest)
	LatestCount    int               // Message count in latest version
	Versions       []ThreadVersionInfo // All versions of this thread
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
	// Parse: /repo/{repo-path} or /repo/{repo-path}/commit/{id} or /repo/{repo-path}/thread/{id}
	path := strings.TrimPrefix(r.URL.Path, "/repo/")
	path = strings.TrimSuffix(path, "/")

	// Check for /commit/ segment
	if idx := strings.Index(path, "/commit/"); idx != -1 {
		repoPath := path[:idx]
		commitID := path[idx+8:]
		s.handleCommit(w, r, repoPath, commitID)
		return
	}

	// Check for /thread/ segment
	if idx := strings.Index(path, "/thread/"); idx != -1 {
		repoPath := path[:idx]
		threadID := path[idx+8:]
		s.handleThread(w, r, repoPath, threadID)
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

	// Load all threads referenced in commit with their continuation context
	var threads []ThreadWithContext
	for _, ref := range commit.Threads {
		var thread *model.Thread
		var err error
		isVersioned := false

		// Try to load specific version first
		if ref.ContentHash != "" {
			thread, err = repo.LoadThreadVersion(ref.ThreadID, ref.ContentHash)
			if err == nil {
				isVersioned = true
			}
		}
		// Fall back to latest version
		if thread == nil || err != nil {
			thread, err = repo.LoadThread(ref.ThreadID)
		}
		if err == nil {
			twc := ThreadWithContext{
				Thread:      thread,
				ContentHash: ref.ContentHash,
				IsVersioned: isVersioned,
			}

			// Load latest version to get current message count
			if isVersioned {
				if latest, latestErr := repo.LoadThread(ref.ThreadID); latestErr == nil {
					twc.LatestCount = len(latest.Messages)
				}
			}

			// Load parent thread if this is a continuation
			if thread.ParentThreadID != "" {
				twc.ParentThread, _ = repo.LoadThread(thread.ParentThreadID)
			}

			// Find any threads that continue from this one
			twc.ChildThreads, _ = repo.FindChildThreads(thread.ID)

			threads = append(threads, twc)
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

// handleThread displays a single thread with its full conversation
func (s *WebServer) handleThread(w http.ResponseWriter, r *http.Request, repoPath, threadID string) {
	absPath := filepath.Join(s.rootPath, repoPath)

	repo, err := openRepo(absPath)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Load the latest version first
	latestThread, err := repo.LoadThread(threadID)
	if err != nil {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}
	latestCount := len(latestThread.Messages)

	// Check if a specific version was requested
	requestedVersion := r.URL.Query().Get("version")
	thread := latestThread
	if requestedVersion != "" {
		if versionedThread, vErr := repo.LoadThreadVersion(threadID, requestedVersion); vErr == nil {
			thread = versionedThread
		}
	}

	// Build version info: find all versions and which commits reference them
	var versions []ThreadVersionInfo
	versionHashes, _ := repo.ListThreadVersions(threadID)
	commits, _ := repo.ListCommits()

	// Build a map of content_hash -> commits that reference it
	hashToCommits := make(map[string][]*model.TinCommit)
	for _, commit := range commits {
		for _, ref := range commit.Threads {
			if ref.ThreadID == threadID && ref.ContentHash != "" {
				hashToCommits[ref.ContentHash] = append(hashToCommits[ref.ContentHash], commit)
			}
		}
	}

	// Create version info for each version that is referenced by at least one commit
	for _, hash := range versionHashes {
		if commitList, ok := hashToCommits[hash]; ok && len(commitList) > 0 {
			vThread, vErr := repo.LoadThreadVersion(threadID, hash)
			if vErr != nil {
				continue
			}
			versions = append(versions, ThreadVersionInfo{
				ContentHash:  hash,
				MessageCount: len(vThread.Messages),
				Commits:      commitList,
				IsCurrent:    hash == requestedVersion || (requestedVersion == "" && hash == latestThread.ComputeContentHash()),
				IsLatest:     hash == latestThread.ComputeContentHash(),
			})
		}
	}

	// Sort versions by message count (ascending)
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i].MessageCount > versions[j].MessageCount {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	// Load parent thread if this is a continuation
	var parentThread *model.Thread
	if thread.ParentThreadID != "" {
		parentThread, _ = repo.LoadThread(thread.ParentThreadID)
	}

	// Find any threads that continue from this one
	childThreads, _ := repo.FindChildThreads(thread.ID)

	// Try to detect code host URL from config or git remote
	var codeHostURL *git.CodeHostURL
	if remoteURL := repo.GetCodeHostURL(); remoteURL != "" {
		codeHostURL = git.ParseGitRemoteURL(remoteURL)
	}

	data := ThreadPageData{
		Title:          "Thread " + threadID[:7],
		RepoPath:       repoPath,
		RepoName:       filepath.Base(repoPath),
		Thread:         thread,
		ParentThread:   parentThread,
		ChildThreads:   childThreads,
		CodeHostURL:    codeHostURL,
		CurrentVersion: requestedVersion,
		LatestCount:    latestCount,
		Versions:       versions,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderTemplate(w, "thread.html", data); err != nil {
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
