# TIN Authentication & Transport

TIN supports multiple transports for remote operations, following Git's model where authentication is handled at the transport layer rather than embedded in the wire protocol.

## Transports

### TCP Transport (Default)

**URL formats:**
- `tin://host:port/path`
- `host:port/path`
- `host/path` (default port 2323)

Used for local networks or trusted environments. No built-in authentication.

```bash
tin remote add origin localhost:2323/myproject.tin
tin push origin main
```

### HTTPS Transport

**URL format:** `https://host/path`

Uses HTTP Basic Auth for authentication. The TIN protocol messages are sent as POST request/response bodies.

```bash
tin remote add origin https://tinhub.dev/user/repo
tin config credentials add tinhub.dev th_yourtoken
tin push origin main
```

**HTTP Endpoints:**
- `POST /{repo-path}/tin-receive-pack` - Push (client sends data)
- `POST /{repo-path}/tin-upload-pack` - Pull (server sends data)
- `POST /{repo-path}/tin-config` - Get/set repository config

**Content-Type:** `application/x-tin-protocol`

## Credential Storage

Credentials are resolved in this order:

1. **Environment variable** `TIN_AUTH_TOKEN` (highest priority)
2. **Per-host credentials** in `.tin/config`
3. **Legacy `auth_token`** in `.tin/config` (deprecated)

### Managing Credentials

```bash
# Store credentials for a host
tin config credentials add tinhub.dev th_xxxxx

# List stored credentials
tin config credentials

# Remove credentials
tin config credentials remove tinhub.dev
```

### Config File Format

```json
{
  "version": 1,
  "credentials": [
    {"host": "tinhub.dev", "token": "th_xxxxx"},
    {"host": "localhost:8443", "token": "th_yyyyy"}
  ]
}
```

## Running an HTTP Server

```bash
# Start HTTP server
tin serve-http --root /var/tin-repos

# With custom address
tin serve-http --addr :8443 --root ~/tin-repos
```

The server:
- Requires HTTP Basic Auth (any username, token as password)
- Auto-creates repositories on push
- Uses the same protocol as TCP, just over HTTP

For production, use a reverse proxy (nginx, caddy) for TLS termination.

## Architecture

```
┌─────────────┐
│   Client    │
├─────────────┤
│  Transport  │ ← TCP or HTTPS (selected by URL scheme)
│  Interface  │
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│    TCP      │     │   HTTPS     │
│  Transport  │     │  Transport  │
│             │     │ + Basic Auth│
└──────┬──────┘     └──────┬──────┘
       │                   │
       ▼                   ▼
   TCP Server         HTTP Server
   (tin serve)      (tin serve-http)
```

### Key Files

- `internal/remote/transport.go` - Transport interface
- `internal/remote/tcp_transport.go` - TCP implementation
- `internal/remote/https_transport.go` - HTTPS with Basic Auth
- `internal/remote/credentials.go` - Credential store
- `internal/remote/http_server.go` - HTTP server handler

## Future Work

- **SSH Transport** - `tin@host:user/repo` format with SSH key authentication
- **Git credential helper compatibility** - Reuse existing credential managers
- **OAuth/OIDC** - Web-based authentication flow
