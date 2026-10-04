// Package storage keeps uploaded files. Files are stored under random keys,
// never under user-supplied names, and are not publicly served: the app
// decides who may read them.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Storage is where uploaded files live. Disk is the built-in implementation;
// an S3-compatible store can implement the same interface.
type Storage interface {
	// Save stores the content and returns its key and size.
	Save(content io.Reader) (key string, size int64, err error)
	// Open returns the stored file for reading (and seeking, for ranges).
	Open(key string) (File, error)
	// Delete removes the stored file.
	Delete(key string) error
}

// File is an opened stored file.
type File interface {
	io.ReadSeekCloser
	ModTime() time.Time
}

// ErrInvalidKey is returned for a key that was not produced by Save.
var ErrInvalidKey = errors.New("storage: invalid key")

// Keys look like 2026/10/<32 hex chars>; anything else is rejected, so a key
// can never point outside the storage directory.
var keyPattern = regexp.MustCompile(`^\d{4}/\d{2}/[0-9a-f]{32}$`)

// Disk stores files in a directory on the local disk.
type Disk struct {
	Root string
}

// NewDisk returns a Disk storing files under root (created when needed).
func NewDisk(root string) *Disk { return &Disk{Root: root} }

func (d *Disk) Save(content io.Reader) (string, int64, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", 0, err
	}
	now := time.Now().UTC()
	key := fmt.Sprintf("%04d/%02d/%s", now.Year(), int(now.Month()), hex.EncodeToString(random))

	path := filepath.Join(d.Root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", 0, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, err
	}
	size, err := io.Copy(f, content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return "", 0, err
	}
	return key, size, nil
}

type diskFile struct {
	*os.File
	modTime time.Time
}

func (f diskFile) ModTime() time.Time { return f.modTime }

func (d *Disk) Open(key string) (File, error) {
	if !keyPattern.MatchString(key) {
		return nil, ErrInvalidKey
	}
	f, err := os.Open(filepath.Join(d.Root, filepath.FromSlash(key)))
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return diskFile{File: f, modTime: info.ModTime()}, nil
}

func (d *Disk) Delete(key string) error {
	if !keyPattern.MatchString(key) {
		return ErrInvalidKey
	}
	err := os.Remove(filepath.Join(d.Root, filepath.FromSlash(key)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
