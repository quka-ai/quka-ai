package register

import "sync"

type funcRegister struct {
	handlers map[any][]any
	locker   sync.Mutex
}

var fr *funcRegister

func init() {
	fr = &funcRegister{
		handlers: make(map[any][]any),
	}
}

type Handler[T any] func(T)

func RegisterFunc[T any](key any, handler Handler[T]) {
	fr.locker.Lock()
	fr.handlers[key] = append(fr.handlers[key], handler)
	fr.locker.Unlock()
}

func ResolveFuncHandlers[T any](key any) []Handler[T] {
	fr.locker.Lock()
	defer fr.locker.Unlock()

	var result []Handler[T]
	for _, v := range fr.handlers[key] {
		h, o := v.(Handler[T])
		if o {
			result = append(result, h)
		}
	}
	return result
}
