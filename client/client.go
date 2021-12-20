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
    "net/url"
)

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

    host, port, _ := net.SplitHostPort(backendUrl.Host)

    if port == "" {
        port = "80"
    }

    dialHost := net.JoinHostPort(host, port)

    return &Client{
        config: config,
        server: server,
        backendUrl: backendUrl,
        dialHost: dialHost,
        tlsConfig: &tls.Config{ServerName: host, InsecureSkipVerify: true},
    }
}

type Client struct {
    config *ssh.ClientConfig
    tlsConfig *tls.Config

    server string
    backendUrl *url.URL
    dialHost string
}

func (c *Client) Start() error {
    log.WithFields(log.Fields{
        "server": c.server,
    }).Info("Dialing server")

    conn, err := ssh.Dial("tcp", c.server, c.config)

    if err != nil {
        return err
    }

    payload := ssh.Marshal(common.RemoteForwardRequest{
        RequestedHost: "",
    })

    success, replyData, err := conn.SendRequest("http-forward", true, payload)

    if !success || err != nil {
        log.WithError(err).Fatalln("Unable to start forward request")
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

    return nil
}

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

    req, err := http.ReadRequest(bufferedCh)

    if err != nil {
        log.WithError(err).Warning("Unable to read request header from ch")
        return
    }

    // TODO: By parsing the request, we're overriding some fields. Perhaps we want to read only the header and then write the body manually?

    // Override host
    req.URL.Scheme = c.backendUrl.Scheme
    req.URL.Host = c.backendUrl.Host
    req.Host = c.backendUrl.Host

    go req.Write(tcpConn)

    io.Copy(rw, tcpConn)
}