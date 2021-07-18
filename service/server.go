package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"krpc/codec"
	"krpc/conf"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Connection's message between client & server
// | Option{..., CodeType: xxx}      | Header..., Body ...                      | Header2, Body2, ...
// |<------    Json Encode   ------> | <------   Encode With CodeType   ------> |

// Server represents an RPC server
type Server struct {
	serviceMap sync.Map // service Name, service
	opt        *conf.Option
}

func NewServer() *Server {
	return &Server{}
}

// DefaultServer the instance of *Server
var DefaultServer = NewServer()


// Register publishes in the server
// init service's methodTypes...
func (s *Server) Register(rcvr interface{}) error {
	svc := newService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(svc.name, svc); dup {
		return errors.New("rpc: service already defined: " + svc.name)
	}
	return nil
}

// findService find service from serviceMap
// argument: service.method
// 1. check service in serviceMap
// 2. check method in service.method
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-format: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	// Load: return interface{}
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

// Accept , lis: {Accept, Close, Addr}
func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("rpc server: accept error: %s\n", err)
			return
		}
		go s.ServeConn(conn)
	}
}

// ServeConn one goroutine per connection
// net.Conn {read, write, close...}
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	// decode a Option instance
	defer func() { _ = conn.Close() }()
	var opt conf.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Printf("rpc server: decode option error: %s\n", err)
		return
	}
	if opt.MagicNumber != conf.MagicNumber {
		log.Printf("rpc server: MagicNumber dismatch, your magic: %v\n", opt.MagicNumber)
		return
	}
	newCodecF := codec.NewCodecFuncMap[opt.CodeType]
	if newCodecF == nil {
		log.Printf("rpc server: code type not found%s\n", opt.CodeType)
		return
	}
	s.opt = &opt
	s.serveCodec(newCodecF(conn))
}

// a placeholder in response when error occurs.
var invalidRequest = struct{}{}

// serveCodec get request & serve (decode every request...)
func (s *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break // can't recover, close the connection
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg, s.opt.HandleTimeout)
	}
	wg.Wait()
}

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
	mtype        *methodType
	svc          *service
}

// readRequest read request from client.
// check service.method...
func (s *Server) readRequest(c codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(c)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	// check if the server has the service.method
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}

	// "占位" & init argv
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReply()

	// make sure that argvi is a pointer, ReadBody need a pointer as parameter.
	// argv: instance or ptr... need check it!
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		// get pointer.
		argvi = req.argv.Addr().Interface()
	}
	// ReadBody(&item)
	if err = c.ReadBody(argvi); err != nil {
		log.Printf("rpc server: read argv err: %s\n", err)
	}
	return req, nil
}

// readRequestHeader read & decode header with codec
func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Printf("rpc server: read header error: %s\n", err)
		}
		return nil, err
	}
	return &h, nil
}

// handleRequest handle client's request if no error.
// todo: send chan ?
func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	callCh := make(chan struct{})
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		callCh <- struct{}{}
		// call it...
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			return
		}
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
	}()
	if timeout == 0 {
		<- callCh
		return
	}
	select {
	case <- time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout ")
		s.sendResponse(cc, req.h, invalidRequest, sending)
	case <- callCh:
		return
	}
}

// sendResponse need mutex
func (s *Server) sendResponse(c codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := c.Write(h, body); err != nil {
		log.Printf("rpc server: write response err: %s\n", err)
	}
}

// Accept Usual accept method
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }
