// Package breaker — circuit breaker.
//
// Состояния и переходы:
//
//   Closed   --FailureThreshold подряд ошибок--> Open
//   Open     --OpenTimeout прошло-------------->  HalfOpen
//   HalfOpen --SuccessThreshold подряд успехов-> Closed
//   HalfOpen --любой провал--------------------> Open
package breaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	ErrOpen          = errors.New("circuit breaker is open")
	ErrTooManyProbes = errors.New("circuit breaker probe limit reached")
)

type Config struct {
	FailureThreshold  int
	OpenTimeout       time.Duration
	HalfOpenMaxProbes int
	SuccessThreshold  int

	// Now — источник «текущего времени» для тестов; nil ⇒ time.Now.
	Now func() time.Time
}

type Breaker struct {
	cfg Config
	mu  sync.Mutex

	state State

	consecutiveFailures  int
	consecutiveSuccesses int
	openedAt             time.Time
	inFlightProbes       int
}

func New(cfg Config) (*Breaker, error) {
	if cfg.FailureThreshold <= 0 {
		return nil, fmt.Errorf("FailureThreshold must be > 0, got %d", cfg.FailureThreshold)
	}
	if cfg.OpenTimeout <= 0 {
		return nil, fmt.Errorf("OpenTimeout must be > 0, got %v", cfg.OpenTimeout)
	}
	if cfg.HalfOpenMaxProbes <= 0 {
		return nil, fmt.Errorf("HalfOpenMaxProbes must be > 0, got %d", cfg.HalfOpenMaxProbes)
	}
	if cfg.SuccessThreshold <= 0 {
		return nil, fmt.Errorf("SuccessThreshold must be > 0, got %d", cfg.SuccessThreshold)
	}
	return &Breaker{cfg: cfg, state: StateClosed}, nil
}

func (b *Breaker) now() time.Time {
	if b.cfg.Now != nil {
		return b.cfg.Now()
	}
	return time.Now()
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maybeAdvanceFromOpen()
}

func (b *Breaker) maybeAdvanceFromOpen() State {
	if b.state == StateOpen && b.now().Sub(b.openedAt) >= b.cfg.OpenTimeout {
		b.state = StateHalfOpen
		b.consecutiveSuccesses = 0
	}
	return b.state
}

func (b *Breaker) transitionToOpen() {
	b.state = StateOpen
	b.openedAt = b.now()
	b.consecutiveSuccesses = 0
}

// Execute вызывает fn под защитой breaker-а.
// Возвращает ErrOpen / ErrTooManyProbes без вызова fn,
// иначе — результат fn().
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()
	state := b.maybeAdvanceFromOpen()

	switch state {
	case StateOpen:
		b.mu.Unlock()
		return ErrOpen

	case StateHalfOpen:
		if b.inFlightProbes >= b.cfg.HalfOpenMaxProbes {
			b.mu.Unlock()
			return ErrTooManyProbes
		}
		b.inFlightProbes++
		b.mu.Unlock()

		err := fn()

		b.mu.Lock()
		b.inFlightProbes--
		if err != nil {
			b.transitionToOpen()
		} else {
			b.consecutiveSuccesses++
			if b.consecutiveSuccesses >= b.cfg.SuccessThreshold {
				b.state = StateClosed
				b.consecutiveFailures = 0
				b.consecutiveSuccesses = 0
			}
		}
		b.mu.Unlock()
		return err

	default: // StateClosed
		b.mu.Unlock()
		err := fn()

		b.mu.Lock()
		if err != nil {
			b.consecutiveFailures++
			if b.consecutiveFailures >= b.cfg.FailureThreshold {
				b.transitionToOpen()
			}
		} else {
			b.consecutiveFailures = 0
		}
		b.mu.Unlock()
		return err
	}
}
