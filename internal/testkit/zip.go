package testkit

import (
	"archive/zip"
	"bytes"
	"testing"
)

// ZipEntry is one entry of a test archive. A name that ends with "/" is a folder entry.
type ZipEntry struct {
	Name    string
	Data    []byte
	Deflate bool
}

// MakeZip writes a zip archive. comment is the archive comment, where GitHub's tag archives hold the commit SHA.
func MakeZip(t testing.TB, entries []ZipEntry, comment string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range entries {
		method := zip.Store
		if entry.Deflate {
			method = zip.Deflate
		}
		file, err := writer.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(entry.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
