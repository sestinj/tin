package web

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dadlerj/tin/internal/storage"
)

// RepoInfo contains metadata about a discovered tin repository
type RepoInfo struct {
	Path          string    // Relative path from root
	AbsPath       string    // Absolute filesystem path
	Name          string    // Display name (basename)
	CurrentBranch string    // Current HEAD branch
	LastActivity  time.Time // Most recent commit timestamp
}

// DiscoverRepos walks the root directory looking for .tin repositories
func DiscoverRepos(rootPath string) ([]RepoInfo, error) {
	var repos []RepoInfo

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}

		if !d.IsDir() {
			return nil
		}

		// Skip hidden directories (but we still check for .tin inside non-hidden dirs)
		if d.Name()[0] == '.' && path != absRoot {
			return filepath.SkipDir
		}

		// Check if this directory contains .tin (working directory with .tin subdirectory)
		tinPath := filepath.Join(path, ".tin")
		if info, err := os.Stat(tinPath); err == nil && info.IsDir() {
			// Found a tin repo
			relPath, _ := filepath.Rel(absRoot, path)
			if relPath == "." {
				relPath = filepath.Base(path)
			}

			info := buildRepoInfo(path, relPath)
			repos = append(repos, info)

			// Don't recurse into this repo's subdirectories
			return filepath.SkipDir
		}

		// Check if this directory IS a bare tin repo (has HEAD, refs/, threads/ directly)
		if isBareRepo(path) {
			relPath, _ := filepath.Rel(absRoot, path)
			if relPath == "." {
				relPath = filepath.Base(path)
			}

			info := buildRepoInfo(path, relPath)
			repos = append(repos, info)

			// Don't recurse into this repo's subdirectories
			return filepath.SkipDir
		}

		return nil
	})

	// Sort by last activity, most recent first
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].LastActivity.After(repos[j].LastActivity)
	})

	return repos, err
}

// isBareRepo checks if a directory is a bare tin repository
// (contains HEAD file and refs/ directory directly)
func isBareRepo(path string) bool {
	// Check for HEAD file
	headPath := filepath.Join(path, "HEAD")
	if _, err := os.Stat(headPath); err != nil {
		return false
	}

	// Check for refs directory
	refsPath := filepath.Join(path, "refs")
	if info, err := os.Stat(refsPath); err != nil || !info.IsDir() {
		return false
	}

	// Check for threads directory (distinguishes tin from git)
	threadsPath := filepath.Join(path, "threads")
	if info, err := os.Stat(threadsPath); err != nil || !info.IsDir() {
		return false
	}

	return true
}

func buildRepoInfo(absPath, relPath string) RepoInfo {
	info := RepoInfo{
		Path:    relPath,
		AbsPath: absPath,
		Name:    filepath.Base(absPath),
	}

	// Try to open and extract metadata (try working dir first, then bare)
	repo, err := storage.Open(absPath)
	if err != nil {
		repo, err = storage.OpenBare(absPath)
		if err != nil {
			return info
		}
	}

	// Get current branch
	if branch, err := repo.ReadHead(); err == nil {
		info.CurrentBranch = branch
	}

	// Get last activity from most recent commit
	commits, err := repo.ListCommits()
	if err == nil && len(commits) > 0 {
		info.LastActivity = commits[0].Timestamp
	}

	return info
}
