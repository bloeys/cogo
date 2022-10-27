//go:generate go run inliner/main.go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/bloeys/cogo/cogo"
)

func test(c *cogo.Coroutine[int, int]) (out int) {
	if cogo.HasGen() {
		return test_cogo(c)
	}

	c.Begin()

	println("\nTick 1")
	c.Yield(1)

	if c.In > 1 {
		println("\nTick 1.5")
		c.Yield(c.In)
	}

	println("\nTick 2")
	c.Yield(2)

	println("\nTick 3")
	c.Yield(3)

	println("\nTick 4")
	c.Yield(4)

	println("\nTick before end")

	return out
}

func main() {

	c := &cogo.Coroutine[int, int]{
		Func: test,
		In:   0,
	}

	c.In = 5
	for out, done := c.Tick(); !done; out, done = c.Tick() {
		println(out)
	}

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
