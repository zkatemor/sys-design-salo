package orders

import (
	"errors"
	"math/rand/v2"
	"strconv"
	"sync"
)

type Item struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type Order struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Total  int    `json:"total"`
	Items  []Item `json:"items"`
}

var ErrNotFound = errors.New("order not found")

type Store struct {
	mu     sync.RWMutex
	orders map[string]Order
}

func NewStore() *Store {
	return &Store{orders: make(map[string]Order)}
}

func (s *Store) Create(userID string, items []Item) Order {
	total := 0
	for _, it := range items {
		total += it.Qty * 100
	}
	o := Order{
		ID:     strconv.FormatInt(rand.Int64(), 36),
		UserID: userID,
		Total:  total,
		Items:  items,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
	return o
}

func (s *Store) Get(id string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	return o, nil
}
