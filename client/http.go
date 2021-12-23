package client

import (
	"bufio"
	"crypto/tls"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

type Proxy interface {
	Handle(rw io.ReadWriteCloser)
}

// HTTPProxy is a proxy implementation to pass http requests.
type HTTPProxy struct {
	dialHost   string
	backendUrl *url.URL
	tlsConfig  *tls.Config
}

// NewHTTPProxy parses the backend url and creates a new proxy for it
func NewHTTPProxy(backendUrl *url.URL) *HTTPProxy {
	host, port, _ := net.SplitHostPort(backendUrl.Host)

	if port == "" {
		port = "80"
	}

	tlsConfig := &tls.Config{ServerName: host, InsecureSkipVerify: true}
	dialHost := net.JoinHostPort(host, port)

	return &HTTPProxy{
		dialHost:   dialHost,
		backendUrl: backendUrl,
		tlsConfig:  tlsConfig,
	}
}

func (p *HTTPProxy) acceptConnections(ch <-chan ssh.NewChannel) {
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

		go p.Handle(ch)
	}
}

// Handle a request from the ssh channel and forwards it to the local http server
func (p *HTTPProxy) Handle(rw io.ReadWriteCloser) {
	tcpConn, err := net.Dial("tcp", p.dialHost)

	if err != nil {
		rw.Close()
		return
	}

	if p.backendUrl.Scheme == "https" || p.backendUrl.Scheme == "wss" {
		// Wrap with TLS
		tcpConn = tls.Client(tcpConn, p.tlsConfig)
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
	headers.Set("Host", p.backendUrl.Host)
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
