package transport

// Transport abstracts communication with a printer device.
type Transport interface {
	Name() string
	Open(address string) error
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close() error
}
