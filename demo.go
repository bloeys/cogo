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

	println("test yield:", 1)
	c.Yield(1)

	if c.Out > 2 {
		c.Yield(1)
	}

	// Yield here until at least 100ms passed
	c.YieldTo(cogo.NewSleeper(100 * time.Millisecond))

	// Yield here until the coroutine 'test2' has finished
	// c.YieldTo(cogo.New(test2, 0))

	println("test yield:", 2)
	c.Yield(2)
}

// func test2(c *cogo.Coroutine[int, int]) {

// 	println("test2222 yield:", 1)
// 	c.Yield(1)

// 	println("test2222 yield:", 2)
// 	c.Yield(2)

// 	println("test2222 before yield none")
// 	c.YieldNone()
// 	println("test2222 after yield none")
// }

func NewApproach(state int) {

	switch state {
	case 1:
		goto lbl_1
	case 2:
		goto lbl_2
	case 3:
		state = 1
		goto lbl_2
	default:
	}

	println("1")
	println("2")
	state = 1
	// return

lbl_1:
	println("3")
	state = 2
	// return

lbl_2:
	{
		switch state {
		case 1:
			goto lbl_3

		default:
		}

		println("4")
		state = 3
		// return

	lbl_3:
	}
}
