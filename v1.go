package s3fs

import (
	"fmt"
	"io/fs"
	"path"

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
	duplicateName := false
	err := s.client.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket:    &s.bucket,
			Delimiter: aws.String("/"),
			Prefix:    aws.String(name),
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				if *obj.Key == name {
					duplicateName = true
					return false
				}

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

	if duplicateName {
		return nil, fmt.Errorf("directory name matches file name: %s", name)
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
