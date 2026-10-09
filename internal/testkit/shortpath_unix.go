//go:build !windows

package testkit

func shortPathName(string) string { return "" }
