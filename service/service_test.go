package service

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int
type Args struct{Num1, Num2 int}

// rpc call function
func (f Foo)Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// it's not a exported method
func (f Foo)sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: " + msg, v...))
	}
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(len(s.method) == 1, "wrong service Method, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong Method, Sum shouldn't nil")
}

func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]

	// "占位" & init argv
	argv := mType.newArgv()
	replyv := mType.newReply()
	argv.Set(reflect.ValueOf(Args{Num2: 3, Num1: 2}))

	err := s.call(mType, argv, replyv)
	_assert(err == nil && *replyv.Interface().(*int) == 5 && mType.NumCalls() == 1, "failed to call Foo.Sum")
}
