# Git Commit Logic in `tin commit`

This document explains how `tin commit` handles git staging and committing.

## Overview

When you run `tin commit`, tin creates both a **tin commit** (stored in `.tin/commits/`) and optionally a **git commit** to capture file changes. The git commit logic handles two distinct scenarios based on thread state.

## The Two Scenarios

### Scenario 1: In-Progress Threads (no git hash yet)

When a thread has `GitCommitHash == ""`, it means the thread is still in progress and hasn't been associated with a git commit yet.

**Flow:**
1. `commitThreadChanges()` is called
2. Gets all changed files via `git status`
3. Stages them with `git add`
4. Creates a git commit with message: `[tin {thread_id}] {first human message}`
5. Stores the resulting git hash in `thread.GitCommitHash`

**Note:** This happens BEFORE the tin commit is created, so the tin commit URL cannot be included in this git commit message.

### Scenario 2: Previously Committed Threads (has git hash)

When a thread already has a `GitCommitHash` (from a previous `tin commit` or from session completion), we skip `commitThreadChanges()` and use the main commit path.

**Flow:**
1. Stage any changed files via `git add`
2. Check if there are staged changes
3. Create git commit with message: `[tin {tin_commit_id}] {message}` + tin commit URL

## Why Two Paths?

This design exists due to a chicken-and-egg problem:

| Need | Requires |
|------|----------|
| Tin commit needs a git hash | Git commit must exist first |
| Git commit should include tin URL | Tin commit must exist first |

For in-progress threads, we prioritize getting the git hash first (so the tin commit can reference it), sacrificing the URL in that git commit.

For already-committed threads, the tin commit can be created first (using the existing git hash), then a new git commit can include the URL.

## Preventing Duplicate Commits

The `threadCommitCreated` flag tracks whether `commitThreadChanges()` created a git commit. This prevents creating a redundant empty git commit just for the URL when file changes were already committed.

```
shouldCreateGitCommit = hasGitChanges || (commitURL != "" && !threadCommitCreated)
```

## File Staging

Both paths stage files before committing:

- **`commitThreadChanges()`**: Calls `GitGetChangedFiles()` then `GitAdd()`
- **Main path**: Also calls `GitGetChangedFiles()` then `GitAdd()`

This ensures file changes are captured regardless of which path executes.

## Commit Message Formats

### From `commitThreadChanges()` (in-progress threads)
```
[tin {thread_id_short}] {first line of first human message}

{rest of message if multi-line}
```

### From main path (completed threads)
```
[tin {tin_commit_id_short}] {first line of commit message}

{rest of message if multi-line}

<https://your-tin-server.com/repo/path/commit/{tin_commit_id}>
```

## Code Location

All this logic lives in `internal/commands/commit.go`:

- Lines 54-71: `commitThreadChanges()` call for in-progress threads
- Lines 138-176: Main git commit path with file staging
- `commitThreadChanges()` function: Lines 231-260
- `formatThreadGitMessage()`: Lines 263-292
- `formatGitCommitMessage()`: Lines 295-337
