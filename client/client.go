package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"krpc/codec"
	"krpc/conf"
	"log"
	"net"
	"sync"
	"time"
)

// Call a remote call's definition
type Call struct {
	Seq           uint64
	ServiceMethod string      // format "<service>.<method>"
	Args          interface{} // arguments in the function
	Reply         interface{} // reply from the function
	Error         error       // error
	Done          chan *Call  // Strobes when call is complete.
}

// done called when done, to notify client.
func (call *Call) done() {
	call.Done <- call
}

// Client is a RPC client
type Client struct {
	// msg's en & de code, & io tool.
	cc       codec.Codec
	opt      *conf.Option
	// to make the msg orderly.
	// header: sending & header are only needed when sending msg
	sending  sync.Mutex // protect following
	header   codec.Header

	mu       sync.Mutex // protect following
	// send sequence
	seq      uint64
	// registered but not been done by server.
	pending  map[uint64]*Call
	closing  bool // user has called Close
	shutdown bool // error occur
}

type clientResult struct {
	client *Client
	err    error
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection has been shutdown")

// Close the connection
func (client *Client)Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

// IsAvailable return if the client is going
func (client *Client)IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()

	return !client.closing && !client.shutdown
}

// function with Call

func (client *Client)registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client)removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()

	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminateCalls called when client or server have a error.
func (client *Client)terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()

	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

// NewClient init first interactive. **opt**
func NewClient(conn net.Conn, opt *conf.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodeType]
	if f == nil {
		err := fmt.Errorf("invalid code type %s", opt.CodeType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// send options to server. handshake...
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: opt error:", err)
		_ = conn.Close()
		return nil, err
	}
	return NewClientCodec(f(conn), opt), nil
}

type newClientFunc func(conn net.Conn, opt *conf.Option) (*Client, error)

func NewClientCodec(cc codec.Codec, opt *conf.Option) *Client {
	client := &Client{
		seq: 1, // seq starts with 1, and 0 means invalid call
		cc: cc,
		opt: opt,
		pending: make(map[uint64]*Call),
	}
	// wait to receive call result from server.
	go client.receive()
	return client
}

// receive receive every call result from server.
func (client *Client)receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		// This call have been done by server.
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// call was already removed or Write failed
			err = client.cc.ReadBody(nil)
		case len(h.Error) != 0:
			call.Error = errors.New(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			// set call.reply
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// error occurs, terminate those pending calls
	client.terminateCalls(err)
}

// send register call & send header,Args to server.
func (client *Client)send(call *Call) {
	// send can't concurrent
	client.sending.Lock()
	defer client.sending.Unlock()

	// register this call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// encode & send the request
	if err = client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		// call may be nil, it usually means that Write partially failed,
		// client has received the response and handled
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go invokes the function asynchronously
// returns the Call structure
func (client *Client)Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered!!")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args: args,
		Reply: reply,
		Done: done,
	}
	// register & send(head, body)
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete and return call errors.
// 1. client Call, send MethodMsg to Server
// 2. Server send result to Client (receive by "client server")
// 3. Call return, get call (reply)
// 4. ctx => client can control it
func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <- ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("rpc client: call failed " + ctx.Err().Error())
	case call := <- call.Done:
		return call.Error
	}
}


// Dial & parse opts

// parseOptions type, magicNumber.
func parseOptions(opts ...*conf.Option) (*conf.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return conf.DefaultOption, nil
	}
	opt := opts[0]
	opt.MagicNumber = conf.MagicNumber
	if opt.CodeType == "" {
		opt.CodeType = conf.DefaultOption.CodeType
	}
	return opt, nil
}

// Dial connects to an RPC server & return a client
// [==] Client's special server.
func Dial(network, address string, opts ...*conf.Option) (client *Client, err error) {
	return dialTimeout(NewClient, network, address, opts...)
}

// dialTimeout
// 1. Dial timeout
// 2. NewClient timeout
func dialTimeout(f newClientFunc, network, address string, opts ...*conf.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectionTimeout)
	if err != nil {
		log.Printf("client Dial error %s\n", err)
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{
			client: client,
			err: err,
		}
	}()
	// no timeout, wait for result
	if opt.ConnectionTimeout == 0 {
		result := <- ch
		return result.client, result.err
	}
	// has timeout
	// 1. timeout
	// 2. result
	select {
	case <-time.After(opt.ConnectionTimeout):
		return nil, fmt.Errorf("rpc client: connection timeout")
	case result := <-ch:
		return result.client, result.err
	}
}
