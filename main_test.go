package kRPC

import (
	"log"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func reflectTest() {
	var wg sync.WaitGroup
	// 通过反射，获取某个结构体的所有方法
	typ := reflect.TypeOf(&wg)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		// get method's argv & return
		argv := make([]string, 0, method.Type.NumIn())
		returns := make([]string, 0, method.Type.NumOut())

		// first argv is wg, so j from 1
		for j := 1; j < cap(argv); j++ {
			argv = append(argv, method.Type.In(j).Name())
		}
		for j := 0; j < cap(returns); j++ {
			returns = append(returns, method.Type.Out(j).Name())
		}
		log.Printf("func (w *%s) %s(%s) %s\n",
			typ.Elem().Name(),
			method.Name,
			strings.Join(argv, ","),
			strings.Join(returns, ","))
	}
}

func TestMy(t *testing.T) {
	reflectTest()
}