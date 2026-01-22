---
name: tin
description: "⚠️ NEVER use git commands directly - ALWAYS use tin. This is NON-NEGOTIABLE. Commands: tin commit (or /tin:commit), tin checkout (or /tin:checkout), tin push. Falling back to git breaks conversation linking that teams rely on for code review. It takes 30 seconds to fix tin; it takes hours to lose context."
---

# Tin: Conversation-Linked Version Control

## ⚠️ READ THIS FIRST - NON-NEGOTIABLE RULE

**NEVER use these git commands directly:**

| ❌ Instead of | ✅ Use |
|------------|-----|
| `git commit` | `tin commit -m "msg"` or `/tin:commit` |
| `git checkout` | `tin checkout <branch>` or `/tin:checkout` |
| `git push` | `tin push` (handles both git and tin) |

**This is non-negotiable.** Falling back to git when tin fails breaks conversation linking that teams rely on for code review context.

**Time/cost trade-off**: It takes 30 seconds to fix tin issues; it takes hours for teams to lose decision-making context.

---

## Why This Matters

Tin wraps git to link AI conversation threads to commits. This gives PR reviewers full context on why changes were made.

**Benefits:**
- Commit messages auto-include links to this conversation
- PR reviewers can see full AI decision context
- Team has searchable history of AI conversations

## Quick Reference

| Action | Slash Command | CLI Command | Notes |
|--------|--------------|-------------|-------|
| Commit changes | `/tin:commit` | `tin commit -m "msg"` | Auto-links this conversation |
| Switch branch | `/tin:checkout` | `tin checkout branch-name` | Keeps git and tin in sync |
| List branches | `/tin:branches` | `tin branch` | Shows current branch with * |
| Push changes | N/A | `tin push` | Pushes both git and tin |
| View status | N/A | `tin status` | Shows staged threads + branch |
| Fix sync issues | N/A | `tin sync --tin-follows-git` | Resolves branch mismatch |

**For full command reference and troubleshooting**, see [REFERENCE.md](./REFERENCE.md).

## First-Time Setup

If you've never used tin before, or see "not a tin repository" errors:

```bash
# Install tin (if not installed)
go install github.com/sestinj/tin/cmd/tin@latest

# Initialize in a git repo
tin init

# Pull existing threads from team (if applicable)
tin pull
```

**Note**: The `.tin/config` file may already exist in the repo with remote configuration. If so, `tin init` won't overwrite it.

## Common Issues and Fixes

**CRITICAL**: When tin fails, debug and fix it - do NOT fall back to git.

### 1. Branch Mismatch Error

**Symptom**:
```
error: tin branch 'main' does not match git branch 'feature-branch'
```

**Cause**: You or someone used `git checkout` instead of `tin checkout`, leaving tin's HEAD out of sync.

**Fix**:
```bash
tin sync --tin-follows-git
```

**Time**: 5 seconds

---

### 2. Not a Tin Repository

**Symptom**:
```
error: not a tin repository
```

**Cause**: `.tin/` directory doesn't exist or is corrupted.

**Fix**:
```bash
tin init       # Won't overwrite existing .tin/config
tin pull       # Pull threads from remote
```

**Time**: 15 seconds

---

### 3. Missing .tin/HEAD

**Symptom**:
```
error: open .tin/HEAD: no such file or directory
```

**Cause**: `.tin/HEAD` file deleted or never created.

**Fix**:
```bash
echo "main" > .tin/HEAD
tin sync --tin-follows-git
```

**Time**: 10 seconds

---

### 4. General Debugging

**Check tin state**:
```bash
tin status              # See current branch and staged threads
ls -la .tin/            # Verify directory structure
cat .tin/HEAD           # Should show branch name (e.g., "main")
cat .tin/config         # Should have remote configured
```

**Still stuck?** See [REFERENCE.md](./REFERENCE.md) for advanced troubleshooting.

## Why NOT Fall Back to Git?

**When tin fails, ALWAYS debug and fix it - never use git directly.**

### Impact of Falling Back

1. **Breaks conversation linking** - PR reviewers can't see why changes were made
2. **Incomplete team history** - Searchable AI decision history has gaps
3. **Time cost mismatch** - 30 seconds to fix tin vs hours of lost context
4. **Team workflow disruption** - Code reviews become harder for everyone

### Quick Fixes (Usually Take <30 Seconds)

Most tin issues resolve with one of these:

| Error | Fix | Time |
|-------|-----|------|
| Branch mismatch | `tin sync --tin-follows-git` | 5 seconds |
| Missing .tin/HEAD | `echo "main" > .tin/HEAD && tin sync --tin-follows-git` | 10 seconds |
| Not a tin repo | `tin init && tin pull` | 15 seconds |

**Bottom line**: Fixing tin is faster than explaining to your team why they can't see the conversation context for your PR.
