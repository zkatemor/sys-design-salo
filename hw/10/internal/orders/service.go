package orders

import (
	"context"
	"math/rand/v2"
	"strconv"

	"obssvc/internal/observability"
	"obssvc/internal/store"

	"go.opentelemetry.io/otel/attribute"
)

type Service struct {
	db  *store.Store
	tel *observability.Telemetry
}

func NewService(db *store.Store, tel *observability.Telemetry) *Service {
	return &Service{db: db, tel: tel}
}

func (s *Service) Create(ctx context.Context, user string, amount int) (store.Order, error) {
	ctx, span := s.tel.Tracer.Start(ctx, "orders.Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("order.user", user),
		attribute.Int("order.amount", amount),
	)

	o := store.Order{
		ID:     strconv.FormatInt(rand.Int64(), 36),
		User:   user,
		Amount: amount,
	}
	if err := s.db.Save(ctx, o); err != nil {
		s.tel.OrdersTotal.WithLabelValues("error").Inc()
		span.RecordError(err)
		return store.Order{}, err
	}
	s.tel.OrdersTotal.WithLabelValues("ok").Inc()
	return o, nil
}

func (s *Service) Get(ctx context.Context, id string) (store.Order, error) {
	ctx, span := s.tel.Tracer.Start(ctx, "orders.Get")
	defer span.End()
	span.SetAttributes(attribute.String("order.id", id))
	return s.db.Get(ctx, id)
}
