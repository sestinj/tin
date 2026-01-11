---
description: Commit the current conversation thread to tin
allowed-tools: Bash(tin commit:*), Bash(tin status:*)
argument-hint: [message]
---

Commit the staged tin threads.

If $ARGUMENTS is provided, use it as the commit message:
```
tin commit -m "$ARGUMENTS"
```

If no message is provided, first run `tin status` to show what will be committed, then ask the user for a commit message before running the commit.
