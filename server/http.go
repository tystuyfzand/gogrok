package server

import (
	"bufio"
	"bytes"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"gogrok.ccatss.dev/common"
	"gogrok.ccatss.dev/server/store"
	gossh "golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HostProvider is a func to provide a host + subdomain
type HostProvider func() string

// HostValidator validates hosts (for example, on a subdomain)
type HostValidator func(host string) bool

// ForwardedHTTPHandler can be enabled by creating a ForwardedTCPHandler and
// adding the HandleSSHRequest callback to the server's RequestHandlers under
// tcpip-forward and cancel-tcpip-forward.
type ForwardedHTTPHandler struct {
	forwards  map[string]*Forward
	provider  HostProvider
	validator HostValidator
	store     store.Store
	sync.RWMutex
}

// Forward contains the forwarded connection
type Forward struct {
	Conn *gossh.ServerConn
	Key  ssh.PublicKey
}

// HandlerOption represents a func used to assign options to a ForwardedHTTPHandler
type HandlerOption func(h *ForwardedHTTPHandler)

// WithProvider sets a default domain provider
func WithProvider(provider HostProvider) HandlerOption {
	return func(h *ForwardedHTTPHandler) {
		h.provider = provider
	}
}

// WithValidator sets a host validator to use for validation of custom hosts
func WithValidator(validator HostValidator) HandlerOption {
	return func(h *ForwardedHTTPHandler) {
		h.validator = validator
	}
}

// WithStore assigns a host store to use for storage
func WithStore(s store.Store) HandlerOption {
	return func(h *ForwardedHTTPHandler) {
		h.store = s
	}
}

func NewHttpHandler(opts ...HandlerOption) ForwardHandler {
	h := &ForwardedHTTPHandler{
		forwards:  make(map[string]*Forward),
		provider:  RandomAnimal,
		validator: DenyAll,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// RequestTypes lets the server know which request types this handler can use
func (h *ForwardedHTTPHandler) RequestTypes() []string {
	return []string{
		common.HttpForward,
		common.CancelHttpForward,
		common.HttpRegisterHost,
		common.HttpUnregisterHost,
	}
}

// ServeHTTP mocks an http server endpoint that uses Request.Host to forward requests
func (h *ForwardedHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.RLock()
	fw, ok := h.forwards[r.Host]
	h.RUnlock()

	if !ok {
		log.Warning("Unknown host ", r.Host)
		http.Error(w, "not found", http.StatusNotFound)
		log.Println("Valid hosts:", h.forwards)
		return
	}

	payload := gossh.Marshal(&common.RemoteForwardChannelData{
		Host:     r.Host,
		ClientIP: r.RemoteAddr,
	})

	ch, reqs, err := fw.Conn.OpenChannel(common.ForwardedHTTPChannelType, payload)

	if err != nil {
		log.WithError(err).Warning("Unable to open ssh connection channel")
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
		log.Warning("Backend returned unexpected response line")
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

func (h *ForwardedHTTPHandler) checkHostOwnership(host, owner string) bool {
	hostModel, err := h.store.Get(host)

	if err != nil {
		return false
	}

	return hostModel.Owner == owner
}

// HandleSSHRequest handles incoming ssh requests.
func (h *ForwardedHTTPHandler) HandleSSHRequest(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	conn := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)

	log.WithField("type", req.Type).Info("Handling request")

	switch req.Type {
	case common.HttpForward:
		return h.handleForwardRequest(ctx, conn, req)
	case common.CancelHttpForward:
		return h.handleCancelRequest(ctx, req)
	case common.HttpRegisterHost:
		return h.handleRegisterRequest(ctx, conn, req)
	case common.HttpUnregisterHost:
		return h.handleUnregisterRequest(ctx, req)
	default:
		return false, nil
	}
}

func (h *ForwardedHTTPHandler) handleForwardRequest(ctx ssh.Context, conn *gossh.ServerConn, req *gossh.Request) (bool, []byte) {
	var reqPayload common.RemoteForwardRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		log.WithError(err).Warning("Error parsing payload for http-forward")
		return false, []byte{}
	}

	pubKey := ctx.Value("publicKey").(ssh.PublicKey)

	keyStr := string(bytes.TrimSpace(gossh.MarshalAuthorizedKey(pubKey)))

	host := strings.ToLower(reqPayload.RequestedHost)

	if host != "" {
		if h.validator != nil && !h.validator(host) {
			return false, []byte("invalid host " + host)
		}

		hostModel, err := h.store.Get(host)

		if hostModel == nil || err != nil {
			return false, []byte("host not registered")
		}

		if hostModel.Owner != keyStr {
			return false, []byte("host claimed and not owned by current key")
		}

		h.RLock()
		current, exists := h.forwards[host]
		h.RUnlock()

		if exists && !reqPayload.Force {
			return false, []byte("host already in use and force not set")
		}

		if exists {
			// Force old connection to close
			current.Conn.Close()
		}

		hostModel.LastUse = time.Now()

		// Save model last use time
		h.store.Add(*hostModel)
	} else {
		host = h.provider()
	}

	// Validate host
	if host == "" {
		h.RLock()
		for {
			host = h.provider()

			_, exists := h.forwards[host]

			if !exists {
				break
			}
		}
		h.RUnlock()
	}

	log.WithField("host", host).Info("Registering host")

	h.Lock()
	h.forwards[host] = &Forward{
		Conn: conn,
		Key:  pubKey,
	}
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
}

func (h *ForwardedHTTPHandler) handleCancelRequest(ctx ssh.Context, req *gossh.Request) (bool, []byte) {
	var reqPayload common.RemoteForwardCancelRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		log.WithError(err).Warning("Error parsing payload for cancel-http-forward")
		return false, []byte{}
	}

	pubKey := ctx.Value("publicKey").(ssh.PublicKey)

	host := strings.ToLower(reqPayload.Host)

	h.RLock()
	fw, exists := h.forwards[host]
	h.RUnlock()

	if !exists {
		return false, []byte("host not found")
	}

	if !bytes.Equal(pubKey.Marshal(), fw.Key.Marshal()) {
		return false, []byte("host not owned by key")
	}

	log.WithField("host", host).Info("Unregistering host")

	h.Lock()
	delete(h.forwards, host)
	h.Unlock()
	return true, nil
}

func (h *ForwardedHTTPHandler) handleRegisterRequest(ctx ssh.Context, conn *gossh.ServerConn, req *gossh.Request) (bool, []byte) {
	var reqPayload common.HostRegisterRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		log.WithError(err).Warning("Error parsing payload for http-register-forward")
		return false, []byte{}
	}

	pubKey := ctx.Value("publicKey").(ssh.PublicKey)

	keyStr := string(bytes.TrimSpace(gossh.MarshalAuthorizedKey(pubKey)))

	host := strings.ToLower(reqPayload.Host)

	if host == "" || h.validator != nil && !h.validator(host) {
		log.WithField("host", host).Warning("Host failed validation")
		return false, []byte("invalid host " + host)
	}

	if h.store.Has(host) {
		log.WithField("host", host).Warning("Host is already taken")
		return false, []byte("host is already taken")
	}

	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	log.WithField("ip", ip).WithField("host", host).Info("Registering host")

	err := h.store.Add(store.Host{
		Host:    host,
		Owner:   keyStr,
		IP:      ip,
		Created: time.Now(),
		LastUse: time.Now(),
	})

	if err != nil {
		return false, []byte(err.Error())
	}

	return true, gossh.Marshal(common.HostRegisterSuccess{
		Host: host,
	})
}

func (h *ForwardedHTTPHandler) handleUnregisterRequest(ctx ssh.Context, req *gossh.Request) (bool, []byte) {
	var reqPayload common.HostRegisterRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		log.WithError(err).Warning("Error parsing payload for http-register-forward")
		return false, []byte{}
	}

	pubKey := ctx.Value("publicKey").(ssh.PublicKey)

	keyStr := string(bytes.TrimSpace(gossh.MarshalAuthorizedKey(pubKey)))

	host := strings.ToLower(reqPayload.Host)

	if host == "" || h.validator != nil && !h.validator(host) {
		return false, []byte("invalid host " + host)
	}

	hostModel, err := h.store.Get(host)

	if hostModel == nil || err != nil {
		return false, []byte(err.Error())
	}

	if hostModel.Owner != keyStr {
		return false, []byte("this host is not owned by you")
	}

	h.store.Remove(host)

	return true, gossh.Marshal(common.HostRegisterSuccess{
		Host: host,
	})
}
