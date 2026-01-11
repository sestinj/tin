package remote

import (
	"fmt"

	"github.com/dadlerj/tin/internal/model"
	"github.com/dadlerj/tin/internal/storage"
)

// Client handles connections to remote tin servers
type Client struct {
	transport Transport
	url       *ParsedURL
}

// Dial connects to a remote tin server using the appropriate transport
func Dial(rawURL string, creds *Credentials) (*Client, error) {
	url, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}

	var transport Transport

	switch url.TransportType() {
	case "https":
		transport, err = NewHTTPSTransport(url, creds)
	default: // "tcp"
		transport, err = NewTCPTransport(url)
	}

	if err != nil {
		return nil, err
	}

	return &Client{
		transport: transport,
		url:       url,
	}, nil
}

// makeHello creates a HelloMessage
// Note: Authentication is handled at the transport layer, not in the protocol
func (c *Client) makeHello(operation string) HelloMessage {
	return HelloMessage{
		Version:   ProtocolVersion,
		Operation: operation,
		RepoPath:  c.url.Path,
	}
}

// Close closes the connection
func (c *Client) Close() error {
	return c.transport.Close()
}

// Push pushes commits and threads to the remote and updates refs
func (c *Client) Push(repo *storage.Repository, branch string, force bool) error {
	// Send hello
	if err := c.transport.Send(MsgHello, c.makeHello("push")); err != nil {
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Receive refs
	msg, err := c.transport.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive refs: %w", err)
	}
	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return fmt.Errorf("server error: %s", errMsg.Message)
	}
	if msg.Type != MsgRefs {
		return fmt.Errorf("expected refs message, got %s", msg.Type)
	}

	var remoteRefs RefsMessage
	if err := msg.DecodePayload(&remoteRefs); err != nil {
		return fmt.Errorf("failed to decode refs: %w", err)
	}

	// Build set of remote commits for quick lookup
	remoteCommits := make(map[string]bool)
	for _, id := range remoteRefs.CommitIDs {
		remoteCommits[id] = true
	}

	// Get local commit for branch
	localCommitID, err := repo.ReadBranch(branch)
	if err != nil {
		return fmt.Errorf("branch %s not found", branch)
	}

	// Collect commits to send (walk back from branch tip)
	commitsToSend := make([]model.TinCommit, 0)
	threadsToSend := make(map[string]*model.Thread)

	current := localCommitID
	for current != "" {
		if remoteCommits[current] {
			break // Remote already has this commit and ancestors
		}

		commit, err := repo.LoadCommit(current)
		if err != nil {
			return fmt.Errorf("failed to load commit %s: %w", current, err)
		}
		commitsToSend = append(commitsToSend, *commit)

		// Collect threads referenced by this commit
		// Load specific versions if ContentHash is available, otherwise load latest
		for _, ref := range commit.Threads {
			// Use ContentHash as key if available to ensure we send the right version
			key := ref.ThreadID
			if ref.ContentHash != "" {
				key = ref.ThreadID + "@" + ref.ContentHash
			}

			if threadsToSend[key] == nil {
				var thread *model.Thread
				var err error

				// Try to load specific version first
				if ref.ContentHash != "" {
					thread, err = repo.LoadThreadVersion(ref.ThreadID, ref.ContentHash)
				}
				// Fall back to latest version
				if thread == nil || err != nil {
					thread, err = repo.LoadThread(ref.ThreadID)
				}
				if err != nil {
					return fmt.Errorf("failed to load thread %s: %w", ref.ThreadID, err)
				}
				threadsToSend[key] = thread
			}
		}

		current = commit.ParentCommitID
	}

	// Convert threads map to slice
	threads := make([]model.Thread, 0, len(threadsToSend))
	for _, t := range threadsToSend {
		threads = append(threads, *t)
	}

	// Reverse commits to send oldest first
	for i, j := 0, len(commitsToSend)-1; i < j; i, j = i+1, j-1 {
		commitsToSend[i], commitsToSend[j] = commitsToSend[j], commitsToSend[i]
	}

	// Send pack
	pack := PackMessage{
		Commits: commitsToSend,
		Threads: threads,
	}
	if err := c.transport.Send(MsgPack, pack); err != nil {
		return fmt.Errorf("failed to send pack: %w", err)
	}

	// Send ref updates
	updateRefs := UpdateRefsMessage{
		Updates: map[string]string{branch: localCommitID},
		Force:   force,
	}
	if err := c.transport.Send(MsgUpdateRefs, updateRefs); err != nil {
		return fmt.Errorf("failed to send update-refs: %w", err)
	}

	// Wait for response
	msg, err = c.transport.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive response: %w", err)
	}

	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return fmt.Errorf("push rejected: %s", errMsg.Message)
	}

	return nil
}

