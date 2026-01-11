# Tin Observations

Workflow issues, bugs, and ergonomic observations collected while test driving tin.

---

**Missing `--unstage` flag in CLI**

When trying to commit only a specific thread (while others were staged), there was no way to unstage the other threads from the CLI. The `tin add --unstage` flag is mentioned in CLAUDE.md but isn't implemented - the `UnstageThread` function exists in the storage layer but isn't exposed as a command. Had to manually edit `.tin/index.json` to unstage threads, which the documentation explicitly warns against.

Workaround: Edit `.tin/index.json` directly to remove unwanted staged threads.

Expected: `tin add --unstage <thread-id>` or a separate `tin unstage <thread-id>` command.
