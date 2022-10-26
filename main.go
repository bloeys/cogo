//go:generate go run inliner/main.go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/bloeys/cogo/cogo"
)

func Wow() {
	println("wow")
}

func test(c *cogo.Coroutine[int, int]) (out int) {

	c.Begin()

	println("Tick 1")
	c.Yield(1)

	println("Tick 2")
	c.Yield(2)

	println("Tick 3")
	c.Yield(3)

	println("Tick 4")
	c.Yield(4)

	println("Tick before end")
	c.End()

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

	c := &cogo.Coroutine[int, int]{
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
