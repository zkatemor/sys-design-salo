package users

import (
	"errors"
	"math/rand/v2"
	"strconv"
	"sync"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var ErrNotFound = errors.New("user not found")

type Store struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewStore() *Store {
	return &Store{users: make(map[string]User)}
}

func (s *Store) Create(name, email string) User {
	u := User{
		ID:    strconv.FormatInt(rand.Int64(), 36),
		Name:  name,
		Email: email,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
	return u
}

func (s *Store) Get(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *Store) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}
