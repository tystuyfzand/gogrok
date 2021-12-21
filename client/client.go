package client

import (
	"bufio"
	"crypto/tls"
	log "github.com/sirupsen/logrus"
	"gogrok.ccatss.dev/common"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

// New creates a new client with the specified server and backend
func New(server, backend string, signer ssh.Signer) *Client {
	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	backendUrl, err := url.Parse(backend)

	if err != nil {
		return nil
	}

	if backendUrl.Scheme == "" {
		backendUrl.Scheme = "http"
	}

	host, port, _ := net.SplitHostPort(backendUrl.Host)

	if port == "" {
		port = "80"
	}

	dialHost := net.JoinHostPort(host, port)

	return &Client{
		config:     config,
		server:     server,
		backendUrl: backendUrl,
		dialHost:   dialHost,
		tlsConfig:  &tls.Config{ServerName: host, InsecureSkipVerify: true},
	}
}

// Client is a remote tunnel client
type Client struct {
	config    *ssh.ClientConfig
	tlsConfig *tls.Config

	server     string
	backendUrl *url.URL
	dialHost   string
}

// Start connects to the server over TCP and starts the tunnel
func (c *Client) Start() error {
	log.WithFields(log.Fields{
		"server": c.server,
	}).Info("Dialing server")

	conn, err := ssh.Dial("tcp", c.server, c.config)

	if err != nil {
		return err
	}

	if c.backendUrl.Scheme == "http" || c.backendUrl.Scheme == "https" {
		go c.handleHttpRequests(conn)
	}

	return nil
}

func (c *Client) handleHttpRequests(conn *ssh.Client) {
	payload := ssh.Marshal(common.RemoteForwardRequest{
		RequestedHost: "",
	})

	success, replyData, err := conn.SendRequest("http-forward", true, payload)

	if !success || err != nil {
		log.WithFields(log.Fields{
			"error":   err,
			"message": string(replyData),
		}).Fatalln("Unable to start forwarding")
	}

	log.WithFields(log.Fields{
		"success": success,
	}).Info("Got response")

	var response common.RemoteForwardSuccess

	if err := ssh.Unmarshal(replyData, &response); err != nil {
		log.WithError(err).Fatalln("Unable to unmarshal data")
	}

	defer func() {
		payload := ssh.Marshal(common.RemoteForwardCancelRequest{Host: response.Host})
		conn.SendRequest("cancel-http-forward", false, payload)
	}()

	log.WithField("host", response.Host).Info("Bound host")

	ch := conn.HandleChannelOpen(common.ForwardedHTTPChannelType)

	for {
		newCh := <-ch

		if newCh == nil {
			break
		}

		ch, r, err := newCh.Accept()

		if err != nil {
			log.WithError(err).Warning("Error accepting channel")
			continue
		}

		go ssh.DiscardRequests(r)

		go c.proxyRequest(ch)
	}
}

// proxyRequest handles a request from the ssh channel and forwards it to the local http server
func (c *Client) proxyRequest(rw io.ReadWriteCloser) {
	tcpConn, err := net.Dial("tcp", c.dialHost)

	if err != nil {
		rw.Close()
		return
	}

	if c.backendUrl.Scheme == "https" || c.backendUrl.Scheme == "wss" {
		// Wrap with TLS
		tcpConn = tls.Client(tcpConn, c.tlsConfig)
	}

	defer rw.Close()
	defer tcpConn.Close()

	bufferedCh := bufio.NewReader(rw)

	tp := textproto.NewReader(bufferedCh)

	var s string
	if s, err = tp.ReadLine(); err != nil {
		return
	}

	// Write the first response line as-is
	tcpConn.Write([]byte(s + "\r\n"))

	// Read headers and proxy each to the output
	mimeHeader, err := tp.ReadMIMEHeader()

	if err != nil {
		return
	}

	// Modify and return our headers
	headers := http.Header(mimeHeader)
	headers.Set("Host", c.backendUrl.Host)
	headers.Write(tcpConn)

	// End headers
	tcpConn.Write([]byte("\r\n"))

	contentLength, err := strconv.ParseInt(headers.Get("Content-Length"), 10, 64)

	if err == nil && contentLength > 0 {
		// Copy request to the tcpConn
		_, err := io.Copy(tcpConn, io.LimitReader(bufferedCh, int64(contentLength)))

		if err != nil {
			log.WithError(err).Warning("Connection error on body read, closing")
			return
		}
	}

	// Copy the response back to the tunnel server
	io.Copy(rw, tcpConn)
}
