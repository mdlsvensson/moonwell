// Package env is everything Moonwell uses from outside the program, gathered in one struct: running another program,
// downloading an address, starting a program that outlives Moonwell, the logger, the per-user cache folder and the
// platform.
//
// New takes a project folder and a logger and returns the Env of the real world; a test builds an Env of its own, or
// replaces the parts it wants to watch or to keep from happening. Run takes a program and its arguments and returns
// what it printed and its exit code. A FetchFunc takes an address and returns the response's status and body. A
// Logger takes lines and passes them to a sink and to a log file. A program that cannot be started is a *diag.Error;
// every other failure is the error as it came, for the caller to word.
//
// The package must not know what it is used for: it names no tool it runs or downloads, reads no manifest and no
// project file, and decides nothing about a build. It imports no package of Moonwell but diag and fsx.
package env

import "net/http"

// Env is everything a command needs from outside the program. Tests replace parts of it.
type Env struct {
	Root     string // the project folder
	Log      *Logger
	Run      RunFunc
	Fetch    FetchFunc
	Spawn    func(program string, args []string) error // detached: the game
	CacheDir string
	Platform string // "windows-x86_64", "linux-x86_64" or ""
}

// New is the real world for the project at root: programs are run, downloads go over HTTP, and the cache folder and
// the platform are this user's and this machine's.
func New(root string, log *Logger) *Env {
	return &Env{
		Root:     root,
		Log:      log,
		Run:      Run,
		Fetch:    HTTPFetch(http.DefaultClient),
		Spawn:    SpawnDetached,
		CacheDir: DefaultCacheDir(),
		Platform: CurrentPlatform(),
	}
}
