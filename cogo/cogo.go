package cogo

import "fmt"

type CoroutineFunc[InT, OutT any] func(c *Coroutine[InT, OutT]) (out OutT)

type Coroutine[InT, OutT any] struct {
	State    int32
	SubState int32
	In       InT
	Func     CoroutineFunc[InT, OutT]
}

func (c *Coroutine[InT, OutT]) Begin() {
}

func (c *Coroutine[InT, OutT]) Tick() (out OutT, done bool) {

	if c.State == -1 {
		return out, true
	}

	out = c.Func(c)
	return out, c.State == -1
}

func (c *Coroutine[InT, OutT]) Yield(out OutT) {
	panic(fmt.Sprintf("Yield got called at runtime, which means the code generator was not run, you used cogo incorrectly, or cogo has a bug. Yield should NOT get called at runtime. coroutine: %+v;;; yield value: %+v;;;", c, out))
}

func HasGen() bool {
	return true
}
