package manifest

type Project struct {
	Root string `json:"-"`
	File string `json:"-"`

	Map       Map                `json:"map"`
	Build     Build              `json:"build"`
	Launch    Launch             `json:"launch"`
	Yue       Yue                `json:"yue"`
	Assets    Assets             `json:"assets"`
	Lint      Lint               `json:"lint"`
	Libraries map[string]Library `json:"libraries"`
	Settings  Settings           `json:"settings"`
	Objects   Objects            `json:"objects"`
}

type Map struct {
	Folder string `json:"folder"`
	Entry  string `json:"entry"`
}

type Build struct {
	Folder string `json:"folder"`
	Minify bool   `json:"minify"`
}

type Launch struct {
	GameExecutable *string  `json:"gameExecutable"`
	Args           []string `json:"args"`
}

type Yue struct {
	Version string  `json:"version"`
	Path    *string `json:"path"`
}

type Assets struct {
	Paths   Ordered[string] `json:"paths"`
	Exclude []string        `json:"exclude"`
}

type Lint struct {
	UnknownGlobals string   `json:"unknownGlobals"`
	Globals        []string `json:"globals"`
}

type Library struct {
	GitHub *string `json:"github"`
	Tag    *string `json:"tag"`
	Path   *string `json:"path"`
	Dir    string  `json:"dir"`
}
