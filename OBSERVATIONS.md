# Tin Observations

Workflow issues, bugs, and ergonomic observations collected while test driving tin.

---

**Missing `--unstage` flag in CLI**

When trying to commit only a specific thread (while others were staged), there was no way to unstage the other threads from the CLI. The `tin add --unstage` flag is mentioned in CLAUDE.md but isn't implemented - the `UnstageThread` function exists in the storage layer but isn't exposed as a command. Had to manually edit `.tin/index.json` to unstage threads, which the documentation explicitly warns against.

Workaround: Edit `.tin/index.json` directly to remove unwanted staged threads.

Expected: `tin add --unstage <thread-id>` or a separate `tin unstage <thread-id>` command.

---

**Conversational threads have no git history**

When a thread is purely conversational (no code changes), `tin commit` creates a tin commit but no git commit. Multiple tin commits can point to the same git hash. This means:

1. These threads are invisible in `git log` - only visible via `tin log`
2. If `.tin/` isn't pushed or gets lost, there's no record these discussions happened
3. The "tin wraps git" mental model breaks down - some tin history exists outside git

This could be a problem for important design discussions, architectural decisions, or planning threads that don't directly produce code but are valuable context for understanding why code exists.

Possible solutions:
- Create empty git commits for thread-only tin commits (with thread metadata in commit message)
- Store thread references in a git-tracked manifest file
- Accept this as intentional (threads without code changes are ephemeral)

---

**Missing `tin remote set-url` subcommand**

When needing to change a remote URL (e.g., from `tinhub.dev` to an IP address), there's no `set-url` subcommand like git has. Had to remove and re-add the remote:

```bash
tin remote remove origin
tin remote add origin 52.90.157.114:2323/sestinj/tin.tin
```

Expected: `tin remote set-url origin <new-url>` to match git's interface.

---

**Credentials stored in version-controlled config file**

The `.tin/config` file is tracked in git (via `!.tin/config` in .gitignore), but credentials are stored there too. This means `tin config credentials add` would commit secrets to the repo.

Current workaround: Use `TIN_AUTH_TOKEN` environment variable instead.

Expected: Either store credentials in a separate `.tin/credentials` file that's gitignored, or in a global `~/.tin/credentials` file outside the repo.

**Update:** Fixed - credentials now stored globally in `~/.config/tin/credentials`.

---

**`tin push` assumes git and tin remote names match**

`tin push <remote> <branch>` uses the same remote name for both git and tin. But git and tin remotes are independent - you might have:
- git remote "origin" → github.com
- tin remote "tinhub" → tinhub.exe.xyz

Running `tin push tinhub main` fails because there's no git remote called "tinhub":

```
$ tin push tinhub main
Pushing git to tinhub/main...
error: git push failed: fatal: 'tinhub' does not appear to be a git repository
```

Workaround: Use matching names for git and tin remotes.

Question: Should tin support separate git/tin remote names? e.g., `tin push --git-remote origin --tin-remote tinhub main`

---

When I `tin commit` and then afterward ask a few follow up questions in those same sessions, it causes there to be unstaged threads that feel messy and aren't necessarily related to whatever next commit I make, but they will be bound to be committed along with the next set of changes

---

**Cross-repo work loses thread context**

When working on multiple repos in a single Claude Code session, the conversation thread is tracked in whichever repo the session was started from. For example:

- Session started in `sestinj/tinhub`
- Made changes to `sestinj/tin` (credential prompting feature)
- Ran `tin commit` in the tin repo
- The commit linked to an *unrelated* thread from a previous tin session

The actual conversation explaining *why* the credential prompting was implemented lives in tinhub's `.tin/` directory, not tin's. So the tin commit has no meaningful thread context.

This is a fundamental issue with how tin tracks conversations at the directory level rather than following where work actually happens.

Possible solutions:
- Support importing/referencing threads from other repos
- Allow specifying a thread ID manually during commit: `tin commit --thread <id-from-other-repo>`
- A "cross-repo thread" concept that can be referenced from multiple repos
- Accept this limitation and document it (current state)

---

**`tin sync` error message didn't show `--tin-follows-git` option**

User scenario:
```bash
tin commit -m "WIP on tasks page"
# Error: tin branch 'main' does not match git branch 'nate/tasks-page-update'

tin sync
# Error: cannot switch git branch due to uncommitted changes
```

The issue: `tin sync` defaults to "git follows tin" which tries to switch the working tree via `git checkout`. When this fails due to uncommitted changes, the error message didn't mention that `--tin-follows-git` flag exists.

User insight: "Why couldn't tin sync just change only the tin branch?"

The answer: It can! `tin sync --tin-follows-git` does exactly this—updates tin's branch pointer without touching git. But:
1. This wasn't mentioned in the error message
2. The functionality existed but wasn't discoverable

**Fixed**: Improved error message in `internal/commands/sync.go` to detect uncommitted changes errors and show all available options:
1. `tin sync --tin-follows-git` (safe, just updates pointer)
2. Stash/commit changes first
3. `tin commit --force` to proceed on mismatched branches

The improved error now guides users to the right solution instead of leaving them stuck.

---

**Tin doesn't handle git worktrees** ✅ FIXED

Git worktrees allow working on multiple branches simultaneously by creating separate working directories that share the same `.git` (via a `.git` file pointing to the main repo's `.git` directory). However, tin was treating each worktree as completely independent.

Problem:
1. `.tin/` directory exists in main repo (not version controlled)
2. Create a git worktree → no `.tin/` directory in worktree
3. Try to `tin commit` in worktree → error: "need to run tin init"

Expected behavior: Tin should detect it's in a git worktree and either:
- Share the `.tin/` directory with the main repo (like git shares `.git/`)
- Auto-detect worktree and initialize appropriately
- Give a helpful error explaining the worktree situation

Git's approach: The `.git` file in a worktree contains `gitdir: /path/to/main/.git/worktrees/<name>`, allowing git commands to work seamlessly across all worktrees.

**Solution implemented:**
- `storage.Open()` now detects when `.git` is a file (not directory), parses it to find the main repo, and uses the main repo's `.tin` directory
- `storage.Init()` similarly detects worktrees - if main repo has tin, it uses that; if not, it initializes tin in the main repo (not the worktree)
- Repository struct keeps `RootPath` as the worktree directory (for git operations) and `TinPath` pointing to the shared `.tin`
- Added comprehensive tests: `TestOpen_Worktree` and `TestInit_Worktree_ExistingTin`

Now tin seamlessly shares state across worktrees just like git does.
