<p align="center">
  <img src="assets/tin-mascot.jpeg" alt="tin mascot" width="200">
</p>

# tin

[![License: Apache](https://img.shields.io/badge/license-Apache%20License%202.0-blue)](https://opensource.org/licenses/Apache-2.0) 

`tin` is git for AI coding conversations.

Today, all real design work happens in conversations with AI agents, and those conversations disappear into local caches. `tin` turns those threads into first-class, version-controlled artifacts, permanently linked to the code they produced.

`tin` fully wraps git, treating conversation threads as the primary unit of change and linking them to git commits.

> [!WARNING]
> This is a proof of concept, 100% vibe coded (this README.md is the only human-edited file in this repo!), and may not be suitable for all agentic coding workflows. See [contributing](#contributing) below if you want to support this project.

## Highlights

- **Thread-based version control**: `tin` is a version control protocol and application for conversational agentic coding. It treats conversation threads as the primary unit of change, and provides a structural link between them and code changes (i.e., git commits).

- **Git wrapper**: `tin` is built on top of git and fully replaces it in your agentic coding workflow. It manages your git state for you; all you worry about is which conversations (and associated changes) should be committed. `tin` uses familiar git commands (see [all commands](COMMANDS.md)).

- **Built for vibe coding**: `tin` is designed to be used by devs and teams that do 95%+ of their coding via agents. Manual code edits and git operations are possible, but break the threads-as-source-of-truth model.

- **Collaborative**: `tin` provides a central repository of the conversations across your whole team and all agents, a single record of the team's decisions and the codebase's evolution.

## Why `tin`

Git retains the history of changes to the codebase. 

`tin` retains the human thoughts behind the code, the history of decisions that led to the code, and the provenance of the code.

For the first time, developers that use coding agents are forced to write down the _design thinking_ behind their code before that code is written. But those thoughts only live in local agent caches today (and proprietary, fractured web services). `tin` is the first version control system to capture and preserve those thoughts _and_ how they resulted in the code's changes, for an entire development team.

### What you get

- **Code provenance**: See exactly which conversation produced each commit, including prompts, assistant responses, and tool calls.
- **Team decision log**: A searchable history of how design decisions were made, across all agents and developers.
- **Onboarding & reviews**: New engineers or reviewers can replay the threads behind complex features instead of guessing from diffs.
- **Compliance & governance**: Keep an auditable record of AI involvement in your codebase.

## How it works

`tin` sits between your AI coding tool and git. It:

- captures each conversation or session as a thread,
- records which git changes each assistant message produced, and
- lets you stage and commit whole threads (and their code changes) together.

You still get a normal git repo, but every commit now has a linked thread explaining how and why it happened.

## Quickstart

```bash
go install github.com/sestinj/tin/cmd/tin@latest

######################
# If using Amp
######################

tin init                        # Initialize in a git repo
amp                             # ... have a conversation
tin amp pull                    # Pull the latest threads from ampcode.com
tin status                      # See pending threads
tin commit -m "Added feature"   # Commit threads + code together

######################
# If using Claude Code
######################

tin init                        # Initialize in a git repo
tin hooks install               # Set up Claude Code integration
claude                          # ... have a conversation
tin status                      # See pending threads
tin commit -m "Added feature"   # Commit threads + code together
```

## Claude Code Plugin

Install the tin plugin for Claude Code to get:
- **Auto-invoked skill**: Claude automatically uses `tin` instead of `git` for commits, checkouts, and pushes
- **Slash commands**: `/tin:commit`, `/tin:branches`, `/tin:checkout`

```bash
# Install the plugin
claude plugin add sestinj/tin

# Or install from local clone
git clone https://github.com/sestinj/tin.git
claude plugin add ./tin
```

After installing the plugin, Claude will automatically:
- Use `tin commit` instead of `git commit`
- Use `tin checkout` instead of `git checkout`
- Use `tin push` instead of `git push`

This ensures all your commits are linked to conversation threads for code review context.

## The full `tin` workflow

1. **Download and install `tin`, and initialize your repository**

Run `go install github.com/sestinj/tin/cmd/tin@latest`

Run `tin init` in your new project folder or existing git repo.

2. **Install your agent's `tin` bindings** (Claude Code only)

If using Claude Code, run `tin hooks install` (with or without the `-g` global flag) to install hooks and slash commands. The hooks will automatically track your conversations and code changes.

3. **Code as normal in your agent**

4. **Pull your latest threads from ampcode.com** (Amp only)

If using Amp, run `tin amp pull` in your repo to import the latest (or N latest) threads into tin from ampcode.com. Imported threads are auto-staged for commit.

5. **Stage and commit your changes using `tin`**

Instead of `git add` and `git commit`, use `tin add` and `tin commit`. You select threads (or portions of a thread) to stage, and `tin` commits both the code changes and the linked conversations.

Push your changes to a `tin` remote (see `tin serve`) if you want to collaborate or backup your history. `tin push` and `tin pull` are supported. See more in the command documentation.

**That's it! :tada:**

`tin` will track your agent conversations (and all versions of them, as they change) and the code changes associated with each one.

### Commands documentation

See [all `tin` commands](COMMANDS.md).

### `tin` server and web viewer

`tin` ships with server commands for hosting remote repositories:

```bash
# TCP server (for local networks or trusted environments)
tin serve --root ~/tin-repos                    # Serve repos on port 2323
tin serve --root ~/tin-repos --host 0.0.0.0     # Listen on all interfaces

# HTTP server with Basic Auth (for production)
tin serve-http --root ~/tin-repos --auth admin:secrettoken
tin serve-http --root ~/tin-repos --auth alice:pass1 --auth bob:pass2

# Or configure auth via environment variable
TIN_SERVER_AUTH=alice:pass1,bob:pass2 tin serve-http --root ~/tin-repos
```

Client setup:
```bash
# For TCP server
tin remote add origin localhost:2323/myproject.tin
tin push origin main

# For HTTP server
tin remote add origin https://host:8443/myproject.tin
tin config credentials add host:8443 admin:secrettoken
tin push origin main
```

`tin` also provides a simple web viewer to see repositories, commits, and threads:
```bash
tin serve --web --root ~/projects --port 8080
```

**The tin web commits page**

<img src="assets/tin-web-commits.png" alt="The tin web viewer: commits" width="500">

**The tin web thread page**

<img src="assets/tin-web-thread.png" alt="The tin web viewer: threads" width="500">

### `tin` in git

All `tin` commits are connected to git commits (if the git commit hash for a tin commit changes, the tin commit hash will as well). All git commits link back to the `tin` threads that created them.

**tin in GitHub**

<img src="assets/tin-in-github.png" alt="Git commits with tin connectivity" width="500">

## Why the name "`tin`"?

>**git** (plural gits)
>
>(British, Ireland, slang, derogatory) A silly, incompetent, stupid or annoying person (usually a man). 

\- [Wiktionary](https://en.wiktionary.org/wiki/git)

>"I'm an egotistical bastard, and I name all my projects after myself. First Linux, now **git**."

\- Linus Torvalds

>**tin can**: (informal, sometimes derogatory) A nickname for a robot or artificial intelligence.
>
>_Do as you’re told, tin can!_

\- [Wiktionary](https://en.wiktionary.org/wiki/tin_can)

## Contributing

`tin` is a proof of concept, 100% vibe coded, and may not be suitable for all agentic coding workflows. 

It started as a holiday hobby project. If `tin` gets adoption, I intend to contribute it to an open source foundation and build a proper maintainer group. If you are interested in helping shape that, please reach out or open an issue.

If this direction resonates, ⭐ the repo and share feedback. This will help me prioritize where to take `tin` next.
