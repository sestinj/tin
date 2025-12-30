package remote

import (
	"fmt"
	"net"

	"github.com/danieladler/tin/internal/model"
	"github.com/danieladler/tin/internal/storage"
)

// Client handles connections to remote tin servers
type Client struct {
	conn net.Conn
	pc   *ProtocolConn
	url  *ParsedURL
}

// Dial connects to a remote tin server
func Dial(rawURL string) (*Client, error) {
	url, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("tcp", url.Address())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", url.Address(), err)
	}

	return &Client{
		conn: conn,
		pc:   NewProtocolConn(conn),
		url:  url,
	}, nil
}

// Close closes the connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Push pushes commits and threads to the remote and updates refs
func (c *Client) Push(repo *storage.Repository, branch string, force bool) error {
	// Send hello
	hello := HelloMessage{
		Version:   ProtocolVersion,
		Operation: "push",
		RepoPath:  c.url.Path,
	}
	if err := c.pc.Send(MsgHello, hello); err != nil {
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Receive refs
	msg, err := c.pc.Receive()
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

	// Build set of remote objects for quick lookup
	remoteCommits := make(map[string]bool)
	for _, id := range remoteRefs.CommitIDs {
		remoteCommits[id] = true
	}
	remoteThreads := make(map[string]bool)
	for _, id := range remoteRefs.ThreadIDs {
		remoteThreads[id] = true
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
		for _, ref := range commit.Threads {
			if !remoteThreads[ref.ThreadID] && threadsToSend[ref.ThreadID] == nil {
				thread, err := repo.LoadThread(ref.ThreadID)
				if err != nil {
					return fmt.Errorf("failed to load thread %s: %w", ref.ThreadID, err)
				}
				threadsToSend[ref.ThreadID] = thread
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
	if err := c.pc.Send(MsgPack, pack); err != nil {
		return fmt.Errorf("failed to send pack: %w", err)
	}

	// Send ref updates
	updateRefs := UpdateRefsMessage{
		Updates: map[string]string{branch: localCommitID},
		Force:   force,
	}
	if err := c.pc.Send(MsgUpdateRefs, updateRefs); err != nil {
		return fmt.Errorf("failed to send update-refs: %w", err)
	}

	// Wait for response
	msg, err = c.pc.Receive()
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

// Pull pulls commits and threads from the remote
func (c *Client) Pull(repo *storage.Repository, branch string) (*RefsMessage, error) {
	// Send hello
	hello := HelloMessage{
		Version:   ProtocolVersion,
		Operation: "pull",
		RepoPath:  c.url.Path,
	}
	if err := c.pc.Send(MsgHello, hello); err != nil {
		return nil, fmt.Errorf("failed to send hello: %w", err)
	}

	// Receive refs
	msg, err := c.pc.Receive()
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

	// Send want
	want := WantMessage{
		CommitIDs: wantCommits,
		ThreadIDs: wantThreads,
	}
	if err := c.pc.Send(MsgWant, want); err != nil {
		return nil, fmt.Errorf("failed to send want: %w", err)
	}

	// Receive pack
	msg, err = c.pc.Receive()
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
		c := commit
		if err := repo.SaveCommit(&c); err != nil {
			return nil, fmt.Errorf("failed to save commit: %w", err)
		}
	}

	// Send OK
	c.pc.SendOK("received")

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
