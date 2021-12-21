package server

import (
	"crypto/dsa"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gogrok.ccatss.dev/common"
	gossh "golang.org/x/crypto/ssh"
	"io"
	"net/http"
	"strings"
)

// ForwardHandler is an interface defining the handler type for forwarding
type ForwardHandler interface {
	HandleSSHRequest(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte)
}

// Server is a struct containing our ssh server, forwarding handler, and other attributes
type Server struct {
	sshServer      *ssh.Server
	forwardHandler ForwardHandler

	sshBindAddress string
	hostSigners    []ssh.Signer

	authorizedKeys []string
}

// Option defines types for server options
type Option func(s *Server)

// WithForwardHandler lets custom forwarding handlers be registered.
// This will support multiple handlers eventually, for HTTP, TCP, etc.
func WithForwardHandler(f ForwardHandler) Option {
	return func(s *Server) {
		s.forwardHandler = f
	}
}

// WithSSHAddress sets the SSH bind address.
func WithSSHAddress(address string) Option {
	return func(s *Server) {
		s.sshBindAddress = address
	}
}

// WithSigner sets the host signer for the server
func WithSigner(signer gossh.Signer) Option {
	return func(s *Server) {
		s.hostSigners = []ssh.Signer{signer}
	}
}

// WithDSAKey allows a DSA key to be used for the server.
// This is a convenience helper with gossh.NewSignerFromKey.
func WithDSAKey(key *dsa.PrivateKey) Option {
	signer, err := gossh.NewSignerFromKey(key)

	if err != nil {
		panic(err)
	}

	return func(s *Server) {
		s.hostSigners = []ssh.Signer{signer}
	}
}

// WithAuthorizedKeys sets the authorized keys to connect to this server
func WithAuthorizedKeys(authorizedKeys []string) Option {
	return func(s *Server) {
		s.authorizedKeys = authorizedKeys
	}
}

// New creates a new Server instance with a range of options.
func New(options ...Option) (*Server, error) {
	s := &Server{}

	for _, opt := range options {
		opt(s)
	}

	if s.forwardHandler == nil {
		s.forwardHandler = NewHttpHandler(WithProvider(RandomAnimal))
	}

	if s.hostSigners == nil || len(s.hostSigners) < 1 {
		key, err := common.GenRSA(4096)

		if err != nil {
			return nil, errors.Wrap(err, "unable to load or generate server key")
		}

		signer, err := gossh.NewSignerFromKey(key)

		if err != nil {
			return nil, errors.Wrap(err, "unable to create signer from key")
		}

		s.hostSigners = []ssh.Signer{signer}
	}

	requestHandlers := make(map[string]ssh.RequestHandler)

	// TODO: Add TCP handler using the same idea, potentially support multiple forwardHandlers

	if _, ok := s.forwardHandler.(http.Handler); ok {
		requestHandlers["http-forward"] = s.forwardHandler.HandleSSHRequest
		requestHandlers["cancel-http-forward"] = s.forwardHandler.HandleSSHRequest
	}

	s.sshServer = &ssh.Server{
		HostSigners:      s.hostSigners,
		Addr:             s.sshBindAddress,
		Handler:          s.sshHandler,
		RequestHandlers:  requestHandlers,
		PublicKeyHandler: s.publicKeyHandler,
	}

	return s, nil
}

// SetAuthorizedKeys is exposed as a way to set/update authorized keys during runtime
func (s *Server) SetAuthorizedKeys(authorizedKeys []string) {
	s.authorizedKeys = authorizedKeys
}

// sshHandler is our basic ssh handler to deny regular ssh sessions.
func (s *Server) sshHandler(session ssh.Session) {
	supportedTypes := []string{"http", "https", "ws", "wss"}

	io.WriteString(session, "This server supports only remote forwarding of request types: "+strings.Join(supportedTypes, ", ")+"\n")
	io.WriteString(session, "For more information, visit https://gogrok.ccatss.dev\n")
	session.Close()
}

// publicKeyHandler handles public keys when authenticating.
// It can be used to authorize based on public keys, or (in the future) register/reserve domains via public key.
func (s *Server) publicKeyHandler(ctx ssh.Context, pubkey ssh.PublicKey) bool {
	keyMarshalled := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(pubkey)))

	log.WithFields(log.Fields{
		"key":        keyMarshalled,
		"sessionID":  ctx.SessionID(),
		"remoteAddr": ctx.RemoteAddr(),
	}).Debug("Client is attempting public key auth")

	ctx.SetValue("publicKey", pubkey)

	if s.authorizedKeys != nil {
		for _, key := range s.authorizedKeys {
			if key == keyMarshalled {
				return true
			}
		}

		return false
	}

	return true
}

// Start will start the SSH server.
func (s *Server) Start() error {
	return s.sshServer.ListenAndServe()
}

// StartHTTP is a convenience method to start a basic http server.
// This uses s.forwardHandler if http.Handler is implemented to serve requests.
func (s *Server) StartHTTP(bind string) error {
	if h, ok := s.forwardHandler.(http.Handler); ok {
		httpServer := &http.Server{
			Addr:    bind,
			Handler: h,
		}

		return httpServer.ListenAndServe()
	}

	return errors.New("forwarding handler doesn't support http")
}

// ServeHTTP is a passthrough to forwardHandler's ServeHTTP
// This can be used to use your own http server implementation, or for TLS/etc
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h, ok := s.forwardHandler.(http.Handler); ok {
		h.ServeHTTP(w, r)
	}
}
