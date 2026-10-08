package testkit

import (
	"archive/zip"
	"bytes"
	"testing"
)

type ZipEntry struct {
	Name    string
	Data    []byte
	Deflate bool
}

func Zip(t testing.TB, comment string, entries ...ZipEntry) []byte {
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
			t.Fatalf("the zip entry %s: %v", entry.Name, err)
			return nil
		}
		if _, err := file.Write(entry.Data); err != nil {
			t.Fatalf("the zip entry %s: %v", entry.Name, err)
			return nil
		}
	}
	if err := writer.SetComment(comment); err != nil {
		t.Fatalf("the zip comment: %v", err)
		return nil
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("the zip archive: %v", err)
		return nil
	}
	return out.Bytes()
}
