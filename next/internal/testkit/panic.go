package testkit

// Panic is what call panics with, and nil when it returns. A test that gives a reader many damaged inputs asks
// it, so that its failure names the input and the test goes on to say so.
func Panic(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
