# Tin

Thread-based version control for conversational coding. Wraps git, treating conversation threads as the primary unit of change.

## Quick Reference

```bash
tin init                  # Initialize (also runs git init if needed)
tin status                # Show current branch and staged threads
tin branch [name]         # List or create branches
tin checkout <ref>        # Switch branch/commit (restores git state)
tin add <thread-id>       # Stage a thread (or --all)
tin commit -m "msg"       # Commit staged threads
tin log                   # Show commit history
tin thread list           # List all threads
tin thread show <id>      # Display thread conversation
tin hooks install         # Install Claude Code hooks (--global for ~/.claude)
tin hooks uninstall       # Remove hooks
```

## Project Structure

```
cmd/tin/main.go           # Entry point
internal/
  commands/               # CLI command implementations
  model/                  # Message, Thread, TinCommit, Branch structs
  storage/                # .tin directory operations
  hooks/                  # Claude Code hook handlers + installation
```

## Data Model

- **Message**: hash (merkle chain), role, content, timestamp, tool_calls, git_hash_after
- **Thread**: id (first message hash), agent, messages[], parent refs for branching
- **TinCommit**: id, message, thread refs, git_commit_hash

Storage in `.tin/`: threads/, commits/, refs/heads/, index.json (staging), HEAD

## Claude Code Integration

Hooks auto-track conversations:
- `SessionStart` → new thread
- `UserPromptSubmit` → append human message, auto-stage
- `Stop` → append assistant response + tool calls + git hash
- `SessionEnd` → mark thread complete

Slash commands: `/branches`, `/commit [msg]`, `/checkout [branch]`

## Building

```bash
go build -o tin ./cmd/tin
```
