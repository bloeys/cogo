//go:generate go run inliner/main.go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/bloeys/cogo/cogo"
)

type CoroutineFunc[InT, OutT any] func(c *Coroutine[InT, OutT]) (out OutT)

type Coroutine[InT, OutT any] struct {
	State int32
	In    InT
	Func  CoroutineFunc[InT, OutT]
}

func (c *Coroutine[InT, OutT]) Tick() (out OutT, done bool) {

	if c.State == -1 {
		return out, true
	}

	out = c.Func(c)
	return out, c.State == -1
}

// func (c *Coroutine[InT, OutT]) Yield(out OutT) {
// }

func (c *Coroutine[InT, OutT]) Break() {
}

func Wow() {
	println("wow")
}

func test(c *Coroutine[int, int]) (out int) {

	cogo.Begin()

	println("Tick 1")
	cogo.Yield(1)

	println("Tick 2")
	cogo.Yield(2)

	println("Tick 3")
	cogo.Yield(3)

	println("Tick 4")
	cogo.Yield(4)

	cogo.End()

	// switch c.State {
	// case 0:
	// 	println("Tick 0")
	// 	c.State++
	// 	return 1, false
	// case 1:
	// 	println("Tick 1")
	// 	c.State++
	// 	return 2, false
	// case 2:
	// 	println("Tick 2")
	// 	c.State++
	// 	return 3, false
	// case 3:
	// 	println("Tick 3")
	// 	c.State++
	// 	return 4, false
	// default:
	// 	return out, true
	// }

	return out
}

// func test2() {

// 	// cogo.Begin()

// 	println("Hey")
// 	cogo.Yield()

// 	println("How you?")
// 	cogo.Yield()

// 	println("Bye")
// 	cogo.Yield()

// 	cogo.End()
// }

func main() {

	x := 1
switch_start:
	switch x {
	case 1:
		println(1)
		x = 3
		goto switch_start
	case 2:
		println(2)
	case 3:
		println(3)
	}
	return
	c := &Coroutine[int, int]{
		Func: test,
		In:   0,
	}

	for out, done := c.Tick(); !done; out, done = c.Tick() {
		println(out)
	}

	// test2()
	// test2()
	// test2()
	// test2()
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
