package store

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("obssvc/store")

type Order struct {
	ID     string `json:"id"`
	User   string `json:"user"`
	Amount int    `json:"amount"`
}

var ErrNotFound = errors.New("order not found")

type Store struct {
	mu     sync.RWMutex
	orders map[string]Order
}

func New() *Store {
	return &Store{orders: make(map[string]Order)}
}

func (s *Store) Save(ctx context.Context, o Order) error {
	ctx, span := tracer.Start(ctx, "store.Save")
	defer span.End()
	span.SetAttributes(
		attribute.String("order.id", o.ID),
		attribute.String("order.user", o.User),
	)

	if err := simulate(ctx); err != nil {
		span.RecordError(err)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (Order, error) {
	ctx, span := tracer.Start(ctx, "store.Get")
	defer span.End()
	span.SetAttributes(attribute.String("order.id", id))

	if err := simulate(ctx); err != nil {
		span.RecordError(err)
		return Order{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	return o, nil
}

func simulate(ctx context.Context) error {
	delay := time.Duration(50+rand.IntN(150)) * time.Millisecond
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	if rand.IntN(20) == 0 {
		return errors.New("transient db error")
	}
	return nil
}
