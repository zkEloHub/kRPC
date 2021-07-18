package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec implement Codec interface
type GobCodec struct {
	// read, write, close func interface.
	conn io.ReadWriteCloser
	// buffering for io.Writer. Should call Flush.
	buf *bufio.Writer
	// (*io.Reader)
	dec *gob.Decoder
	// (*io.Writer)
	enc *gob.Encoder
}

// todo: ?? syntax
var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	gobCodec := &GobCodec{
		conn: conn,
		buf: buf,
		dec: gob.NewDecoder(conn),
		enc: gob.NewEncoder(buf),
	}
	return gobCodec
}

// ReadHeader read from io.Reader & decode in h
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// ReadBody read from io.Reader & decode in body
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write write to io.Writer & encode
func (c *GobCodec) Write(h *Header, body interface{}) error {
	var err error
	defer func() {
		// write with buffer, need flush
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err = c.enc.Encode(h); err != nil {
		log.Printf("encoding header error: %s\n", err)
		return err
	}
	if err = c.enc.Encode(body); err != nil {
		log.Printf("encoding body error: %s\n", body)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}
