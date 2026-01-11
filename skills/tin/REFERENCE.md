# Tin Reference Documentation

## Full Command Reference

### Status and Info

```bash
tin status                     # Show staged threads + branch state
tin branch                     # List branches (* marks current)
tin log                        # Show commit history with threads
```

### Committing

```bash
tin commit -m "message"        # Commit with explicit message
tin push                       # Push BOTH git and tin to remote
```

### Branch Operations

```bash
tin checkout feature-branch    # Switch branches (git + tin)
tin checkout -b new-feature    # Create and switch to new branch
tin branch new-feature         # Create branch without switching
```

### Thread Management

Threads are usually auto-staged by hooks, but can be managed manually:

```bash
tin add .                      # Stage all unstaged threads
tin add abc123                 # Stage specific thread
tin add abc123@10              # Stage first N messages only
tin add --unstage abc123       # Unstage a thread
```

### Syncing

```bash
tin sync --tin-follows-git     # Sync tin HEAD to match git branch
tin pull                       # Pull threads from remote
```

### Thread Viewing

```bash
tin thread list                # List all threads
tin thread show <id>           # Display thread conversation
tin thread delete <id>         # Delete a thread (use -f for committed/active)
```

### Hooks

```bash
tin hooks install              # Install Claude Code hooks (project)
tin hooks install --global     # Install hooks globally (~/.claude)
tin hooks uninstall            # Remove hooks
```

### Amp Integration

```bash
tin amp pull                   # Pull latest thread from Amp
tin amp pull 5                 # Pull 5 most recent threads
tin amp pull T-019b7d09-...    # Pull specific thread by ID
```

## Data Model

### Message

Each message in a thread contains:
- `hash` - Merkle chain hash for integrity
- `role` - human or assistant
- `content` - Message text
- `timestamp` - When the message was created
- `tool_calls` - Any tool invocations (optional)
- `git_hash_after` - Git commit hash after this message (optional)

### Thread

A thread is a conversation:
- `id` - First message hash (unique identifier)
- `agent` - Which AI agent (claude-code, amp, etc.)
- `messages[]` - Array of messages
- `parent` - Parent thread reference for branching (optional)

### TinCommit

A tin commit links threads to git:
- `id` - Commit hash
- `message` - Commit message
- `threads[]` - Array of thread references with message counts
- `git_commit_hash` - Corresponding git commit
- `timestamp` - When committed
- `author` - Who made the commit

## Storage Structure

Tin stores data in `.tin/` directory:

```
.tin/
├── config              # Remote configuration
├── HEAD                # Current branch name
├── index.json          # Staged threads
├── threads/            # Thread JSON files
├── commits/            # Commit JSON files
└── refs/
    └── heads/          # Branch references
```

## Claude Code Hooks

When installed, tin hooks auto-track conversations:

| Hook | Action |
|------|--------|
| `SessionStart` | Create new thread |
| `UserPromptSubmit` | Append human message, auto-stage |
| `Stop` | Append assistant response + tool calls + git hash |
| `SessionEnd` | Mark thread complete |

## Advanced Troubleshooting

### Branch Mismatch After Git Checkout

If you accidentally used `git checkout` instead of `tin checkout`:

```bash
tin sync --tin-follows-git
```

This syncs tin's HEAD to match git's current branch.

### Recovering from Corrupted State

```bash
# Check current state
tin status
ls -la .tin/
cat .tin/HEAD
cat .tin/config

# Reset HEAD if missing
echo "main" > .tin/HEAD
tin sync --tin-follows-git

# Re-initialize if needed (preserves config)
tin init
```

### Force Commit on Mismatch

If you need to commit despite branch mismatch (not recommended):

```bash
tin commit --force -m "message"
```

### Debugging Hook Issues

Check hook output:

```bash
# Hooks are in .claude/settings.json (local) or ~/.claude/settings.json (global)
cat .claude/settings.json

# Test hook manually
tin hook session-start
tin hook user-prompt
tin hook stop
tin hook session-end
```

### Resetting Remote Configuration

```bash
# Check current remote
cat .tin/config

# Remote is configured as:
# {
#   "remotes": [{"name": "origin", "url": "..."}],
#   "code_host_url": "https://github.com/...",
#   "thread_host_url": "..."
# }
```

## Why Not Fall Back to Git?

When tin fails, always fix it rather than using git directly:

1. **Breaks conversation linking** - PR reviewers lose context on why changes were made
2. **Incomplete history** - Team's searchable AI decision history becomes incomplete
3. **Quick fixes** - Most tin issues resolve in 30 seconds with `tin sync` or `tin init`

The conversation context is valuable for code review and debugging - don't lose it by falling back to raw git.
