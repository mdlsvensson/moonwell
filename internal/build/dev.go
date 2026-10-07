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

// This file holds dev: the door and one cycle of it, and when a check is due. What a project is watched for, and
// which changes of a file count, is in dev_watched.go. The watcher itself is in watch.go.

// Pace is how often dev looks at the files, and how long they must stay unchanged before it checks again. A
// Debounce of zero is none: the look that finds a change starts the check.
type Pace struct{ Interval, Debounce time.Duration }

// DefaultPace is a look every 250 ms and a check 150 ms after the last change.
var DefaultPace = Pace{Interval: 250 * time.Millisecond, Debounce: 150 * time.Millisecond}

// Dev checks the project, and checks it again whenever a source, an object file, a manifest, an asset, a local
// library or the preview picture changes, until ctx is cancelled. A check that is under way is finished first.
//
// It returns nil when it was told to stop. A check that fails is logged, and the watching goes on. Each check
// writes the ids module, as a build does. What a check writes is not watched, or each check would start the
// next: of the project's own folders nothing below dist/ and .moonwell/ is, and src/generated/ never counts.
//
// The project's own folders are looked at before the first check, so what is saved in them while it runs is
// checked after it: a source, an object file, an asset, a manifest. The local libraries and the preview picture
// are known by the manifest, as it evaluates after the first check, and are watched from then: what is saved in
// them while the first check runs is found by no look, and a library the manifest names later is watched by the
// next Dev. Pkl is looked for by each check until one finds it, and that program is kept.
func Dev(ctx context.Context, e *env.Env, pace Pace) error {
	if pace.Interval <= 0 {
		// A plain error: the pace is the command's own, DefaultPace or a test's, so one without an interval is a
		// mistake in Moonwell and nothing the user can put right.
		return errors.New("build.Dev: the pace has no interval; pass DefaultPace")
	}
	if !fsx.IsDir(filepath.Join(e.Root, sourcesDir)) {
		return errNoSources(e.Root)
	}
	// The first look at the project's own folders comes before the first check, which writes nothing there that
	// counts: the look after it finds what was saved meanwhile.
	watch := ownFolders(e.Root)
	files := newWatcher(watch.roots)
	// Ctrl+C ends the watching and not a check that is under way: the check holds the build lock, and gives it
	// back as it ends.
	working := context.WithoutCancel(ctx)
	pkl := cycle(working, e, "")
	named := namedFolders(e.Root, startingManifest(working, e, pkl))
	files.add(named.roots...)
	// Every first look is taken before the line that says what is watched: what is saved as the line appears is
	// found by the next look.
	e.Log.Info(watch.and(named).line())

	ticker := time.NewTicker(pace.Interval)
	defer ticker.Stop()
	var waiting unchecked
	// No check is started once Dev was told to stop. Told while a check runs, it returns as the check ends, and
	// takes no further look; told while a look is taken, it starts no check for what that look found.
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

// cycle is one check of the project, with the ids module written. A failure is logged and ends nothing.
//
// pkl is the Pkl program an earlier cycle found, "" when none has: the cycle then looks for it, and without one
// there is no check. It returns the program for the cycles after it, "" when there is none still.
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

// startingManifest is the project's manifest as it evaluates when dev starts, for what it names to watch. It is
// nil without a Pkl program, and for a manifest that does not load: the first cycle has said why.
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

// unchecked is the changes of the files that no check has answered.
type unchecked struct {
	waits bool      // a look found a change, and no check has started since
	since time.Time // when a look last found one
}

// due notes what a look at the files found, at the time now, and reports whether a check is due: whether a look
// has found a change since the last check, and no look has found one for the debounce. A check that is due is
// taken as started. The time is that of the look and not that of the change, which no look tells: files that are
// being saved one after the other are checked when the last of them has lain still.
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

// ---- errors ----

// errNoSources names the project folder, as the refusal of a folder without a manifest does: the folder is what
// is no project.
func errNoSources(root string) error {
	return &diag.Error{
		Msg:  "The src/ folder is missing.",
		File: root,
		Hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
	}
}
