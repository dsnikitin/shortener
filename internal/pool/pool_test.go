package pool

import (
	"sync"
	"testing"
)

type testObject struct {
	value int
	reset bool
}

func (t *testObject) Reset() {
	t.reset = true
	t.value = 0
}

// TestNew проверяет создание пула
func TestNew(t *testing.T) {
	pool, _ := New(5, func() *testObject {
		return &testObject{value: 42}
	})

	if pool.maxSize != 5 {
		t.Errorf("Expected maxSize 5, got %d", pool.maxSize)
	}
	if cap(pool.objects) != 5 {
		t.Errorf("Expected capacity 5, got %d", cap(pool.objects))
	}
	if pool.factory == nil {
		t.Error("factory should not be nil")
	}
}

// TestGet проверяет получение объектов из пула
func TestGet(t *testing.T) {
	pool, _ := New(2, func() *testObject {
		return &testObject{value: 42}
	})

	// Первый Get - пул пуст, создаем новый объект
	obj1 := pool.Get()
	if obj1.value != 42 {
		t.Errorf("Expected value 42, got %d", obj1.value)
	}

	// Возвращаем объект в пул
	pool.Put(obj1)

	// Второй Get - получаем существующий объект
	obj2 := pool.Get()
	if obj2 != obj1 {
		t.Error("Get() should return existing object from pool")
	}
}

// TestGetFromEmptyPool проверяет получение из пустого пула
func TestGetFromEmptyPool(t *testing.T) {
	objNumber := 0
	pool, _ := New(2, func() *testObject {
		objNumber++
		return &testObject{value: objNumber}
	})

	// Все Get должны создавать новые объекты
	obj1 := pool.Get()
	obj2 := pool.Get()
	obj3 := pool.Get()

	if objNumber != 3 {
		t.Errorf("Factory should be called 3 times, got %d", objNumber)
	}
	if obj1.value != 1 || obj2.value != 2 || obj3.value != 3 {
		t.Error("Objects should have sequential values from factory")
	}
}

// TestPut проверяет возврат объектов в пул
func TestPut(t *testing.T) {
	pool, _ := New(2, func() *testObject {
		return &testObject{}
	})

	obj1 := &testObject{value: 1, reset: false}
	obj2 := &testObject{value: 2, reset: false}
	obj3 := &testObject{value: 3, reset: false}

	// Возвращаем объекты
	pool.Put(obj1)
	pool.Put(obj2)

	// Проверяем, что Reset был вызван
	if !obj1.reset {
		t.Error("Put() should call Reset() on obj1")
	}
	if !obj2.reset {
		t.Error("Put() should call Reset() on obj2")
	}
	if obj1.value != 0 || obj2.value != 0 {
		t.Error("Reset() should reset object fields")
	}

	// Проверяем состояние пула
	pool.mu.Lock()
	if len(pool.objects) != 2 {
		t.Errorf("Pool should have 2 objects, got %d", len(pool.objects))
	}
	pool.mu.Unlock()

	// Пытаемся добавить третий объект (пул полный)
	pool.Put(obj3)

	// Reset все равно должен быть вызван
	if !obj3.reset {
		t.Error("Put() should call Reset() even if pool is full")
	}

	// Проверяем, что пул не увеличился
	pool.mu.Lock()
	if len(pool.objects) != 2 {
		t.Errorf("Pool size should remain 2, got %d", len(pool.objects))
	}
	pool.mu.Unlock()
}

// TestPutNil проверяет возврат nil объекта
func TestPutNil(t *testing.T) {
	pool, _ := New(2, func() *testObject {
		return &testObject{}
	})

	// Put с nil не должен вызывать панику
	pool.Put(nil)

	// Проверяем, что состояние пула не изменилось
	pool.mu.Lock()
	if len(pool.objects) != 0 {
		t.Errorf("Pool should be empty, got %d objects", len(pool.objects))
	}
	pool.mu.Unlock()
}

// TestConcurrent проверяет конкурентный доступ
func TestConcurrent(t *testing.T) {
	pool, _ := New(10, func() *testObject {
		return &testObject{}
	})

	var wg sync.WaitGroup
	workers := 50
	iterations := 100

	for i := range workers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				obj := pool.Get()
				obj.value = id * j
				pool.Put(obj)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что размер пула не превышен
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(pool.objects) > 10 {
		t.Errorf("Pool exceeded max size: %d", len(pool.objects))
	}
}

// TestGetAfterPut проверяет получение после возврата
func TestGetAfterPut(t *testing.T) {
	pool, _ := New(3, func() *testObject {
		return &testObject{value: 100}
	})

	// Создаем несколько объектов
	objs := make([]*testObject, 3)
	for i := range 3 {
		objs[i] = pool.Get()
	}

	// Возвращаем их в обратном порядке (LIFO)
	for i := 2; i >= 0; i-- {
		pool.Put(objs[i])
	}

	// Должны получить объекты в обратном порядке (LIFO)
	for i := range 3 {
		obj := pool.Get()
		if obj != objs[i] {
			t.Error("Pool should work as LIFO stack")
		}
	}
}

// TestPoolBoundaries проверяет граничные случаи
func TestPoolBoundaries(t *testing.T) {
	pool, _ := New(1, func() *testObject {
		return &testObject{}
	})

	obj1 := &testObject{value: 1}
	obj2 := &testObject{value: 2}

	pool.Put(obj1)
	pool.Put(obj2) // Этот не должен добавиться (пул полный)

	// Должны получить obj1, а не obj2
	got := pool.Get()
	if got != obj1 {
		t.Error("Pool should return the only object it contains")
	}

	// После получения obj1, пул пуст
	got = pool.Get()
	if got == obj1 || got == obj2 {
		t.Error("Pool should create new object when empty")
	}
}

// TestFactory проверяет что factory вызывается только при необходимости
func TestFactory(t *testing.T) {
	factoryCallCount := 0
	pool, _ := New(2, func() *testObject {
		factoryCallCount++
		return &testObject{value: factoryCallCount}
	})

	// Первый Get - factory вызывается
	obj1 := pool.Get()
	if factoryCallCount != 1 {
		t.Errorf("Factory should be called once, got %d", factoryCallCount)
	}

	// Возвращаем объект
	pool.Put(obj1)

	// Второй Get - factory НЕ вызывается (берем из пула)
	obj2 := pool.Get()
	if factoryCallCount != 1 {
		t.Errorf("Factory should not be called when pool has objects, got %d calls", factoryCallCount)
	}
	if obj2 != obj1 {
		t.Error("Should get existing object from pool")
	}
}
