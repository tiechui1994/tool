package ctx

type Listener interface {
	RawAddress() string
	Address() string
	Close() error
}
