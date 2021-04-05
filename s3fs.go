package s3fs

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type s3FS struct {
	client *s3.S3
	bucket string
}

func NewS3FS(client *s3.S3, bucket string) fs.FS {
	return &s3FS{
		client: client,
		bucket: bucket,
	}
}

func (s *s3FS) Open(name string) (fs.File, error) {
	name, err := trimName(name)
	if err != nil {
		return nil, fmt.Errorf("could not format filename: %w", err)
	}

	// special case root of the bucket
	if name == "" {
		return openDir(s, name)
	}

	// could be either a file or a directory at this point, so list with the name as a prefix.
	// if we find an exact match for either an object or a common prefix, then open that.
	// if neither match the name exactly then for our purposes it doesn't exist.
	//
	// note that because s3 isn't really a filesystem, its possible to find both an object
	// and a common prefix with the same exact name as `name`. If both match return an error.

	fileMatch := false
	dirMatch := false

	err = s.client.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket:    &s.bucket,
			Delimiter: aws.String("/"),
			Prefix:    aws.String(name),
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				if *obj.Key == name {
					fileMatch = true
				}
			}

			for _, cp := range page.CommonPrefixes {
				if name+"/" == *cp.Prefix {
					dirMatch = true
				}
			}

			return true
		},
	)

	if err != nil {
		return nil, fmt.Errorf("could not list s3 objects: %w", err)
	}

	if fileMatch && dirMatch {
		return nil, fmt.Errorf("directory name matches file name: %s", name)
	}

	if fileMatch {
		return openFile(s, name)
	}

	if dirMatch {
		return openDir(s, name+"/")
	}

	return nil, fs.ErrNotExist
}

func openDir(s *s3FS, name string) (fs.File, error) {
	entries := []fs.DirEntry{}
	err := s.client.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket:    &s.bucket,
			Delimiter: aws.String("/"),
			Prefix:    aws.String(name),
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				entries = append(
					entries,
					&s3FileInfo{
						name:    path.Base(*obj.Key),
						mode:    fs.FileMode(0400),
						size:    *obj.Size,
						modTime: *obj.LastModified,
					},
				)
			}

			for _, cp := range page.CommonPrefixes {
				entries = append(
					entries,
					&s3FileInfo{
						name: path.Base(*cp.Prefix),
						mode: fs.FileMode(0400) | fs.ModeDir,
						size: 0,
					},
				)
			}

			return true
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error listing s3 dir: %w", err)
	}

	if len(entries) == 0 {
		return nil, fs.ErrNotExist
	}

	return &s3Directory{
		entries: entries,
		ptr:     0,
		fileInfo: s3FileInfo{
			name: path.Base(name),
			mode: fs.FileMode(0400) | fs.ModeDir,
			size: 0,
		},
	}, nil
}

func openFile(s *s3FS, name string) (fs.File, error) {
	object, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
	})

	if err != nil {
		return nil, fmt.Errorf("error getting s3 object: %w", err)
	}

	return &s3File{
		body: object.Body,
		fileInfo: s3FileInfo{
			name:    path.Base(name),
			mode:    fs.FileMode(0400),
			size:    *object.ContentLength,
			modTime: *object.LastModified,
		},
	}, nil
}

func trimName(name string) (string, error) {
	if name == "./." || name == "/" {
		return "", fmt.Errorf("invalid name: %s", name)
	}

	if name == "." {
		return "", nil
	}

	return strings.TrimSuffix(
		strings.TrimPrefix(
			name,
			"./",
		),
		"/",
	), nil
}

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
