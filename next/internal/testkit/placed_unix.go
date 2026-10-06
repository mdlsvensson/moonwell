//go:build !windows

package testkit

// shortSpelling is "": only Windows writes a folder with short names.
func shortSpelling(string) string { return "" }