// GetConfig retrieves the remote repository's config
func (c *Client) GetConfig() (*ConfigMessage, error) {
	// Send hello
	if err := c.transport.Send(MsgHello, c.makeHello("config")); err != nil {
		return nil, fmt.Errorf("failed to send hello: %w", err)
	}

	// Send get-config request
	if err := c.transport.Send(MsgGetConfig, GetConfigMessage{}); err != nil {
		return nil, fmt.Errorf("failed to send get-config: %w", err)
	}

	// Receive config
	msg, err := c.transport.Receive()
	if err != nil {
		return nil, fmt.Errorf("failed to receive config: %w", err)
	}
	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return nil, fmt.Errorf("server error: %s", errMsg.Message)
	}
	if msg.Type != MsgConfig {
		return nil, fmt.Errorf("expected config message, got %s", msg.Type)
	}

	var config ConfigMessage
	if err := msg.DecodePayload(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// SetConfig updates the remote repository's config
func (c *Client) SetConfig(config *SetConfigMessage) error {
	// Send hello
	if err := c.transport.Send(MsgHello, c.makeHello("config")); err != nil {
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Send set-config request
	if err := c.transport.Send(MsgSetConfig, config); err != nil {
		return fmt.Errorf("failed to send set-config: %w", err)
	}

	// Receive response
	msg, err := c.transport.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive response: %w", err)
	}
	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return fmt.Errorf("server error: %s", errMsg.Message)
	}

	return nil
}

// Pull pulls commits and threads from the remote
func (c *Client) Pull(repo *storage.Repository, branch string) (*RefsMessage, error) {
	// Send hello
	if err := c.transport.Send(MsgHello, c.makeHello("pull")); err != nil {
		return nil, fmt.Errorf("failed to send hello: %w", err)
	}

	// Receive refs
	msg, err := c.transport.Receive()
	if err != nil {
		return nil, fmt.Errorf("failed to receive refs: %w", err)
	}
	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return nil, fmt.Errorf("server error: %s", errMsg.Message)
	}
	if msg.Type != MsgRefs {
		return nil, fmt.Errorf("expected refs message, got %s", msg.Type)
	}

	var remoteRefs RefsMessage
	if err := msg.DecodePayload(&remoteRefs); err != nil {
		return nil, fmt.Errorf("failed to decode refs: %w", err)
	}

	// Build set of local objects
	localCommits := make(map[string]bool)
	commits, _ := repo.ListCommits()
	for _, c := range commits {
		localCommits[c.ID] = true
	}

	localThreads := make(map[string]bool)
	threads, _ := repo.ListThreads()
	for _, t := range threads {
		localThreads[t.ID] = true
	}

	// Determine what we need
	wantCommits := make([]string, 0)
	wantThreads := make([]string, 0)
	wantThreadVersions := make([]ThreadVersionRef, 0)

	for _, id := range remoteRefs.CommitIDs {
		if !localCommits[id] {
			wantCommits = append(wantCommits, id)
		}
	}
	for _, id := range remoteRefs.ThreadIDs {
		if !localThreads[id] {
			wantThreads = append(wantThreads, id)
		}
	}

	// Check for thread versions we don't have
	for threadID, versions := range remoteRefs.ThreadVersions {
		for _, contentHash := range versions {
			if !repo.HasThreadVersion(threadID, contentHash) {
				wantThreadVersions = append(wantThreadVersions, ThreadVersionRef{
					ThreadID:    threadID,
					ContentHash: contentHash,
				})
			}
		}
	}

	// Send want
	want := WantMessage{
		CommitIDs:      wantCommits,
		ThreadIDs:      wantThreads,
		ThreadVersions: wantThreadVersions,
	}
	if err := c.transport.Send(MsgWant, want); err != nil {
		return nil, fmt.Errorf("failed to send want: %w", err)
	}

	// Receive pack
	msg, err = c.transport.Receive()
	if err != nil {
		return nil, fmt.Errorf("failed to receive pack: %w", err)
	}
	if msg.Type == MsgError {
		var errMsg ErrorMessage
		msg.DecodePayload(&errMsg)
		return nil, fmt.Errorf("server error: %s", errMsg.Message)
	}
	if msg.Type != MsgPack {
		return nil, fmt.Errorf("expected pack message, got %s", msg.Type)
	}

	var pack PackMessage
	if err := msg.DecodePayload(&pack); err != nil {
		return nil, fmt.Errorf("failed to decode pack: %w", err)
	}

	// Save received objects
	for _, thread := range pack.Threads {
		t := thread
		if err := repo.SaveThread(&t); err != nil {
			return nil, fmt.Errorf("failed to save thread: %w", err)
		}
	}
	for _, commit := range pack.Commits {
		co := commit
		if err := repo.SaveCommit(&co); err != nil {
			return nil, fmt.Errorf("failed to save commit: %w", err)
		}
	}

	// Send OK
	c.transport.Send(MsgOK, OKMessage{Message: "received"})

	// Update local branch if specified
	if branch != "" {
		if remoteCommitID, ok := remoteRefs.Branches[branch]; ok {
			if err := repo.WriteBranch(branch, remoteCommitID); err != nil {
				return nil, fmt.Errorf("failed to update branch: %w", err)
			}
		}
	}

	return &remoteRefs, nil
}
