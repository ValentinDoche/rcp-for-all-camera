package rcpmod

// The wire protocol between the application (host) and a module. Every
// message is JSON. The host calls the module's rcp_call export with a
// Request and reads back a Response; the module calls the host's rcp.call
// import with a HostRequest and reads back a HostResponse (fetched with
// rcp.take). docs/modules-camera.html (ABI annex) describes the exports
// and imports; these types are the reference for the fields.

// Ops of Request (host to module).
const (
	OpInfo      = "info"
	OpNew       = "new"
	OpControls  = "controls"
	OpPage      = "page"
	OpConnect   = "connect"
	OpClose     = "close"
	OpGet       = "get"
	OpSet       = "set"
	OpWSMessage = "wsMessage"
	OpWSClosed  = "wsClosed"
	OpTimer     = "timer"
)

// Request is a call from the host to the module.
type Request struct {
	Op     string  `json:"op"`
	Config *Config `json:"config,omitempty"` // OpNew
	ID     string  `json:"id,omitempty"`     // OpGet, OpSet
	Value  *Value  `json:"value,omitempty"`  // OpSet
	Socket int     `json:"socket,omitempty"` // OpWSMessage, OpWSClosed
	Text   string  `json:"text,omitempty"`   // OpWSMessage
	Error  string  `json:"error,omitempty"`  // OpWSClosed: why it closed ("" = closed by the module)
	Timer  int     `json:"timer,omitempty"`  // OpTimer
}

// Response is the module's answer. Exactly one of Result and Error is set
// for a call that returns something; both are empty for a call that only
// succeeds.
type Response struct {
	// Result is the op's result: Info (OpInfo), []ControlSpec (OpControls),
	// Page (OpPage) or Value (OpGet).
	Result any        `json:"result,omitempty"`
	Error  *WireError `json:"error,omitempty"`
}

// WireError is an error crossing the boundary: a code (CodeInvalidValue...)
// and a message for the log.
type WireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Ops of HostRequest (module to host).
const (
	HostHTTP    = "http"
	HostWSOpen  = "wsOpen"
	HostWSSend  = "wsSend"
	HostWSClose = "wsClose"
	HostAfter   = "after"
	HostCancel  = "cancel"
	HostEmit    = "emit"
)

// HostRequest is a call from the module to the host.
type HostRequest struct {
	Op     string `json:"op"`
	Method string `json:"method,omitempty"` // HostHTTP
	Path   string `json:"path,omitempty"`   // HostHTTP, HostWSOpen
	Body   []byte `json:"body,omitempty"`   // HostHTTP
	Socket int    `json:"socket,omitempty"` // HostWSSend, HostWSClose
	Text   string `json:"text,omitempty"`   // HostWSSend
	Ms     int64  `json:"ms,omitempty"`     // HostAfter
	Timer  int    `json:"timer,omitempty"`  // HostCancel
	Event  *Event `json:"event,omitempty"`  // HostEmit
}

// HostResponse is the host's answer.
type HostResponse struct {
	Status int    `json:"status,omitempty"` // HostHTTP
	Body   []byte `json:"body,omitempty"`   // HostHTTP
	Socket int    `json:"socket,omitempty"` // HostWSOpen
	Timer  int    `json:"timer,omitempty"`  // HostAfter
	Error  string `json:"error,omitempty"`  // any op that failed
}
