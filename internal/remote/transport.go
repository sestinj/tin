package remote

// Transport provides a communication channel for the TIN protocol.
// Different implementations handle TCP, HTTPS, etc.
type Transport interface {
	// Send sends a message with the given type and payload
	Send(msgType MessageType, payload any) error

	// Receive reads and returns the next message
	Receive() (*Message, error)

	// Close closes the transport
	Close() error
}

// Credentials holds authentication information for a remote
type Credentials struct {
	Username string // Can be anything for token auth, often "x-token-auth"
	Password string // The actual token (th_xxx)
}
