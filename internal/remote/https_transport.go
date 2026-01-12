package remote

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// ContentTypeTinProtocol is the content type for TIN protocol messages
	ContentTypeTinProtocol = "application/x-tin-protocol"
)

// HTTPSTransport implements Transport over HTTPS
type HTTPSTransport struct {
	client    *http.Client
	baseURL   string
	creds     *Credentials
	operation string // Set from HelloMessage
	repoPath  string // Set from HelloMessage (not used for HTTP, but stored)

	// Message buffering for request/response batching
	sendBuf []Message
	recvBuf []Message
}

// NewHTTPSTransport creates a new HTTPS transport
func NewHTTPSTransport(url *ParsedURL, creds *Credentials) (*HTTPSTransport, error) {
	// Build base URL
	port := url.Port
	if port == "" || port == "443" {
		// Don't include default port in URL
		port = ""
	}

	var baseURL string
	if port != "" {
		baseURL = fmt.Sprintf("https://%s:%s%s", url.Host, port, url.Path)
	} else {
		baseURL = fmt.Sprintf("https://%s%s", url.Host, url.Path)
	}

	return &HTTPSTransport{
		client:  &http.Client{},
		baseURL: strings.TrimSuffix(baseURL, "/"),
		creds:   creds,
		sendBuf: make([]Message, 0),
		recvBuf: make([]Message, 0),
	}, nil
}

// Send buffers a message. The actual HTTP request happens on Receive.
func (t *HTTPSTransport) Send(msgType MessageType, payload any) error {
	// Encode payload
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	msg := Message{
		Type:    msgType,
		Payload: payloadBytes,
	}

	// Extract operation from hello message but don't send it
	// (HTTP uses endpoints to determine operation, not Hello message)
	if msgType == MsgHello {
		var hello HelloMessage
		if err := json.Unmarshal(payloadBytes, &hello); err == nil {
			t.operation = hello.Operation
			t.repoPath = hello.RepoPath
		}
		// Don't buffer Hello for HTTP - the endpoint indicates the operation
		return nil
	}

	t.sendBuf = append(t.sendBuf, msg)
	return nil
}

// Receive returns the next message from the response buffer.
// If the buffer is empty, it flushes pending sends via HTTP and fills the buffer.
func (t *HTTPSTransport) Receive() (*Message, error) {
	// If we have buffered responses, return the next one
	if len(t.recvBuf) > 0 {
		msg := t.recvBuf[0]
		t.recvBuf = t.recvBuf[1:]
		return &msg, nil
	}

	// No buffered responses - flush pending sends and get response
	// For HTTP, we allow empty sendBuf - this triggers a "get refs" request

	// Make HTTP request
	responses, err := t.doRequest()
	if err != nil {
		return nil, err
	}

	// Buffer all responses
	t.recvBuf = responses

	// Return first response
	if len(t.recvBuf) == 0 {
		return nil, fmt.Errorf("server returned no messages")
	}

	msg := t.recvBuf[0]
	t.recvBuf = t.recvBuf[1:]
	return &msg, nil
}

// doRequest sends buffered messages as an HTTP POST and returns response messages
func (t *HTTPSTransport) doRequest() ([]Message, error) {
	// Determine endpoint based on operation
	endpoint := t.endpointForOperation(t.operation)

	// Encode request messages as newline-delimited JSON
	var body bytes.Buffer
	for _, msg := range t.sendBuf {
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message: %w", err)
		}
		body.Write(data)
		body.WriteByte('\n')
	}

	// Clear send buffer
	t.sendBuf = t.sendBuf[:0]

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", ContentTypeTinProtocol)
	req.Header.Set("Accept", ContentTypeTinProtocol)

	// Add Basic Auth if credentials available
	if t.creds != nil && t.creds.Password != "" {
		username := t.creds.Username
		if username == "" {
			username = "x-token-auth"
		}
		auth := base64.StdEncoding.EncodeToString(
			[]byte(username + ":" + t.creds.Password),
		)
		req.Header.Set("Authorization", "Basic "+auth)
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP-level errors
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication required: server returned 401 Unauthorized")
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response messages (newline-delimited JSON)
	return t.parseResponse(resp.Body)
}

// endpointForOperation returns the HTTP endpoint for the given operation
func (t *HTTPSTransport) endpointForOperation(operation string) string {
	switch operation {
	case "push":
		return t.baseURL + "/tin-receive-pack"
	case "pull":
		return t.baseURL + "/tin-upload-pack"
	case "config":
		return t.baseURL + "/tin-config"
	default:
		return t.baseURL + "/tin-" + operation
	}
}

// parseResponse parses newline-delimited JSON messages from the response body
func (t *HTTPSTransport) parseResponse(r io.Reader) ([]Message, error) {
	var messages []Message
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("failed to decode response message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return messages, nil
}

// Close is a no-op for HTTP (stateless)
func (t *HTTPSTransport) Close() error {
	return nil
}

// SendError sends an error message
func (t *HTTPSTransport) SendError(code, message string) error {
	return t.Send(MsgError, ErrorMessage{Code: code, Message: message})
}

// SendOK sends an OK message
func (t *HTTPSTransport) SendOK(message string) error {
	return t.Send(MsgOK, OKMessage{Message: message})
}
