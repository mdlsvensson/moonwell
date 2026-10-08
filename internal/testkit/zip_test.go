package testkit

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestZipWritesEveryEntryUnderTheNameGivenAndTheComment(t *testing.T) {
	entries := []ZipEntry{
		{Name: "lib-0.1.0/"},
		{Name: "lib-0.1.0/src/greet.lua", Data: []byte("return {}"), Deflate: true},
		{Name: "lib-0.1.0/README", Data: []byte("stored as it is")},
		{Name: "../escaped", Data: []byte("x")},
		{Name: "/rooted", Data: []byte("y"), Deflate: true},
	}
	const comment = "0123456789abcdef0123456789abcdef01234567"
	archive := Zip(t, comment, entries...)
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if reader == nil {
		t.Fatalf("the archive cannot be read: %v", err)
	}
	if reader.Comment != comment || len(reader.File) != len(entries) {
		t.Fatalf("the archive has the comment %q and %d entries", reader.Comment, len(reader.File))
	}
	for i, want := range entries {
		got := reader.File[i]
		method := zip.Store
		if want.Deflate {
			method = zip.Deflate
		}
		if got.Name != want.Name || got.Method != method {
			t.Errorf("entry %d is %q with the method %d, want %q with %d", i, got.Name, got.Method, want.Name, method)
		}
		file, err := got.Open()
		if err != nil {
			t.Fatalf("%s: %v", got.Name, err)
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil || !bytes.Equal(data, want.Data) {
			t.Errorf("%s holds %q, %v, want %q", got.Name, data, err, want.Data)
		}
	}
}

func TestZipOfNothingIsAnArchiveWithoutEntriesOrComment(t *testing.T) {
	archive := Zip(t, "")
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) != 0 || reader.Comment != "" {
		t.Errorf("the archive: %v, %+v", err, reader)
	}
}

func TestNeedNetworkSkipsTheTestUnlessNetworkTestsAreAskedFor(t *testing.T) {
	for value, skips := range map[string]int{"1": 0, "": 1, "0": 1, "true": 1} {
		t.Setenv("MOONWELL_NETWORK_TESTS", value)
		stand := newStandIn(t)
		NeedNetwork(stand)
		if len(stand.skipped) != skips || len(stand.failed) != 0 {
			t.Errorf("MOONWELL_NETWORK_TESTS=%q: skipped %q and failed %q, want %d skips", value, stand.skipped, stand.failed, skips)
		}
		if skips == 1 && len(stand.skipped) == 1 && !bytes.Contains([]byte(stand.skipped[0]), []byte("MOONWELL_NETWORK_TESTS=1")) {
			t.Errorf("the skip says %q, want it to name the variable", stand.skipped[0])
		}
	}
}
