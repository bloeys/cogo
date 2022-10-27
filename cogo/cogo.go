package cogo

type CoroutineFunc[InT, OutT any] func(c *Coroutine[InT, OutT]) (out OutT)

type Coroutine[InT, OutT any] struct {
	State int32
	In    InT
	Func  CoroutineFunc[InT, OutT]
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
}

func HasGen() bool {
	return true
}
