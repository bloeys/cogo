package cogo

import "fmt"

type CoroutineFunc[InT, OutT any] func(c *Coroutine[InT, OutT])

var _ Yielder = &Coroutine[int, int]{}

type Coroutine[InT, OutT any] struct {
	State    int32
	SubState int32
	In       InT
	Out      OutT
	Func     CoroutineFunc[InT, OutT]
	Yielder  Yielder
}

func (c *Coroutine[InT, OutT]) Begin() {
}

func (c *Coroutine[InT, OutT]) Tick() (done bool) {

	if c.State == -1 {
		return true
	}

	if c.Yielder != nil {
		if !c.Yielder.Tick() {
			return false
		}

		c.Yielder = nil
	}

	oldYielder := c.Yielder
	c.Func(c)

	// On YieldTo() we want to always tick once before returning, so here we check do that.
	// Also, if the yielder was done after one tick we nil it
	if c.Yielder != oldYielder {
		if c.Yielder.Tick() {
			c.Yielder = nil
		} else {
			return false
		}
	}

	return c.State == -1
}

// Yield yields and sets the Out variable to the passed variable
func (c *Coroutine[InT, OutT]) Yield(out OutT) {
	panic(fmt.Sprintf("Yield got called at runtime, which means the code generator was not run, you used cogo incorrectly, or cogo has a bug. Yield should NOT get called at runtime. coroutine: %+v;;; yield value: %+v;;;", c, out))
}

// YieldTo gives control to a Yielder object and immediately executes one Tick on it.
// Future 'Tick' calls on the original coroutine will run 'Tick' on passed Yielder.
//
// The original coroutine will only resume execution once this yielder reports that its done
func (c *Coroutine[InT, OutT]) YieldTo(y Yielder) {
	panic(fmt.Sprintf("YieldTo got called at runtime, which means the code generator was not run, you used cogo incorrectly, or cogo has a bug. Yield should NOT get called at runtime. coroutine: %+v;;; yielder value: %+v;;;", c, y))
}

// YieldNone yields without updating the Out variable
func (c *Coroutine[InT, OutT]) YieldNone() {
	panic(fmt.Sprintf("YieldNone got called at runtime, which means the code generator was not run, you used cogo incorrectly, or cogo has a bug. Yield should NOT get called at runtime. coroutine: %+v;;;", c))
}

func HasGen() bool {
	return true
}

func New[InT, OutT any](coro CoroutineFunc[InT, OutT], input InT) (c *Coroutine[InT, OutT]) {
	return &Coroutine[InT, OutT]{
		Func: coro,
		In:   input,
	}
}
