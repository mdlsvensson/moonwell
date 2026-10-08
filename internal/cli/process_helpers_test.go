package cli

import (
	"bufio"
	"io"
)

func sendLines(stream io.Reader, lines chan<- string) {
	defer close(lines)
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

func readAll(stream io.Reader) ([]byte, error) { return io.ReadAll(stream) }
