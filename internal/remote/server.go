package remote

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

// Server handles remote tin connections
type Server struct {
	host       string
	port       int
	repoPath   string // single repo mode (legacy)
	rootPath   string // multi-repo mode: serve any repo under this root
	autoCreate bool   // auto-create repos on push
	listener   net.Listener
}

// NewServer creates a new tin server for a single repository
func NewServer(host string, port int, repoPath string) *Server {
	return &Server{
		host:     host,
		port:     port,
		repoPath: repoPath,
	}
}

// NewMultiRepoServer creates a server that can serve multiple repositories under a root path
func NewMultiRepoServer(host string, port int, rootPath string, autoCreate bool) *Server {
	return &Server{
		host:       host,
		port:       port,
		rootPath:   rootPath,
		autoCreate: autoCreate,
	}
}

// Start starts the server and listens for connections
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	log.Printf("tin server listening on %s", addr)
	if s.rootPath != "" {
		log.Printf("serving repositories under: %s", s.rootPath)
		if s.autoCreate {
			log.Printf("auto-create enabled: new repos will be created on push")
		}
	} else {
		log.Printf("serving repository: %s", s.repoPath)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("new connection from %s", remoteAddr)

	pc := NewProtocolConn(conn)

	// Read hello message
	msg, err := pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to read hello: %v", remoteAddr, err)
		return
	}

	if msg.Type != MsgHello {
		pc.SendError(ErrCodeInvalidRequest, "expected hello message")
		return
	}

	var hello HelloMessage
	if err := msg.DecodePayload(&hello); err != nil {
		pc.SendError(ErrCodeInvalidRequest, "invalid hello payload")
		return
	}

	if hello.Version != ProtocolVersion {
		pc.SendError(ErrCodeProtocolVersion, fmt.Sprintf("unsupported protocol version: %d", hello.Version))
		return
	}

	// Resolve repository path
	repoPath, err := s.resolveRepoPath(hello.RepoPath)
	if err != nil {
		pc.SendError(ErrCodeInvalidRequest, err.Error())
		return
	}

	log.Printf("[%s] repo: %s, operation: %s", remoteAddr, repoPath, hello.Operation)

	// Open or create repository
	repo, err := storage.OpenBare(repoPath)
	if err != nil {
		// Try to auto-create on push if enabled
		if s.autoCreate && hello.Operation == "push" {
			log.Printf("[%s] creating new repository: %s", remoteAddr, repoPath)
			repo, err = storage.InitBare(repoPath)
			if err != nil {
				pc.SendError(ErrCodeInternal, "failed to create repository: "+err.Error())
				return
			}
		} else {
			pc.SendError(ErrCodeNotFound, "repository not found: "+repoPath)
			return
		}
	}

	switch hello.Operation {
	case "push":
		s.handlePush(pc, repo, remoteAddr)
	case "pull":
		s.handlePull(pc, repo, remoteAddr)
	case "config":
		s.handleConfig(pc, repo, remoteAddr)
	default:
		pc.SendError(ErrCodeInvalidRequest, "unknown operation: "+hello.Operation)
	}
}

// resolveRepoPath resolves the repository path from the client request
func (s *Server) resolveRepoPath(clientPath string) (string, error) {
	// Single-repo mode: ignore client path, use configured repo
	if s.rootPath == "" {
		return s.repoPath, nil
	}

	// Multi-repo mode: resolve path under root
	// Strip leading slashes to make path relative
	cleanPath := strings.TrimLeft(clientPath, "/")

	// Clean the path to normalize it
	cleanPath = filepath.Clean(cleanPath)

	// Reject paths that try to escape (.. components)
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return "", fmt.Errorf("invalid repository path: %s", clientPath)
	}

	// Reject empty paths
	if cleanPath == "" || cleanPath == "." {
		return "", fmt.Errorf("repository path required")
	}

	// Join with root (cleanPath is now guaranteed to be relative)
	fullPath := filepath.Join(s.rootPath, cleanPath)

	// Double-check the result is under root (belt and suspenders)
	absRoot, _ := filepath.Abs(s.rootPath)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("invalid repository path: %s", clientPath)
	}

	return fullPath, nil
}

