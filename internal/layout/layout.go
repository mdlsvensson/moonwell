// Package layout names the folders and files Moonwell keeps inside a project, relative to its root and with "/".
package layout

const (
	// LibrariesDir holds one folder of modules for each library of the manifest.
	LibrariesDir = ".moonwell/libraries"
	// LibraryAssetsDir holds one folder for each library that ships assets.
	LibraryAssetsDir = ".moonwell/library-assets"
	// ObjectIDsFile is the generated module with the ids of the manifest's objects.
	ObjectIDsFile = "src/generated/objects.yue"
)
