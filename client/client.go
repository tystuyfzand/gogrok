package client

import (
	"errors"
	"gogrok.ccatss.dev/common"
	"golang.org/x/crypto/ssh"
	"net"
	"net/url"
)

var (
	ErrUnsupportedBackend = errors.New("unsupported backend type")
)

// New creates a new client with the specified server and backend
func New(server string, signer ssh.Signer) *Client {
	return &Client{
		server: server,
		signer: signer,
	}
}

// Client is a remote tunnel client
type Client struct {
	conn *ssh.Client

	server string
	signer ssh.Signer
}

// Open opens a connection to the server
// Note: This is called automatically on client operations.
func (c *Client) Open() error {
	if c.conn != nil {
		return nil
	}

	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(c.signer),
		},
	}

	conn, err := ssh.Dial("tcp", c.server, config)

	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

// Start connects to the server over TCP and starts the tunnel
func (c *Client) Start(backend, requestedHost string) (string, error) {
	if err := c.Open(); err != nil {
		return "", err
	}

	backendUrl, err := url.Parse(backend)

	if err != nil {
		return "", nil
	}

	if backendUrl.Scheme == "" {
		backendUrl.Scheme = "http"
	}

	if backendUrl.Scheme == "http" || backendUrl.Scheme == "https" {
		proxy := NewHTTPProxy(backendUrl)

		return c.StartHTTPForwarding(proxy, requestedHost)
	}

	return "", ErrUnsupportedBackend
}

// Register a host as reserved with the server
func (c *Client) Register(host string) error {
	if err := c.Open(); err != nil {
		return err
	}

	payload := ssh.Marshal(common.HostRegisterRequest{
		Host: host,
	})

	success, replyData, err := c.conn.SendRequest(common.HttpRegisterHost, true, payload)

	if err != nil {
		return err
	}

	if !success {
		return errors.New(string(replyData))
	}

	var res common.HostRegisterSuccess

	if err = ssh.Unmarshal(replyData, &res); err != nil {
		return err
	}

	return nil
}

// Unregister a reserved host with the server
func (c *Client) Unregister(host string) error {
	if err := c.Open(); err != nil {
		return err
	}

	payload := ssh.Marshal(common.HostRegisterRequest{
		Host: host,
	})

	success, replyData, err := c.conn.SendRequest(common.HttpUnregisterHost, true, payload)

	if err != nil {
		return err
	}

	if !success {
		return errors.New(string(replyData))
	}

	var res common.HostRegisterSuccess

	if err = ssh.Unmarshal(replyData, &res); err != nil {
		return err
	}

	return nil
}

// StartHTTPForwarding starts a basic http proxy/forwarding service
func (c *Client) StartHTTPForwarding(proxy *HTTPProxy, requestedHost string) (string, error) {
	payload := ssh.Marshal(common.RemoteForwardRequest{
		RequestedHost: requestedHost,
	})

	success, replyData, err := c.conn.SendRequest(common.HttpForward, true, payload)

	if err != nil {
		return "", err
	}

	if !success {
		return "", errors.New(string(replyData))
	}

	var response common.RemoteForwardSuccess

	if err := ssh.Unmarshal(replyData, &response); err != nil {
		return "", err
	}

	ch := c.conn.HandleChannelOpen(common.ForwardedHTTPChannelType)

	go func() {
		proxy.acceptConnections(ch)

		payload := ssh.Marshal(common.RemoteForwardCancelRequest{Host: response.Host})
		c.conn.SendRequest(common.CancelHttpForward, false, payload)
	}()

	return response.Host, nil
}
