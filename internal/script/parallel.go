package script

import (
	"fmt"
	"runtime/debug"
)

const maxParallel = 8

type taskOutcome[R any] struct {
	index  int
	result R
	err    error
}

func runParallel[T, R any](items []T, work func(T) (R, error)) (results []R, firstErr error) {
	results = make([]R, len(items))
	outcomes := make(chan taskOutcome[R])
	started, running := 0, 0
	for {
		canStartNext := firstErr == nil && started < len(items) && running < maxParallel
		if canStartNext {
			go func(index int) {
				result, err := callRecoveringPanic(work, items[index])
				outcomes <- taskOutcome[R]{index, result, err}
			}(started)
			started, running = started+1, running+1
			continue
		}
		if running == 0 {
			return results, firstErr
		}
		outcome := <-outcomes
		running--
		results[outcome.index] = outcome.result
		if firstErr == nil {
			firstErr = outcome.err
		}
	}
}

func callRecoveringPanic[T, R any](work func(T) (R, error), item T) (result R, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v\n%s", recovered, debug.Stack())
		}
	}()
	return work(item)
}
