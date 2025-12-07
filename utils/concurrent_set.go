package utils

import (
	"fmt"
	"sync"
)

type ConcurrentSet[T comparable] struct {
	set map[T]struct{}
	mu  sync.RWMutex
}

func NewConcurrentSet[T comparable]() *ConcurrentSet[T] {
	return &ConcurrentSet[T]{
		set: make(map[T]struct{}),
	}
}

func WithCapacityConcurrentSet[T comparable](capacity int) *ConcurrentSet[T] {
	if capacity < 0 {
		panic("Cannot allocate with a negative capacity")
	}

	if capacity == 0 {
		return NewConcurrentSet[T]()
	}

	return &ConcurrentSet[T]{
		set: make(map[T]struct{}, capacity),
	}
}

func (s *ConcurrentSet[T]) Contains(k T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.set[k]
	return ok
}

func (s *ConcurrentSet[T]) Insert(k T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set[k] = struct{}{}
}

func (s *ConcurrentSet[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.set)
}

func (s *ConcurrentSet[T]) Keys() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]T, 0, len(s.set))

	for k := range s.set {
		keys = append(keys, k)
	}

	return keys
}

func (s *ConcurrentSet[T]) String() string {
	return fmt.Sprint(s.Keys())
}

func (s *ConcurrentSet[T]) Iter(continueFn func(k T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for k := range s.set {
		if !continueFn(k) {
			break
		}
	}
}

func (s *ConcurrentSet[T]) ToSlice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]T, 0, len(s.set))

	for k := range s.set {
		keys = append(keys, k)
	}

	return keys
}
