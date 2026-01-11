package remote

import (
	"fmt"
	"net"
)

// TCPTransport implements Transport over a raw TCP connection
type TCPTransport struct {
	conn net.Conn
	pc   *ProtocolConn
}

// NewTCPTransport creates a new TCP transport connected to the given URL
func NewTCPTransport(url *ParsedURL) (*TCPTransport, error) {
	conn, err := net.Dial("tcp", url.Address())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", url.Address(), err)
	}

	return &TCPTransport{
		conn: conn,
		pc:   NewProtocolConn(conn),
	}, nil
}

// Send sends a message with the given type and payload
func (t *TCPTransport) Send(msgType MessageType, payload any) error {
	return t.pc.Send(msgType, payload)
}

// Receive reads and returns the next message
func (t *TCPTransport) Receive() (*Message, error) {
	return t.pc.Receive()
}

// Close closes the TCP connection
func (t *TCPTransport) Close() error {
	return t.conn.Close()
}

// SendError sends an error message
func (t *TCPTransport) SendError(code, message string) error {
	return t.pc.SendError(code, message)
}

// SendOK sends an OK message
func (t *TCPTransport) SendOK(message string) error {
	return t.pc.SendOK(message)
}
