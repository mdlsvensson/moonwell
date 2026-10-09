package env

import (
	"net/http"
)

type Env struct {
	Root      string
	Log       *Logger
	Run       RunFunc
	Fetch     FetchFunc
	Spawn     func(program string, args []string) error
	CacheDir  string
	ConfigDir string
	Platform  string
}

func New(root string, log *Logger) *Env {
	return &Env{
		Root:      root,
		Log:       log,
		Run:       Run,
		Fetch:     HTTPFetch(http.DefaultClient),
		Spawn:     SpawnDetached,
		CacheDir:  DefaultCacheDir(),
		ConfigDir: DefaultConfigDir(),
		Platform:  CurrentPlatform(),
	}
}
