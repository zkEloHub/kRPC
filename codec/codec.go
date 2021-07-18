package codec

import "io"

// Header in between client & server.
type Header struct {
	ServiceMethod string // format "Service.Method"
	Seq           uint64 // sequence number chose by client
	Error         string // modify: error type is interface, can't encode by gob.
}

// Codec To implement different Codec
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// NewCodecFunc return Codec interface.
type NewCodecFunc func(closer io.ReadWriteCloser) Codec

type CodeType string

const (
	GobType  CodeType = "application/gob"
	JsonType CodeType = "application/json" // not implemented
)

var NewCodecFuncMap map[CodeType]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[CodeType]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
