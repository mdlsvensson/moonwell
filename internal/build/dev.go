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
	pkl := runCheckCycle(checkCtx, e, "")
	manifestSet := manifestWatchSet(e.Root, loadInitialManifest(checkCtx, e, pkl))
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
				pkl = runCheckCycle(checkCtx, e, pkl)
			}
		}
	}
	return nil
}

func runCheckCycle(ctx context.Context, e *env.Env, pkl string) (pklProgram string) {
	if pkl == "" {
		program, err := toolchain.FindPkl(ctx, e)
		if err != nil {
			e.Log.Error(diag.Format(err))
			return ""
		}
		pkl = program
	}
	if _, err := runCheck(ctx, e, pkl, true); err != nil {
		e.Log.Error(diag.Format(err))
	}
	return pkl
}

func loadInitialManifest(ctx context.Context, e *env.Env, pkl string) *manifest.Project {
	if pkl == "" {
		return nil
	}
	project, err := manifest.Load(ctx, e, pkl)
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
