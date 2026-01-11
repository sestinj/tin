# TIN Authentication Design

## Current State

A temporary authentication mechanism was added directly to the TIN wire protocol, embedding an `auth` field in the `hello` message:

```json
{
  "type": "hello",
  "payload": {
    "version": 1,
    "operation": "push",
    "repo_path": "/user/repo",
    "auth": {
      "type": "token",
      "token": "th_xxxxx"
    }
  }
}
```

This works but **diverges from Git's design philosophy**. TIN's goal is to mirror Git's architecture closely, which makes design decisions simple and trustworthy.

## How Git Handles Authentication

Git does NOT embed authentication in its wire protocol. Instead, Git delegates authentication entirely to the transport layer:

### SSH Transport (`git@github.com:user/repo.git`)
- Authentication happens via SSH before Git ever speaks
- User authenticates with SSH keys
- The Git pack protocol has zero auth fields
- SSH provides encryption, host verification, and auth in one layer

### HTTPS Transport (`https://github.com/user/repo.git`)
- Authentication via HTTP Basic Auth headers
- Tokens (like GitHub PATs) are passed as the password
- Git uses credential helpers to store/retrieve credentials
- The Git pack protocol still has zero auth fields

### Git Protocol (`git://host/repo.git`)
- No authentication at all
- Typically used for read-only public access
- Rarely used for push operations

## Proposed Design for TIN

Following Git's model, TIN should support multiple transports with transport-layer authentication.

### Option 1: SSH Transport

**URL format:** `tin@tinhub.dev:user/repo` or `ssh://tin@tinhub.dev/user/repo`

**How it works:**
1. User has SSH key pair (~/.ssh/id_ed25519)
2. User registers public key with TinHub (like GitHub SSH keys)
3. `tin push` initiates SSH connection to tinhub.dev
4. SSH authenticates user via key
5. SSH spawns `tin-receive-pack` (or similar) on server
6. TIN protocol flows over the authenticated SSH channel
7. Server knows user identity from SSH auth

**Server-side requirements:**
- Run SSH server (or use Fly.io's SSH infrastructure)
- Map SSH public keys to user accounts
- Spawn TIN handler process for authenticated connections

**Client-side changes:**
- Add SSH transport in `internal/remote/`
- Parse `tin@host:path` URL format
- Use Go's `x/crypto/ssh` package
- Remove `auth` field from protocol

**Pros:**
- Battle-tested authentication model
- Users already have SSH keys
- Encryption included
- Exactly how Git does it

**Cons:**
- More complex server infrastructure
- Need to manage SSH keys in TinHub

### Option 2: HTTPS Transport

**URL format:** `https://tinhub.dev/user/repo.tin`

**How it works:**
1. User has API token (th_xxxxx)
2. `tin push` makes HTTPS request with `Authorization: Basic` header
3. Username can be anything, password is the token
4. Server validates token, identifies user
5. TIN protocol flows over HTTPS (likely as POST body or WebSocket)

**Server-side requirements:**
- HTTPS endpoint that accepts TIN protocol
- Validate Basic Auth credentials
- Could use existing Hono API infrastructure

**Client-side changes:**
- Add HTTPS transport in `internal/remote/`
- Implement credential helper system (like `git credential`)
- Use standard HTTP client with Basic Auth
- Remove `auth` field from protocol

**Credential helper flow (matching Git):**
```bash
# Git's credential helper protocol
$ git credential fill
protocol=https
host=github.com

# Helper responds with stored credentials
protocol=https
host=github.com
username=user
password=ghp_xxxx
```

TIN could implement the same:
```bash
$ tin credential fill
protocol=https
host=tinhub.dev

# Response
protocol=https
host=tinhub.dev
username=user
password=th_xxxx
```

**Pros:**
- Simpler server infrastructure (just HTTPS)
- Works through corporate firewalls
- Can reuse existing web infrastructure

**Cons:**
- Need to implement credential helper system
- Slightly more complex client code

### Option 3: Both (Like Git)

Git supports both SSH and HTTPS. TIN could do the same:

- `tin@tinhub.dev:user/repo` → SSH transport
- `https://tinhub.dev/user/repo` → HTTPS transport
- `tin://tinhub.dev/user/repo` → Unauthenticated TCP (read-only)

## Recommendation

**Start with SSH transport.** Reasons:

1. **Simpler protocol** - No need for credential helpers initially
2. **Users expect it** - Every Git user understands SSH keys
3. **Security included** - Encryption and auth in one layer
4. **Server identity** - Host key verification prevents MITM
5. **Matches Git's primary model** - Most developers use SSH for Git

**Implementation order:**
1. Remove `auth` field from TIN protocol (revert to pure protocol)
2. Implement SSH transport in client
3. Implement SSH server handler in TinHub
4. Add public key management to TinHub web UI
5. Later: Add HTTPS transport as alternative

## Files to Modify

### Revert Protocol Changes
- `internal/remote/protocol.go` - Remove `AuthInfo` and `Auth` field from `HelloMessage`
- `internal/remote/client.go` - Remove `SetAuthToken()`, `makeHello()`, `authToken` field
- `internal/commands/push.go` - Remove `getAuthToken()`, `dialWithAuth()`
- `internal/commands/pull.go` - Remove `getAuthTokenForPull()`
- `internal/commands/config.go` - Remove `auth_token` config key
- `internal/storage/repository.go` - Remove `AuthToken` from `Config` struct

### New SSH Transport
- `internal/remote/ssh.go` - SSH client implementation
- `internal/remote/transport.go` - Transport interface (SSH, HTTPS, TCP)
- `internal/remote/url.go` - Parse different URL formats

### TinHub Server
- New SSH server component
- Public key storage in database
- Key management API endpoints
- Web UI for SSH key management

## Open Questions

1. Should TIN support Git's credential helper protocol directly? This would allow reusing existing credential managers.

2. For SSH, should we use the system SSH client (`ssh` binary) like Git does, or implement SSH in Go? Using the system client is simpler but less portable.

3. Should the plain TCP transport (`tin://`) remain for local/trusted networks, or be removed entirely?

## References

- [Git Protocol Documentation](https://git-scm.com/docs/protocol-v2)
- [Git Credential Storage](https://git-scm.com/docs/gitcredentials)
- [GitHub SSH Key Setup](https://docs.github.com/en/authentication/connecting-to-github-with-ssh)
- [Go x/crypto/ssh Package](https://pkg.go.dev/golang.org/x/crypto/ssh)
