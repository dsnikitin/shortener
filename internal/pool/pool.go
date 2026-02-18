package pool

import (
	"fmt"
	"reflect"
	"sync"
)

type Resetter interface {
	Reset()
}

type Pool[T Resetter] struct {
	mu      sync.Mutex
	objects []T
	maxSize int
	factory func() T
}

func New[T Resetter](size int, factory func() T) (*Pool[T], error) {
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	if size < 1 {
		return nil, fmt.Errorf("size should be greater than 0")
	}

	return &Pool[T]{
		objects: make([]T, 0, size),
		maxSize: size,
		factory: factory,
	}, nil
}

func (p *Pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()

	objectsCount := len(p.objects)

	if objectsCount == 0 {
		return p.factory()
	}

	obj := p.objects[objectsCount-1]
	p.objects = p.objects[:objectsCount-1]
	return obj
}

// Put возвращает объект обратно в пул.
func (p *Pool[T]) Put(obj T) {
	if reflect.ValueOf(obj).IsNil() {
		return
	}

	obj.Reset()

	p.mu.Lock()
	if len(p.objects) < p.maxSize {
		p.objects = append(p.objects, obj)
	}
	p.mu.Unlock()
}
