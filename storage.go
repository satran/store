package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	Dir string

	store map[string][]byte
	sync.Mutex
}

func NewStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(os.Args[1])
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	return &Store{
		Dir:   abs,
		store: make(map[string][]byte),
	}, nil
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.Lock()
	v, ok := s.store[key]
	s.Unlock()
	return v, ok
}

func (s *Store) Set(key string, value []byte) {
	s.Lock()
	s.store[key] = value
	s.Unlock()
}

func (s *Store) SetStore(value map[string][]byte) {
	s.Lock()
	s.store = value
	s.Unlock()
}
