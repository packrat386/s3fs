package s3fs

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

type s3FileInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    fs.FileMode
}

func (fi *s3FileInfo) Name() string {
	return fi.name
}

func (fi *s3FileInfo) Size() int64 {
	return fi.size
}

func (fi *s3FileInfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi *s3FileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *s3FileInfo) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi *s3FileInfo) Sys() interface{} {
	return nil
}

func (fi *s3FileInfo) Info() (fs.FileInfo, error) {
	return fi, nil
}

func (fi *s3FileInfo) Type() fs.FileMode {
	return fi.Mode().Type()
}

type s3File struct {
	body     io.ReadCloser
	fileInfo s3FileInfo
}

func (f *s3File) Stat() (fs.FileInfo, error) {
	return &f.fileInfo, nil
}

func (f *s3File) Read(buf []byte) (int, error) {
	return f.body.Read(buf)
}

func (f *s3File) Close() error {
	return f.body.Close()
}

type s3Directory struct {
	entries  []fs.DirEntry
	ptr      int
	fileInfo s3FileInfo
}

func (d *s3Directory) Stat() (fs.FileInfo, error) {
	return &d.fileInfo, nil
}

func (d *s3Directory) Read(buf []byte) (int, error) {
	return 0, fmt.Errorf("cannot read a directory")
}

func (d *s3Directory) Close() error {
	return nil
}

func (d *s3Directory) ReadDir(n int) ([]fs.DirEntry, error) {
	out := []fs.DirEntry{}
	if n <= 0 {
		for d.ptr < len(d.entries) {
			out = append(out, d.entries[d.ptr])
			d.ptr++
		}

		return out, nil
	}

	target := d.ptr + n
	if target > len(d.entries) {
		target = len(d.entries)
	}

	for d.ptr < target {
		out = append(out, d.entries[d.ptr])
		d.ptr++
	}

	if len(out) == 0 {
		return nil, io.EOF
	}

	return out, nil
}

func trimName(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid name: %s", name)
	}

	if name == "." {
		return "", nil
	}

	return strings.TrimPrefix(
		name,
		"./",
	), nil
}
