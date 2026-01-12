package remote

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dadlerj/tin/internal/model"
	"github.com/dadlerj/tin/internal/storage"
)

// AuthValidator validates authentication credentials
type AuthValidator interface {
	// Validate checks if the credentials are valid and returns the user identifier
	Validate(username, password string) (userID string, valid bool)
}

// AllowAllAuthValidator allows any credentials (for testing/development)
type AllowAllAuthValidator struct{}

func (v *AllowAllAuthValidator) Validate(username, password string) (string, bool) {
	return username, true
}

// TokenAuthValidator validates against a preset list of username/password pairs
type TokenAuthValidator struct {
	// credentials maps username to password
	credentials map[string]string
}

// NewTokenAuthValidator creates a validator with the given username/password pairs
func NewTokenAuthValidator(creds map[string]string) *TokenAuthValidator {
	return &TokenAuthValidator{
		credentials: creds,
	}
}

func (v *TokenAuthValidator) Validate(username, password string) (string, bool) {
	if v.credentials == nil {
		return "", false
	}
	expectedPassword, exists := v.credentials[username]
	if !exists {
		return "", false
	}
	if expectedPassword != password {
		return "", false
	}
	return username, true
}

// HTTPHandler handles HTTP requests for the TIN protocol
type HTTPHandler struct {
	rootPath      string
	autoCreate    bool
	authValidator AuthValidator
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(rootPath string, autoCreate bool, authValidator AuthValidator) *HTTPHandler {
	if authValidator == nil {
		authValidator = &AllowAllAuthValidator{}
	}
	return &HTTPHandler{
		rootPath:      rootPath,
		autoCreate:    autoCreate,
		authValidator: authValidator,
	}
}

// ServeHTTP implements http.Handler
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract Basic Auth credentials
	username, password, hasAuth := r.BasicAuth()
	if !hasAuth {
		w.Header().Set("WWW-Authenticate", `Basic realm="tin"`)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	userID, valid := h.authValidator.Validate(username, password)
	if !valid {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Parse path to get repo path and operation
	// Expected format: /{repo-path}/tin-{operation}
	// e.g., /user/repo/tin-receive-pack
	path := strings.TrimPrefix(r.URL.Path, "/")

	var repoPath, operation string
	if idx := strings.LastIndex(path, "/tin-"); idx != -1 {
		repoPath = "/" + path[:idx]
		opPart := path[idx+5:] // skip "/tin-"
		switch opPart {
		case "receive-pack":
			operation = "push"
		case "upload-pack":
			operation = "pull"
		case "config":
			operation = "config"
		default:
			http.Error(w, "Unknown operation", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	log.Printf("[HTTP %s] repo: %s, operation: %s", userID, repoPath, operation)

	// Resolve repository path
	fullRepoPath, err := h.resolveRepoPath(repoPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Open or create repository
	repo, err := storage.OpenBare(fullRepoPath)
	if err != nil {
		if h.autoCreate && operation == "push" {
			log.Printf("[HTTP %s] creating new repository: %s", userID, fullRepoPath)
			repo, err = storage.InitBare(fullRepoPath)
			if err != nil {
				http.Error(w, "Failed to create repository", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Repository not found", http.StatusNotFound)
			return
		}
	}

	// Set response content type
	w.Header().Set("Content-Type", ContentTypeTinProtocol)

	// Create a buffer for the response
	var respBuf bytes.Buffer

	// Create protocol connection adapters using the helper
	reqPC := NewProtocolConnFromHTTP(r.Body, io.Discard)
	respPC := NewProtocolConnFromHTTP(strings.NewReader(""), &respBuf)

	// Handle the operation
	switch operation {
	case "push":
		h.handlePush(reqPC, respPC, repo, userID)
	case "pull":
		h.handlePull(reqPC, respPC, repo, userID)
	case "config":
		h.handleConfig(reqPC, respPC, repo, userID)
	}

	// Write response
	w.Write(respBuf.Bytes())
}

// resolveRepoPath resolves the repository path (same logic as TCP server)
func (h *HTTPHandler) resolveRepoPath(clientPath string) (string, error) {
	// Clean the path
	cleanPath := strings.TrimLeft(clientPath, "/")
	cleanPath = filepath.Clean(cleanPath)

	// Reject paths that try to escape
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return "", fmt.Errorf("invalid repository path: %s", clientPath)
	}

	if cleanPath == "" || cleanPath == "." {
		return "", fmt.Errorf("repository path required")
	}

	// Join with root path
	fullPath := filepath.Join(h.rootPath, cleanPath)

	// Double-check the result is under root
	absRoot, _ := filepath.Abs(h.rootPath)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("invalid repository path: %s", clientPath)
	}

	return fullPath, nil
}

func (h *HTTPHandler) handlePush(reqPC, respPC *ProtocolConn, repo *storage.Repository, userID string) {
	// HTTP push has two phases:
	// Phase 1 (refs negotiation): empty request → server sends refs
	// Phase 2 (actual push): Pack + UpdateRefs → server sends OK

	// Try to receive first message
	msg, err := reqPC.Receive()
	if err != nil {
		// Empty request body = refs negotiation phase
		// Send refs so client knows what we have
		refs, err := buildRefsMessage(repo)
		if err != nil {
			respPC.SendError(ErrCodeInternal, "failed to build refs: "+err.Error())
			return
		}
		if err := respPC.Send(MsgRefs, refs); err != nil {
			log.Printf("[HTTP %s] failed to send refs: %v", userID, err)
		}
		log.Printf("[HTTP %s] sent refs (negotiation phase)", userID)
		return
	}

	if msg.Type != MsgPack {
		respPC.SendError(ErrCodeInvalidRequest, "expected pack message")
		return
	}

	var pack PackMessage
	if err := msg.DecodePayload(&pack); err != nil {
		respPC.SendError(ErrCodeInvalidRequest, "invalid pack payload")
		return
	}

	// Save received objects
	for _, thread := range pack.Threads {
		t := thread
		if err := repo.SaveThread(&t); err != nil {
			log.Printf("[HTTP %s] failed to save thread %s: %v", userID, thread.ID, err)
		}
	}

	for _, commit := range pack.Commits {
		c := commit
		if err := repo.SaveCommit(&c); err != nil {
			log.Printf("[HTTP %s] failed to save commit %s: %v", userID, commit.ID, err)
		}
	}

	log.Printf("[HTTP %s] received %d threads, %d commits", userID, len(pack.Threads), len(pack.Commits))

	// Receive ref updates
	msg, err = reqPC.Receive()
	if err != nil {
		log.Printf("[HTTP %s] failed to receive update-refs: %v", userID, err)
		respPC.SendError(ErrCodeInvalidRequest, "failed to receive update-refs")
		return
	}

	if msg.Type != MsgUpdateRefs {
		respPC.SendError(ErrCodeInvalidRequest, "expected update-refs message")
		return
	}

	var updateRefs UpdateRefsMessage
	if err := msg.DecodePayload(&updateRefs); err != nil {
		respPC.SendError(ErrCodeInvalidRequest, "invalid update-refs payload")
		return
	}

	// Apply ref updates
	for branch, commitID := range updateRefs.Updates {
		if !updateRefs.Force {
			currentCommitID, _ := repo.ReadBranch(branch)
			if currentCommitID != "" {
				if !isAncestor(repo, currentCommitID, commitID) {
					respPC.SendError(ErrCodeNotFastForward, fmt.Sprintf("non-fast-forward update rejected for %s", branch))
					return
				}
			}
		}

		if err := repo.WriteBranch(branch, commitID); err != nil {
			respPC.SendError(ErrCodeInternal, "failed to update ref: "+err.Error())
			return
		}
		log.Printf("[HTTP %s] updated %s -> %s", userID, branch, commitID[:12])
	}

	respPC.SendOK("push successful")
}

func (h *HTTPHandler) handlePull(reqPC, respPC *ProtocolConn, repo *storage.Repository, userID string) {
	// Send refs advertisement
	refs, err := buildRefsMessage(repo)
	if err != nil {
		respPC.SendError(ErrCodeInternal, "failed to build refs: "+err.Error())
		return
	}

	if err := respPC.Send(MsgRefs, refs); err != nil {
		log.Printf("[HTTP %s] failed to send refs: %v", userID, err)
		return
	}

	// Receive want message
	msg, err := reqPC.Receive()
	if err != nil {
		log.Printf("[HTTP %s] failed to receive want: %v", userID, err)
		respPC.SendError(ErrCodeInvalidRequest, "failed to receive want")
		return
	}

	if msg.Type != MsgWant {
		respPC.SendError(ErrCodeInvalidRequest, "expected want message")
		return
	}

	var want WantMessage
	if err := msg.DecodePayload(&want); err != nil {
		respPC.SendError(ErrCodeInvalidRequest, "invalid want payload")
		return
	}

	// Build pack with requested objects
	pack := PackMessage{
		Commits: make([]model.TinCommit, 0),
		Threads: make([]model.Thread, 0),
	}

	for _, threadID := range want.ThreadIDs {
		thread, err := repo.LoadThread(threadID)
		if err != nil {
			continue
		}
		pack.Threads = append(pack.Threads, *thread)
	}

	for _, versionRef := range want.ThreadVersions {
		thread, err := repo.LoadThreadVersion(versionRef.ThreadID, versionRef.ContentHash)
		if err != nil {
			continue
		}
		pack.Threads = append(pack.Threads, *thread)
	}

	for _, commitID := range want.CommitIDs {
		commit, err := repo.LoadCommit(commitID)
		if err != nil {
			continue
		}
		pack.Commits = append(pack.Commits, *commit)
	}

	if err := respPC.Send(MsgPack, pack); err != nil {
		log.Printf("[HTTP %s] failed to send pack: %v", userID, err)
		return
	}

	log.Printf("[HTTP %s] sent %d threads, %d commits", userID, len(pack.Threads), len(pack.Commits))
}

func (h *HTTPHandler) handleConfig(reqPC, respPC *ProtocolConn, repo *storage.Repository, userID string) {
	msg, err := reqPC.Receive()
	if err != nil {
		log.Printf("[HTTP %s] failed to receive config message: %v", userID, err)
		respPC.SendError(ErrCodeInvalidRequest, "failed to receive config message")
		return
	}

	switch msg.Type {
	case MsgGetConfig:
		config, err := repo.ReadConfig()
		if err != nil {
			respPC.SendError(ErrCodeInternal, "failed to read config")
			return
		}
		configMsg := ConfigMessage{
			CodeHostURL: config.CodeHostURL,
		}
		respPC.Send(MsgConfig, configMsg)
		log.Printf("[HTTP %s] sent config", userID)

	case MsgSetConfig:
		var setConfig SetConfigMessage
		if err := msg.DecodePayload(&setConfig); err != nil {
			respPC.SendError(ErrCodeInvalidRequest, "invalid set-config payload")
			return
		}

		config, err := repo.ReadConfig()
		if err != nil {
			respPC.SendError(ErrCodeInternal, "failed to read config")
			return
		}

		config.CodeHostURL = setConfig.CodeHostURL
		if err := repo.WriteConfig(config); err != nil {
			respPC.SendError(ErrCodeInternal, "failed to write config")
			return
		}

		respPC.SendOK("config updated")
		log.Printf("[HTTP %s] updated code_host_url to %s", userID, setConfig.CodeHostURL)

	default:
		respPC.SendError(ErrCodeInvalidRequest, "expected get-config or set-config message")
	}
}

// ParseBasicAuth parses a Basic auth header value
func ParseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", "", false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// httpReadWriter wraps separate readers and writers
type httpReadWriter struct {
	r io.Reader
	w io.Writer
}

func (rw *httpReadWriter) Read(p []byte) (n int, err error) {
	return rw.r.Read(p)
}

func (rw *httpReadWriter) Write(p []byte) (n int, err error) {
	return rw.w.Write(p)
}

// NewProtocolConnFromHTTP creates a ProtocolConn from HTTP request/response
func NewProtocolConnFromHTTP(r io.Reader, w io.Writer) *ProtocolConn {
	return &ProtocolConn{
		reader: bufio.NewReader(r),
		writer: w,
	}
}
