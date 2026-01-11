---
description: Switch to a different tin branch
allowed-tools: Bash(tin checkout:*), Bash(tin branch:*)
argument-hint: [branch]
---

Switch to a different tin branch.

If $ARGUMENTS is provided, checkout that branch:
```
tin checkout $ARGUMENTS
```

If no branch name is provided, first run `tin branch` to show available branches, then ask the user which branch to checkout.
