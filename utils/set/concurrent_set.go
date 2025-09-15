package set

// Package set provides a generic implementation of a thread-safe concurrent set.
//
// A Set is a collection of unique elements, implemented using Go's built-in map type.
// The Set is parameterized with a type T, which must be comparable.
//
// This package offers several functions and methods to manipulate and work with sets,
// including the ability to iterate over the elements, map and filter them, and
// collect them back into a new Set. (It's been influened by Rust)

import (
	"fmt"
	"maps"
	"sync"
)

// A `ConcurrentSet` is implemented as a `map[T]struct{}` and an `RWMutex`.
//
// As with maps, a ConcurrentSet requires T to be a comparable, meaning it can
// accept structs if and only if they don't have a type
// like a slice/map/anything that is not comparable
//
// Examples:
//
//	package main
//
//	import (
//		"fmt"
//
//		"github.com/Jamlie/set/concurrentset"
//	)
//
//	type Person struct {
//		Id   int
//		Name string
//		Age  int
//	}
//
//	func main() {
//		intsSet := concurrentset.New[int]()
//		intsSet.Insert(1)
//		intsSet.Insert(2)
//		intsSet.Insert(3)
//		intsSet.Delete(1)
//
//		fmt.Println(intsSet.Len())
//		fmt.Println(intsSet)
//		if intsSet.Contains(2) {
//			fmt.Println("Set contains number 2")
//		}
//
//		uniquePeople := concurrentset.New[Person]()
//		uniquePeople.Insert(Person{Id: 21, Name: "John", Age:30})
//		uniquePeople.Insert(Person{Id: 22, Name: "Jane", Age:30})
//		uniquePeople.Insert(Person{Id: 23, Name: "Roland", Age:30})
//
//		newUnique := uniquePeople.Clone()
//
//		if !newUnique.Empty() {
//			newUnique.Clear()
//		}
//
//		uniquePeople.
//			Iter().
//			Map(func(k Person) Person {
//				return Person{
//					Id:   k.Id * 3,
//					Name: k.Name,
//					Age:  k.Age,
//				}
//			}).
//			Filter(func(k Person) bool {
//				return k.Id%2 == 1
//			}).
//			Collect()
//		fmt.Println(uniquePeople)
//	}
type ConcurrentSet[T comparable] struct {
	set map[T]struct{}
	mu  sync.RWMutex
}

func New[T comparable]() *ConcurrentSet[T] {
	return &ConcurrentSet[T]{
		set: make(map[T]struct{}),
	}
}

func WithCapacity[T comparable](capacity int) *ConcurrentSet[T] {
	if capacity < 0 {
		panic("Cannot allocate with a negative capacity")
	}

	if capacity == 0 {
		return New[T]()
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
