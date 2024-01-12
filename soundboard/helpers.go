package soundboard

import (
	"context"
	"errors"
)

var (
	ErrCancelled = errors.New("context was cancelled before event triggered")
)

func toPtr[T any](x T) *T {
	return &x
}

func await(ctx context.Context, event chan error) error {
	select {
	case <-ctx.Done():
		return ErrCancelled
	case err := <-event:
		return err
	}
}

func queryAll[S []P, P *T, T any](fn func(last P) (S, error)) (S, error) {
	var all S
	var last T
	for {
		next, err := fn(&last)
		if err != nil {
			return nil, err
		}
		if l := len(next); l > 0 {
			all = append(all, next...)
			last = *next[l-1]
		} else {
			break
		}
	}
	return all, nil
}
