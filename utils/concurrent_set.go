package utils

import (
	"fmt"
	"maps"
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

func (s *ConcurrentSet[T]) Insert(k T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set[k] = struct{}{}
}

func (s *ConcurrentSet[T]) Delete(k T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.set, k)
}

func (s *ConcurrentSet[T]) Len() int {
	return len(s.set)
}

func (s *ConcurrentSet[T]) Contains(k T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.set[k]
	return ok
}

func (s *ConcurrentSet[T]) Clone() *ConcurrentSet[T] {
	return &ConcurrentSet[T]{
		set: maps.Clone(s.set),
	}
}

func (s *ConcurrentSet[T]) Keys() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]T, 0, s.Len())

	for k := range s.set {
		keys = append(keys, k)
	}

	return keys
}

func (s *ConcurrentSet[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.set)
}

func (s *ConcurrentSet[T]) Empty() bool {
	return s.Len() == 0
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

	keys := make([]T, 0, s.Len())

	for k := range s.set {
		keys = append(keys, k)
	}

	return keys
}
