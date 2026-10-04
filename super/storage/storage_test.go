package storage

import (
	"io"
	"strings"
	"testing"
)

func TestDiskSaveOpenDelete(t *testing.T) {
	d := NewDisk(t.TempDir())
	key, size, err := d.Save(strings.NewReader("hej verden"))
	if err != nil {
		t.Fatal(err)
	}
	if size != 10 || !keyPattern.MatchString(key) {
		t.Fatalf("key %q size %d", key, size)
	}
	f, err := d.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(f)
	f.Close()
	if string(data) != "hej verden" {
		t.Fatalf("content %q", data)
	}
	if err := d.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Open(key); err == nil {
		t.Fatal("file still there after Delete")
	}
}

func TestDiskRejectsForeignKeys(t *testing.T) {
	d := NewDisk(t.TempDir())
	for _, key := range []string{"../../etc/passwd", "2026/10/../../x", "/etc/passwd", "2026/10/ABC", ""} {
		if _, err := d.Open(key); err != ErrInvalidKey {
			t.Errorf("Open(%q) = %v, want ErrInvalidKey", key, err)
		}
		if err := d.Delete(key); err != ErrInvalidKey {
			t.Errorf("Delete(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}
