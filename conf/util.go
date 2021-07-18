package conf

import (
	"krpc/codec"
	"time"
)

const MagicNumber = 0x3bef5b

// Option Client tell Server, what kind of CodeType to use, then use this type to decode/encode.
type Option struct {
	MagicNumber       int            // marks this is a krpc request
	CodeType          codec.CodeType // client may choose different Codec to encode body
	ConnectionTimeout time.Duration
	HandleTimeout     time.Duration
}

var DefaultOption = &Option{
	MagicNumber:       MagicNumber,
	CodeType:          codec.GobType,
	ConnectionTimeout: time.Second * 10,
}