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

type Pace struct{ Interval, Debounce time.Duration }

var DefaultPace = Pace{Interval: 250 * time.Millisecond, Debounce: 150 * time.Millisecond}

func Dev(ctx context.Context, e *env.Env, pace Pace) error {
	if pace.Interval <= 0 {
		return errors.New("build.Dev: the pace has no interval; pass DefaultPace")
	}
	if !fsx.IsDir(filepath.Join(e.Root, sourcesDir)) {
		return errNoSources(e.Root)
	}
	watch := ownFolders(e.Root)
	files := newWatcher(watch.roots)
	working := context.WithoutCancel(ctx)
	pkl := cycle(working, e, "")
	named := namedFolders(e.Root, startingManifest(working, e, pkl))
	files.add(named.roots...)
	e.Log.Info(watch.and(named).line())

	ticker := time.NewTicker(pace.Interval)
	defer ticker.Stop()
	var waiting unchecked
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			changed := files.poll()
			if ctx.Err() == nil && waiting.due(changed, time.Now(), pace.Debounce) {
				pkl = cycle(working, e, pkl)
			}
		}
	}
	return nil
}

func cycle(ctx context.Context, e *env.Env, pkl string) (found string) {
	if pkl == "" {
		program, err := toolchain.PklProgram(ctx, e)
		if err != nil {
			e.Log.Error(diag.Format(err))
			return ""
		}
		pkl = program
	}
	if _, err := check(ctx, e, pkl, true); err != nil {
		e.Log.Error(diag.Format(err))
	}
	return pkl
}

func startingManifest(ctx context.Context, e *env.Env, pkl string) *manifest.Project {
	if pkl == "" {
		return nil
	}
	p, err := manifest.Load(ctx, e, pkl)
	if err != nil {
		return nil
	}
	return p
}

type unchecked struct {
	waits bool
	since time.Time
}

func (u *unchecked) due(changed bool, now time.Time, debounce time.Duration) bool {
	if changed {
		u.waits, u.since = true, now
	}
	if !u.waits || now.Sub(u.since) < debounce {
		return false
	}
	u.waits = false
	return true
}

func errNoSources(root string) error {
	return &diag.Error{
		Msg:  "The src/ folder is missing.",
		File: root,
		Hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
	}
}
