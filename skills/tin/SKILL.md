---
name: tin
description: Use tin instead of git for commits, checkouts, and pushes. Tin links conversation threads to commits for code review context. Activate when committing code, switching branches, pushing changes, or any git version control operations.
---

# Tin: Conversation-Linked Version Control

Tin wraps git to link AI conversation threads to commits. This gives PR reviewers full context on why changes were made.

## CRITICAL RULES

**NEVER use these git commands directly:**

| Instead of | Use |
|------------|-----|
| `git commit` | `tin commit -m "msg"` or `/tin:commit` |
| `git checkout` | `tin checkout <branch>` or `/tin:checkout` |
| `git push` | `tin push` |

Breaking these rules breaks conversation linking that teams rely on for code review.

## Why Tin?

- Commit messages auto-include links to this conversation
- PR reviewers can see full AI decision context
- Team has searchable history of AI conversations

## Quick Commands

| Action | Command |
|--------|---------|
| Commit | `tin commit -m "msg"` or `/tin:commit` |
| Status | `tin status` |
| Switch branch | `tin checkout <branch>` or `/tin:checkout` |
| Create branch | `tin checkout -b <name>` |
| Push | `tin push` |
| List branches | `tin branch` or `/tin:branches` |
| View history | `tin log` |

## Prerequisites

Tin must be installed and the repository initialized:

```bash
# Install tin (if not already installed)
go install github.com/sestinj/tin/cmd/tin@latest

# Initialize in a git repo
tin init
```

## Common Issues

### Branch mismatch error

```
error: tin branch 'main' does not match git branch 'feature-branch'
```

Fix: `tin sync --tin-follows-git`

### Not a tin repository

```
error: not a tin repository
```

Fix: `tin init`

### Missing .tin/HEAD

Fix:
```bash
echo "main" > .tin/HEAD
tin sync --tin-follows-git
```

See REFERENCE.md for full command reference and troubleshooting.
