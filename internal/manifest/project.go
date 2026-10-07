package manifest

// Project is an evaluated manifest: Project.pkl.
type Project struct {
	// Root is the project folder: the Root of the Env the project was loaded with.
	Root string `json:"-"`
	// File is the manifest that was evaluated, from Root: moonwell.local.pkl when it exists, else moonwell.pkl.
	File string `json:"-"`

	Map       Map                `json:"map"`
	Build     Build              `json:"build"`
	Launch    Launch             `json:"launch"`
	Yue       Yue                `json:"yue"`
	Assets    Assets             `json:"assets"`
	Lint      Lint               `json:"lint"`
	Libraries map[string]Library `json:"libraries"` // by the key that names the library's folder
	Settings  Settings           `json:"settings"`
	Objects   Objects            `json:"objects"`
}

// Map names the source map and the gameplay entry.
type Map struct {
	Folder string `json:"folder"` // under maps/
	Entry  string `json:"entry"`  // from the project folder
}

// Build says where and how the map is packed.
type Build struct {
	Folder string `json:"folder"`
	Minify bool   `json:"minify"`
}

// Launch says how the game is started.
type Launch struct {
	GameExecutable *string  `json:"gameExecutable"`
	Args           []string `json:"args"`
}

// Yue pins the YueScript compiler.
type Yue struct {
	Version string  `json:"version"`
	Path    *string `json:"path"` // a compiler to use in place of the pinned one
}

// Assets is the manifest's assets block.
type Assets struct {
	Paths   Ordered[string] `json:"paths"` // a file under assets/ to its exact in-map path
	Exclude []string        `json:"exclude"`
}

// Lint configures the check for globals that nothing defines.
type Lint struct {
	UnknownGlobals string   `json:"unknownGlobals"` // "error" or "warning"
	Globals        []string `json:"globals"`
}

// Library is a library of modules and of files for the map: a tag of a GitHub repository, or a local folder when
// Path is set.
type Library struct {
	GitHub *string `json:"github"`
	Tag    *string `json:"tag"`
	Path   *string `json:"path"`
	Dir    string  `json:"dir"` // the folder inside the library that module names start from; "" when it names none
}
