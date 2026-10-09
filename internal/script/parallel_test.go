package script

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunParallelGivesWhatEachItemGaveInTheOrderOfTheItems(t *testing.T) {
	if gave, err := runParallel(nil, func(int) (int, error) { t.Error("work was started without an item"); return 0, nil }); err != nil || len(gave) != 0 {
		t.Errorf("of no items: %v, %v", gave, err)
	}
	const count = 30
	items := make([]int, count)
	ended := make([]chan struct{}, count)
	for i := range items {
		items[i], ended[i] = i, make(chan struct{})
	}
	var guard sync.Mutex
	var order []int
	gave, err := runParallel(items, func(item int) (string, error) {
		if (item+1)%maxParallel != 0 && item != count-1 {
			select {
			case <-ended[item+1]:
			case <-time.After(5 * time.Second):
			}
		}
		guard.Lock()
		order = append(order, item)
		guard.Unlock()
		close(ended[item])
		return fmt.Sprint("of ", item), nil
	})
	if err != nil || len(gave) != count || slices.IsSorted(order) {
		t.Fatalf("eachOf = %q, %v; the work ended in the order %v, which must not be that of the items", gave, err, order)
	}
	for i, result := range gave {
		if result != fmt.Sprint("of ", i) {
			t.Errorf("item %d gave %q", i, result)
		}
	}
}

func TestRunParallelStartsNoWorkAfterAnErrorAndReturnsItWhenTheRunningWorkHasEnded(t *testing.T) {
	first, later := errors.New("the first failure"), errors.New("a later failure")
	synctest.Test(t, func(t *testing.T) {
		firstMayEnd, othersMayEnd, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var guard sync.Mutex
		started, running := 0, 0
		counted := func() (int, int) {
			guard.Lock()
			defer guard.Unlock()
			return started, running
		}
		var err error
		go func() {
			defer close(returned)
			_, err = runParallel(make([]int, 40), func(int) (int, error) {
				guard.Lock()
				started++
				running++
				mine := started
				guard.Unlock()
				defer func() {
					guard.Lock()
					running--
					guard.Unlock()
				}()
				if mine == 1 {
					<-firstMayEnd
					return 0, first
				}
				<-othersMayEnd
				return 0, later
			})
		}()
		synctest.Wait()
		if in, still := counted(); in != maxParallel || still != maxParallel {
			t.Errorf("%d were started and %d run before any has ended, want 8 and 8", in, still)
		}
		close(firstMayEnd)
		synctest.Wait()
		in, still := counted()
		select {
		case <-returned:
			t.Errorf("eachOf returned %v while %d of its work still ran", err, still)
		default:
		}
		if in != maxParallel || still != maxParallel-1 {
			t.Errorf("after the first failure %d were started and %d run, want 8 and 7", in, still)
		}
		close(othersMayEnd)
		<-returned
		if in, still := counted(); err != first || in != maxParallel || still != 0 {
			t.Errorf("eachOf = %v; %d were started and %d still run, want the first failure, 8 and 0", err, in, still)
		}
	})
}
