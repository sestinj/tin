---
description: Commit the current conversation thread to tin
allowed-tools: Bash(tin commit:*), Bash(tin status:*), Bash(tin add:*), Bash(git status:*), Bash(git diff:*)
argument-hint: [message]
---

IMPORTANT: In a tin repository, ALWAYS use `tin commit` instead of `git commit`. The `tin commit` command:
1. Stages all changed files to git
2. Creates a git commit with thread metadata
3. Creates a tin commit linking the conversation thread
4. Updates thread status to committed

This ensures the conversation context is preserved alongside code changes.

## Workflow

1. First, check what will be committed:
   ```
   tin status
   git status
   ```

2. If there are unstaged threads, stage them:
   ```
   tin add --all
   ```

3. Commit with the provided message or ask for one:
   - If $ARGUMENTS is provided: `tin commit -m "$ARGUMENTS"`
   - If no message provided: show status and ask user for a commit message

## Example

```bash
tin status          # See staged threads
git diff --stat     # See code changes
tin add --all       # Stage any unstaged threads
tin commit -m "Add authentication feature"
```
