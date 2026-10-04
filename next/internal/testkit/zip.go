package testkit

import (
	"archive/zip"
	"bytes"
	"testing"
)

// ZipEntry is one entry of a test archive. Its name is written as given, whatever it is: a name that ends with
// "/" is a folder entry, and one that points outside the archive ("../x", "/x") is how a test builds an archive
// that must not be trusted.
type ZipEntry struct {
	Name    string
	Data    []byte
	Deflate bool // compressed; stored as it is otherwise
}

// Zip is a zip archive of the entries in the order given. comment is the archive's comment, where an archive of
// a tag that GitHub serves holds the commit.
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
