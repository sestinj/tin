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
