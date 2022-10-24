//go:generate go run inliner/main.go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/bloeys/cogo/cogo"
)

type Coroutine[T any] struct {
	State int32
	In    *T
}

func (c *Coroutine[T]) Run(f func(in *T)) {
	f(c.In)
}

var state = 0

func Wow() {
	println("wow")
}

// func test() {

// 	cogo.Begin()

// 	println("hi")
// 	println("this is from state_0")
// 	cogo.Yield()
// 	state = 1

// 	if 1 > 2 {
// 		println("gg")
// 	}

// 	println("Bye")
// 	println("this is from state_1")
// 	cogo.Yield()
// 	state = 2

// 	cogo.End()
// }

func test2() {

	cogo.Begin()

	println("Hey")
	cogo.Yield()

	println("How you?")
	cogo.Yield()

	println("Bye")
	cogo.Yield()

	cogo.End()
}

func main() {

	// test()
	// test()
	// test()

	test2()
	test2()
	test2()
	test2()

	println("Final state:", state)
}

func FileLine() int {
	_, _, lineNum, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get line number. Stack trace: " + string(debug.Stack()))
	}
	return lineNum
}

func FileLineString() string {
	_, _, lineNum, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get line number. Stack trace: " + string(debug.Stack()))
	}

	return fmt.Sprint(lineNum)
}
