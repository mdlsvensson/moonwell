package build

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

type WatchTiming struct{ Interval, Debounce time.Duration }

var DefaultWatchTiming = WatchTiming{Interval: 250 * time.Millisecond, Debounce: 150 * time.Millisecond}

func Dev(ctx context.Context, e *env.Env, timing WatchTiming) error {
	if timing.Interval <= 0 {
		return errors.New("build.Dev: the timing has no interval; pass DefaultWatchTiming")
	}
	if !fsx.IsDir(filepath.Join(e.Root, sourcesDir)) {
		return errNoSources(e.Root)
	}
	projectSet := projectWatchSet(e.Root)
	watcher := newWatcher(projectSet.roots)
	checkCtx := context.WithoutCancel(ctx)
	findPkl := rememberPkl(toolchain.FindPkl)
	runCheckCycle(checkCtx, e, findPkl)
	manifestSet := manifestWatchSet(e.Root, loadInitialSettings(e))
	watcher.add(manifestSet.roots...)
	e.Log.Info(projectSet.merge(manifestSet).describe())

	ticker := time.NewTicker(timing.Interval)
	defer ticker.Stop()
	var pending pendingChange
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			changed := watcher.poll()
			if ctx.Err() == nil && pending.isDue(changed, time.Now(), timing.Debounce) {
				runCheckCycle(checkCtx, e, findPkl)
			}
		}
	}
	return nil
}

func rememberPkl(findPkl PklFinder) PklFinder {
	found := ""
	return func(ctx context.Context, e *env.Env) (string, error) {
		if found != "" {
			return found, nil
		}
		program, err := findPkl(ctx, e)
		if err == nil {
			found = program
		}
		return program, err
	}
}

func runCheckCycle(ctx context.Context, e *env.Env, findPkl PklFinder) {
	if _, err := runCheck(ctx, e, findPkl, true); err != nil {
		e.Log.Error(diag.Format(err))
	}
}

func loadInitialSettings(e *env.Env) *manifest.Project {
	project, err := manifest.Load(e)
	if err != nil {
		return nil
	}
	return project
}

type pendingChange struct {
	isWaiting bool
	since     time.Time
}

func (c *pendingChange) isDue(changed bool, now time.Time, debounce time.Duration) bool {
	if changed {
		c.isWaiting, c.since = true, now
	}
	if !c.isWaiting || now.Sub(c.since) < debounce {
		return false
	}
	c.isWaiting = false
	return true
}

func errNoSources(root string) error {
	return &diag.Error{
		Msg:  "The src/ folder is missing.",
		File: root,
		Hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
	}
}
