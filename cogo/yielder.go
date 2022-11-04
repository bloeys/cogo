package cogo

type Yielder interface {
	Tick() (done bool)
}
