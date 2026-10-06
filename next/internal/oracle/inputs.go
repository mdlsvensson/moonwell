package oracle

import "github.com/mdlsvensson/moonwell/next/internal/testkit"

// Changed is testkit.Changed, for the oracles that ask for it here.
func Changed(text string, seed, index uint64) string { return testkit.Changed(text, seed, index) }

// Swept is testkit.Swept, for the oracles that ask for it here.
func Swept(text string) []string { return testkit.Swept(text) }

// PartingLine is testkit.PartingLine, for the oracles that ask for it here.
func PartingLine(data []byte, alike func(upTo []byte) bool) (int, []byte) {
	return testkit.PartingLine(data, alike)
}
