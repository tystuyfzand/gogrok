package server

import (
    "bufio"
    "github.com/gliderlabs/ssh"
    log "github.com/sirupsen/logrus"
    "gogrok.ccatss.dev/common"
    gossh "golang.org/x/crypto/ssh"
    "io"
    "net/http"
    "net/textproto"
    "strconv"
    "strings"
    "sync"
)

// HostProvider is a func to provide a host + subdomain
type HostProvider func() string

// HostValidator validates hosts (for example, on a subdomain)
type HostValidator func(host string) bool

// ForwardedHTTPHandler can be enabled by creating a ForwardedTCPHandler and
// adding the HandleSSHRequest callback to the server's RequestHandlers under
// tcpip-forward and cancel-tcpip-forward.
type ForwardedHTTPHandler struct {
    forwards map[string]*gossh.ServerConn
    provider HostProvider
    validator HostValidator
    sync.RWMutex
}

type HandlerOption func(h *ForwardedHTTPHandler)

func WithProvider(provider HostProvider) HandlerOption {
    return func(h *ForwardedHTTPHandler) {
        h.provider = provider
    }
}

func WithValidator(validator HostValidator) HandlerOption {
    return func(h *ForwardedHTTPHandler) {
        h.validator = validator
    }
}

func NewHttpHandler(opts ...HandlerOption) ForwardHandler {
    h := &ForwardedHTTPHandler{
        forwards: make(map[string]*gossh.ServerConn),
        provider: RandomAnimal,
    }

    for _, opt := range opts {
        opt(h)
    }

    return h
}

func (h *ForwardedHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    h.RLock()
    sshConn, ok := h.forwards[r.Host]
    h.RUnlock()

    if !ok {
        log.Warning("Unknown host ", r.Host)
        http.Error(w, "not found", http.StatusNotFound)
        return
    }

    payload := gossh.Marshal(&common.RemoteForwardChannelData{
        Host: r.Host,
        ClientIP: r.RemoteAddr,
    })

    ch, reqs, err := sshConn.OpenChannel(common.ForwardedHTTPChannelType, payload)

    if err != nil {
        return
    }

    go gossh.DiscardRequests(reqs)

    defer ch.Close()

    // Ensure we have Connection: close, keep alive isn't supported
    r.Header.Set("Connection", "close")

    // Write the request to our channel
    r.Write(ch)

    // Read the response
    bufReader := bufio.NewReader(ch)

    tp := textproto.NewReader(bufReader)

    var s string
    if s, err = tp.ReadLine(); err != nil {
        w.WriteHeader(http.StatusBadGateway)
        return
    }

    _, responseCodeStr, _, ok := parseResponseLine(s)

    if !ok {
        w.WriteHeader(http.StatusBadGateway)
        return
    }

    responseCode, err := strconv.Atoi(responseCodeStr)

    if responseCode < http.StatusContinue || responseCode > http.StatusNetworkAuthenticationRequired {
        return
    }

    mimeHeader, err := tp.ReadMIMEHeader()

    if err != nil {
        return
    }

    for k, v := range mimeHeader {
        w.Header()[k] = v
    }

    // Set our forwarded address
    forwardedHeader := r.Header.Get("X-Forwarded-For")

    // TODO: Only trust this from trusted sources.
    if forwardedHeader != "" {
        forwardedHeader = strings.Join(append([]string{r.RemoteAddr}, strings.Split(",", forwardedHeader)...), ",")
    } else {
        forwardedHeader = r.RemoteAddr
    }

    w.Header().Set("X-Forwarded-For", forwardedHeader)

    if r.TLS != nil {
        w.Header().Set("X-Forwarded-Proto", "https")
    }

    w.WriteHeader(responseCode)

    io.Copy(w, bufReader)
}

// parseResponseLine parses "HTTP/1.1 200 OK" into its three parts.
func parseResponseLine(line string) (httpVersion, responseCode, responseText string, ok bool) {
    s1 := strings.Index(line, " ")
    s2 := strings.Index(line[s1+1:], " ")
    if s1 < 0 || s2 < 0 {
        return
    }
    s2 += s1 + 1
    return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

// HandleSSHRequest handles incoming ssh requests.
func (h *ForwardedHTTPHandler) HandleSSHRequest(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
    conn := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)

    switch req.Type {
    case "http-forward":
        var reqPayload common.RemoteForwardRequest
        if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
            // TODO: log parse failure
            log.WithError(err).Warning("Error parsing payload for http-forward")
            return false, []byte{}
        }

        host := reqPayload.RequestedHost

        if host != "" && h.validator != nil && !h.validator(host) {
            return false, []byte("invalid host " + host)
        }

        // Validate host
        if host == "" {
            host = h.provider()
        }

        h.RLock()
        for {
            _, exists := h.forwards[host]

            if !exists {
                break
            }

            host = h.provider()
        }
        h.RUnlock()

        h.Lock()
        h.forwards[host] = conn
        h.Unlock()
        log.WithField("host", host).Info("Registered host")

        go func() {
            <-ctx.Done()

            log.WithField("host", host).Info("Removed host")
            h.Lock()
            delete(h.forwards, host)
            h.Unlock()
        }()

        return true, gossh.Marshal(&common.RemoteForwardSuccess{
            Host: host,
        })

    case "cancel-http-forward":
        var reqPayload common.RemoteForwardCancelRequest
        if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
            // TODO: log parse failure
            return false, []byte{}
        }
        h.Lock()
        delete(h.forwards, reqPayload.Host)
        h.Unlock()
        return true, nil

    case "http-register-forward":
        var reqPayload common.RemoteForwardRequest
        if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
            // TODO: log parse failure
            log.WithError(err).Warning("Error parsing payload for http-register-forward")
            return false, []byte{}
        }

        log.Println("Key:", ctx.Value("publicKey"))

        // Claimed forward via SSH Public Key
        return true, nil
    default:
        return false, nil
    }
}
