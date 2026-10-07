package cli

import (
	"bufio"
	"io"
)

// sendLines sends each line a program writes to a stream, and closes the channel when the program closes the
// stream.
func sendLines(stream io.Reader, lines chan<- string) {
	defer close(lines)
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

// readAll is all that a program writes to a stream until it closes it.
func readAll(stream io.Reader) ([]byte, error) { return io.ReadAll(stream) }
