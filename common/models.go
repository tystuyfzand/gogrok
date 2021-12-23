package common

const (
	ForwardedHTTPChannelType = "forwarded-http"
)

// RemoteForwardRequest represents a forwarding request
type RemoteForwardRequest struct {
	RequestedHost string
	Force         bool
}

// RemoteForwardSuccess returns when a successful request is processed
// Host represents the assigned remote host
type RemoteForwardSuccess struct {
	Host string
}

// RemoteForwardCancelRequest represents a forwarding cancel request
type RemoteForwardCancelRequest struct {
	Host string
}

// RemoteForwardChannelData is sent when opening a channel to say which host/client ip is accessed
type RemoteForwardChannelData struct {
	Host     string
	ClientIP string
}

// HostRegisterRequest is used when registering a host
type HostRegisterRequest struct {
	Host string
}

// HostRegisterSuccess is the response from the server for a Claim request
type HostRegisterSuccess struct {
	Host string
}