func (s *Server) handlePush(pc *ProtocolConn, repo *storage.Repository, remoteAddr string) {
	// Send refs advertisement
	refs, err := buildRefsMessage(repo)
	if err != nil {
		pc.SendError(ErrCodeInternal, "failed to build refs: "+err.Error())
		return
	}

	if err := pc.Send(MsgRefs, refs); err != nil {
		log.Printf("[%s] failed to send refs: %v", remoteAddr, err)
		return
	}

	// Receive pack
	msg, err := pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to receive pack: %v", remoteAddr, err)
		return
	}

	if msg.Type != MsgPack {
		pc.SendError(ErrCodeInvalidRequest, "expected pack message")
		return
	}

	var pack PackMessage
	if err := msg.DecodePayload(&pack); err != nil {
		pc.SendError(ErrCodeInvalidRequest, "invalid pack payload")
		return
	}

	// Save received objects
	for _, thread := range pack.Threads {
		t := thread // avoid closure issue
		if err := repo.SaveThread(&t); err != nil {
			log.Printf("[%s] failed to save thread %s: %v", remoteAddr, thread.ID, err)
		}
	}

	for _, commit := range pack.Commits {
		c := commit // avoid closure issue
		if err := repo.SaveCommit(&c); err != nil {
			log.Printf("[%s] failed to save commit %s: %v", remoteAddr, commit.ID, err)
		}
	}

	log.Printf("[%s] received %d threads, %d commits", remoteAddr, len(pack.Threads), len(pack.Commits))

	// Receive ref updates
	msg, err = pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to receive update-refs: %v", remoteAddr, err)
		return
	}

	if msg.Type != MsgUpdateRefs {
		pc.SendError(ErrCodeInvalidRequest, "expected update-refs message")
		return
	}

	var updateRefs UpdateRefsMessage
	if err := msg.DecodePayload(&updateRefs); err != nil {
		pc.SendError(ErrCodeInvalidRequest, "invalid update-refs payload")
		return
	}

	// Apply ref updates
	for branch, commitID := range updateRefs.Updates {
		// Check fast-forward if not force
		if !updateRefs.Force {
			currentCommitID, _ := repo.ReadBranch(branch)
			if currentCommitID != "" {
				// Check if this is a fast-forward
				if !isAncestor(repo, currentCommitID, commitID) {
					pc.SendError(ErrCodeNotFastForward, fmt.Sprintf("non-fast-forward update rejected for %s (use --force)", branch))
					return
				}
			}
		}

		if err := repo.WriteBranch(branch, commitID); err != nil {
			pc.SendError(ErrCodeInternal, "failed to update ref: "+err.Error())
			return
		}
		log.Printf("[%s] updated %s -> %s", remoteAddr, branch, commitID[:12])
	}

	pc.SendOK("push successful")
}

func (s *Server) handlePull(pc *ProtocolConn, repo *storage.Repository, remoteAddr string) {
	// Send refs advertisement
	refs, err := buildRefsMessage(repo)
	if err != nil {
		pc.SendError(ErrCodeInternal, "failed to build refs: "+err.Error())
		return
	}

	if err := pc.Send(MsgRefs, refs); err != nil {
		log.Printf("[%s] failed to send refs: %v", remoteAddr, err)
		return
	}

	// Receive want message
	msg, err := pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to receive want: %v", remoteAddr, err)
		return
	}

	if msg.Type != MsgWant {
		pc.SendError(ErrCodeInvalidRequest, "expected want message")
		return
	}

	var want WantMessage
	if err := msg.DecodePayload(&want); err != nil {
		pc.SendError(ErrCodeInvalidRequest, "invalid want payload")
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
			log.Printf("[%s] thread not found: %s", remoteAddr, threadID)
			continue
		}
		pack.Threads = append(pack.Threads, *thread)
	}

	for _, commitID := range want.CommitIDs {
		commit, err := repo.LoadCommit(commitID)
		if err != nil {
			log.Printf("[%s] commit not found: %s", remoteAddr, commitID)
			continue
		}
		pack.Commits = append(pack.Commits, *commit)
	}

	if err := pc.Send(MsgPack, pack); err != nil {
		log.Printf("[%s] failed to send pack: %v", remoteAddr, err)
		return
	}

	log.Printf("[%s] sent %d threads, %d commits", remoteAddr, len(pack.Threads), len(pack.Commits))

	// Wait for OK
	msg, err = pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to receive ok: %v", remoteAddr, err)
		return
	}

	if msg.Type == MsgOK {
		log.Printf("[%s] pull completed", remoteAddr)
	}
}

