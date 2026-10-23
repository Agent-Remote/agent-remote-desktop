package app

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
)

// A query owns its worker until it is collected, including after cancellation.
// Generations keep observations invalidated by a configuration change from
// becoming visible when an older worker finishes.
type networkQuery[T any] struct {
	report     *T
	err        error
	busy       bool
	stale      bool
	generation uint64
	cancel     context.CancelFunc
	done       chan struct{}
	results    chan networkQueryResult[T]
}

type networkQueryResult[T any] struct {
	report     T
	err        error
	generation uint64
}

func (query *networkQuery[T]) start(work func(context.Context) (T, error)) {
	ctx, cancel := context.WithCancel(context.Background())
	query.cancel, query.busy = cancel, true
	query.report, query.err, query.stale = nil, nil, false
	query.done, query.results = make(chan struct{}), make(chan networkQueryResult[T], 1)
	done, results, generation := query.done, query.results, query.generation
	go func() {
		defer close(done)
		defer cancel()
		report, err := work(ctx)
		results <- networkQueryResult[T]{report: report, err: err, generation: generation}
	}()
}

func (query *networkQuery[T]) poll(c *ui.Context) {
	if !query.busy {
		return
	}
	select {
	case result := <-query.results:
		query.busy = false
		if result.generation != query.generation {
			return
		}
		query.err = result.err
		if result.err == nil {
			query.report = &result.report
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (query *networkQuery[T]) stop() {
	if query.busy {
		query.generation++
		query.cancel()
		query.report, query.err = nil, context.Canceled
	}
}

func (query *networkQuery[T]) invalidate() {
	query.stale = query.stale || query.busy || query.report != nil || query.err != nil
	query.generation++
	query.report, query.err = nil, nil
	if query.busy {
		query.cancel()
	}
}

func (query *networkQuery[T]) close() {
	if query.cancel != nil {
		query.cancel()
		<-query.done
	}
}
