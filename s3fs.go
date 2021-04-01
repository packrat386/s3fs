package s3fs

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
	headres, err := s.client.HeadObject(&s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
	})

	// the error handling here looks strange because it is. If this request didn't
	// fail then we have a file. If it failed in a very particular way (key not found)
	// then we still want to check if it's a directory. Any other failure we wrap
	// and propagate.
	if err == nil {
		file := s3File{fileInfo: s3FileInfo{name: name, mode: fs.FileMode(0400)}}

		if headres.ContentLength != nil {
			file.fileInfo.size = *headres.ContentLength
		}

		if headres.LastModified != nil {
			file.fileInfo.modTime = *headres.LastModified
		}

		return &file, nil
	}

	awsErr, ok := err.(awserr.Error)
	if !ok {
		return nil, fmt.Errorf("error HEADing s3 file: %w", err)
	}

	if awsErr.Code() != s3.ErrCodeNoSuchKey {
		return nil, fmt.Errorf("error HEADing s3 file: %w", err)
	}

	// if we got here the key didn't exist, so check to see if `name` is a directory
	listres, err := s.client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  &s.bucket,
		MaxKeys: aws.Int64(1),
		Prefix:  aws.String(name),
	})

	if err != nil {
		return nil, fmt.Errorf("error listing s3 dir: %w", err)
	}

	if listres.KeyCount == nil || *listres.KeyCount == 0 {
		return nil, fs.ErrNotExist
	}

	return &s3Directory{
		fileInfo: s3FileInfo{
			name: name,
			mode: fs.FileMode(0400) | fs.ModeDir,
			size: 0,
		},
	}, nil
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

type s3File struct {
	fileInfo s3FileInfo
}

func (f *s3File) Stat() (fs.FileInfo, error) {
	return &f.fileInfo, nil
}

func (f *s3File) Read(buf []byte) (int, error) {
	panic("not implemented")
}

func (f *s3File) Close() error {
	panic("not implemented")
}

type s3Directory struct {
	fileInfo s3FileInfo
}

func (d *s3Directory) Stat() (fs.FileInfo, error) {
	return &d.fileInfo, nil
}

func (d *s3Directory) Read(buf []byte) (int, error) {
	panic("not implemented")
}

func (d *s3Directory) Close() error {
	panic("not implemented")
}

func (d *s3Directory) ReadDir(n int) ([]fs.DirEntry, error) {
	panic("not implemented")
}
