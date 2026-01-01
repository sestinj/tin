# Tin Command Reference

## Core Commands

### tin init

Initialize a new tin repository.

```
tin init [options] [path]
```

**Options:**
- `--bare` - Create a bare repository (for use as a remote)

**Examples:**
```bash
tin init                           # Initialize in current directory
tin init --bare /path/to/repo.tin  # Create bare repo for remote use
```

---

### tin status

Show the current state of the repository.

```
tin status
```

Displays:
- Current branch and latest commit
- Threads staged for commit
- Unstaged threads
- Active threads (conversations in progress)

---

### tin branch

List, create, or delete branches.

```
tin branch [options] [name]
```

**Options:**
- `-a, --all` - Show branches with their commit info
- `-d, --delete <name>` - Delete a branch

**Examples:**
```bash
tin branch                 # List all branches
tin branch feature-auth    # Create a new branch
tin branch -a              # List with commit info
tin branch -d old-branch   # Delete a branch
```

---

### tin checkout

Switch branches or restore working tree.

```
tin checkout [options] <branch|commit>
```

**Options:**
- `-b` - Create a new branch and switch to it

**Examples:**
```bash
tin checkout main            # Switch to main branch
tin checkout -b new-feature  # Create and switch to new branch
tin checkout abc123          # Checkout a specific commit
```

---

### tin merge

Merge a branch into the current branch, combining both git history and thread history.

```
tin merge <branch>
tin merge --continue
tin merge --abort
```

**Options:**
- `--continue` - Complete merge after resolving git conflicts
- `--abort` - Cancel an in-progress merge

**Thread Conflict Handling:**

If the same thread exists on both branches with different content, both versions are kept. The source branch's version is renamed with a suffix (e.g., `thread-id_from_feature-branch`).

**Git Conflict Handling:**

If git encounters merge conflicts, the merge will pause. Resolve the conflicts in your editor, then run `tin merge --continue` to complete the merge, or `tin merge --abort` to cancel.

**Examples:**
```bash
tin merge feature-auth      # Merge feature-auth into current branch
tin merge --continue        # Complete paused merge after conflict resolution
tin merge --abort           # Cancel in-progress merge
```

---

### tin add

Stage threads for commit.

```
tin add [options] <thread-id>...
```

**Options:**
- `-A, --all, .` - Stage all unstaged threads

**Arguments:**
- `<thread-id>` - Thread ID or prefix to stage
- `<thread-id>@N` - Stage only first N messages of a thread

**Examples:**
```bash
tin add abc123      # Stage thread abc123
tin add abc123@10   # Stage first 10 messages only
tin add .           # Stage all unstaged threads
tin add --all       # Stage all unstaged threads
```

---

### tin commit

Record changes to the repository.

```
tin commit [-m <message>]
```

**Options:**
- `-m, --message <msg>` - Commit message (auto-generated if not provided)

**Examples:**
```bash
tin commit                               # Auto-generate message from thread
tin commit -m "Add user authentication"  # Use explicit message
```

---

### tin log

Show commit history.

```
tin log [options]
```

**Options:**
- `-n <number>` - Limit to last n commits (default: 10)
- `--all` - Show all commits

**Examples:**
```bash
tin log          # Show last 10 commits
tin log -n 5     # Show last 5 commits
tin log --all    # Show all commits
```

---

## Thread Commands

### tin thread list

List all threads.

```
tin thread list
```

---

### tin thread show

Show details of a thread.

```
tin thread show <id>
```

---

### tin thread start

Start a new thread (typically used by hooks).

```
tin thread start [options]
```

**Options:**
- `--agent <name>` - Agent name (e.g., claude-code)
- `--session-id <id>` - Agent session ID

---

### tin thread append

Append a message to a thread (typically used by hooks).

```
tin thread append [options]
```

**Options:**
- `--thread <id>` - Thread ID (required)
- `--role <role>` - Message role: human or assistant (required)
- `--content <text>` - Message content (required)
- `--git-hash <hash>` - Git commit hash after this message
- `--tool-calls <json>` - Tool calls as JSON array

---

### tin thread complete

Mark a thread as completed.

```
tin thread complete <id>
```

---

### tin thread delete

Delete a thread.

```
tin thread delete [options] <id>
```

**Options:**
- `-f, --force` - Force deletion of active or committed threads

---

## Hooks Commands

### tin hooks install

Install hooks for Claude Code.

```
tin hooks install [options]
```

**Options:**
- `-g, --global` - Install to global settings (~/.claude/) instead of project

Installs hooks that automatically:
- Track conversation sessions as threads
- Record human prompts and assistant responses
- Capture tool calls and git state changes
- Auto-stage threads for easy committing

Also installs slash commands: `/branches`, `/commit`, `/checkout`

---

### tin hooks uninstall

Remove hooks from Claude Code.

```
tin hooks uninstall [options]
```

**Options:**
- `-g, --global` - Remove from global settings

---

## Remote Commands

### tin remote

Manage remote repositories.

```
tin remote [command]
```

**Subcommands:**
- `(none)` - List all remotes
- `add <name> <url>` - Add a remote
- `remove <name>` - Remove a remote

**Examples:**
```bash
tin remote
tin remote add origin localhost:2323/myproject.tin
tin remote remove origin
```

---

### tin push

Push commits and threads to a remote repository.

```
tin push [options] [remote] [branch]
```

**Options:**
- `-f, --force` - Force push (overwrite remote)

**Arguments:**
- `remote` - Remote name (default: origin)
- `branch` - Branch to push (default: current branch)

**Examples:**
```bash
tin push
tin push origin main
tin push --force origin main
```

---

### tin pull

Pull commits and threads from a remote repository.

```
tin pull [remote] [branch]
```

**Arguments:**
- `remote` - Remote name (default: origin)
- `branch` - Branch to pull (default: current branch)

**Examples:**
```bash
tin pull
tin pull origin main
```

---

### tin serve

Start a tin server for push/pull operations or web viewing.

```
tin serve [options] [repo-path]
```

**Options:**
- `--host <host>` - Host to bind to (default: localhost)
- `--port, -p <n>` - Port to listen on (default: 2323)
- `--repo, -r <path>` - Path to a single bare repository
- `--root <path>` - Serve any repository under this directory (auto-creates on push)
- `--web` - Start HTML web viewer instead of push/pull server (requires --root)

**Examples:**
```bash
# Multi-repo server (recommended)
tin serve --root ~/tin-repos

# Single-repo server
tin serve /path/to/repo.tin

# Web viewer
tin serve --web --root ~/projects --port 8080

# Production setup
tin serve --host 0.0.0.0 --port 2323 --root /var/tin-repos
```

---

## Configuration Commands

### tin config

View and modify tin configuration.

```
tin config [command]
```

**Subcommands:**
- `(none), list` - Show all configuration values
- `get <key>` - Get a specific value
- `set <key> <value>` - Set a value

**Available keys:**
- `thread_host_url` - Base URL for tin web viewer
- `code_host_url` - URL for code repository (e.g., GitHub URL)

**Examples:**
```bash
tin config                                      # List all config
tin config get thread_host_url                  # Get value
tin config set thread_host_url http://localhost:8080
tin config set code_host_url https://github.com/user/repo
```

---

## Global Options

All commands support:
- `-h, --help` - Show help for the command
- `-v, --version` - Show version (top-level only)
