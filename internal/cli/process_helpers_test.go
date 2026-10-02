package cli_test

import (
	"bufio"
	"io"
)

// readProcessLines sends each line a program writes, and closes the channel when the program closes the stream.
func readProcessLines(reader io.Reader, lines chan<- string) {
	defer close(lines)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

func readAllProcess(reader io.Reader) ([]byte, error) { return io.ReadAll(reader) }
