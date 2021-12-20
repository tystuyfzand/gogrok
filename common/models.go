package common

const (
	ForwardedHTTPChannelType = "forwarded-http"
)

type RemoteForwardRequest struct {
	RequestedHost string
}

type RemoteForwardSuccess struct {
	Host string
}

type RemoteForwardCancelRequest struct {
	Host string
}

type RemoteForwardChannelData struct {
	Host     string
	ClientIP string
}
