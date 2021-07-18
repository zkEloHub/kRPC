package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// net/rpc
// method can be called:
// 1. method's type is exported
// 2. method is exported
// 3. two arguments, both exported
// 4. the second argument must be pointer
// 5. return type is error.
// a type's methods
type methodType struct {
	// method self
	method    reflect.Method
	// first argument
	ArgType   reflect.Type
	// second argument
	ReplyType reflect.Type
	// the rpc method's call number
	numCalls  uint64
}

func (m *methodType)NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}


// newArgv, newReply: for making rpc_call's instance

func (m *methodType)newArgv() reflect.Value {
	var argv reflect.Value
	// arg may be a pointer type, or a value type
	// Elem(): Only for Array, Chan, Map, Ptr, or Slice.
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}

	return argv
}

// prev: replyType must be a pointer type
func (m *methodType)newReply() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())

	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}

	return replyv
}


// service (var **foo** Foo)
type service struct {
	// service's name.
	name   string

	// TypeOf, ValueOf
	typ    reflect.Type
	rcvr   reflect.Value

	// key: name, value: argv, reply...
	method map[string]*methodType
}

// newService init service and parse the method msg.
func newService(rcvr interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.typ = reflect.TypeOf(rcvr)
	// Indirect => value, Type().Name() => value's name
	s.name = reflect.Indirect(s.rcvr).Type().Name()

	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

// registerMethods scrape valid method.
// valid: exported or buildIn argument, return an error type.
func (s *service)registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		// msg about the 'method'
		mType := method.Type
		// **Must**: func(arg, *reply)
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method: method,
			ArgType: argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s \n", s.name, method.Name)
	}
}

func (s *service)call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	// arguments: itemSelf(foo), argv, replyv
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

// exported or builtin
func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}