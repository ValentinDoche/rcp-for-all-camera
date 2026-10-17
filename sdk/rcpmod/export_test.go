package rcpmod

// The dispatcher, for its external tests.
type Dispatcher = dispatcher

// NewDispatcher returns a dispatcher running m, with call as the host.
func NewDispatcher(m Module, call func(HostRequest) (HostResponse, error), log func(Level, string)) *Dispatcher {
	return newDispatcher(m, call, log)
}

// Handle answers one encoded request.
func (d *dispatcher) Handle(in []byte) []byte { return d.handle(in) }