func buildRefsMessage(repo *storage.Repository) (*RefsMessage, error) {
	refs := &RefsMessage{
		Branches:  make(map[string]string),
		CommitIDs: make([]string, 0),
		ThreadIDs: make([]string, 0),
	}

	// Get HEAD
	head, err := repo.ReadHead()
	if err == nil {
		refs.HEAD = head
	}

	// Get branches
	branches, err := repo.ListBranches()
	if err == nil {
		for _, branch := range branches {
			commitID, err := repo.ReadBranch(branch)
			if err == nil {
				refs.Branches[branch] = commitID
			}
		}
	}

	// Get all commit IDs
	commits, err := repo.ListCommits()
	if err == nil {
		for _, commit := range commits {
			refs.CommitIDs = append(refs.CommitIDs, commit.ID)
		}
	}

	// Get all thread IDs
	threads, err := repo.ListThreads()
	if err == nil {
		for _, thread := range threads {
			refs.ThreadIDs = append(refs.ThreadIDs, thread.ID)
		}
	}

	return refs, nil
}

// isAncestor checks if ancestorID is an ancestor of commitID
func isAncestor(repo *storage.Repository, ancestorID, commitID string) bool {
	if ancestorID == commitID {
		return true
	}

	// Walk back from commitID looking for ancestorID
	current := commitID
	for current != "" {
		commit, err := repo.LoadCommit(current)
		if err != nil {
			return false
		}
		if commit.ParentCommitID == ancestorID {
			return true
		}
		current = commit.ParentCommitID
	}

	return false
}

func (s *Server) handleConfig(pc *ProtocolConn, repo *storage.Repository, remoteAddr string) {
	// Read config message (get or set)
	msg, err := pc.Receive()
	if err != nil {
		log.Printf("[%s] failed to receive config message: %v", remoteAddr, err)
		return
	}

	switch msg.Type {
	case MsgGetConfig:
		// Send current config
		config, err := repo.ReadConfig()
		if err != nil {
			pc.SendError(ErrCodeInternal, "failed to read config: "+err.Error())
			return
		}
		configMsg := ConfigMessage{
			CodeHostURL: config.CodeHostURL,
		}
		if err := pc.Send(MsgConfig, configMsg); err != nil {
			log.Printf("[%s] failed to send config: %v", remoteAddr, err)
		}
		log.Printf("[%s] sent config", remoteAddr)

	case MsgSetConfig:
		var setConfig SetConfigMessage
		if err := msg.DecodePayload(&setConfig); err != nil {
			pc.SendError(ErrCodeInvalidRequest, "invalid set-config payload")
			return
		}

		// Update config
		config, err := repo.ReadConfig()
		if err != nil {
			pc.SendError(ErrCodeInternal, "failed to read config: "+err.Error())
			return
		}

		config.CodeHostURL = setConfig.CodeHostURL
		if err := repo.WriteConfig(config); err != nil {
			pc.SendError(ErrCodeInternal, "failed to write config: "+err.Error())
			return
		}

		pc.SendOK("config updated")
		log.Printf("[%s] updated code_host_url to %s", remoteAddr, setConfig.CodeHostURL)

	default:
		pc.SendError(ErrCodeInvalidRequest, "expected get-config or set-config message")
	}
}
