//go:generate cogo
package main

import (
	"time"

	"github.com/bloeys/cogo/cogo"
)

func runDemo() {

	c := cogo.New(test, 0)

	ticks := 1
	start := time.Now()
	for done := c.Tick(); !done; done = c.Tick() {
		println("Ticks done:", ticks, "; Output:", c.Out, "\n")
		ticks++
		time.Sleep(1 * time.Millisecond)
	}

	println("Time taken:", time.Since(start).String())
}

func test(c *cogo.Coroutine[int, int]) {
	if cogo.HasGen() {
		test_cogo(c)
		return
	}

	c.Begin()

	println("test yield:", 1)
	c.Yield(1)

	c.YieldTo(cogo.NewSleeper(100 * time.Millisecond))

	c.YieldTo(cogo.New(test2, 0))

	println("test yield:", 2)
	c.Yield(2)
}

func test2(c *cogo.Coroutine[int, int]) {
	if cogo.HasGen() {
		test2_cogo(c)
		return
	}

	c.Begin()

	println("test2222 yield:", 1)
	c.Yield(1)

	println("test2222 yield:", 2)
	c.Yield(2)

	println("test2222 before yield none")
	c.YieldNone()
	println("test2222 after yield none")
}
