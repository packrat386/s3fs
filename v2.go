package s3fs

import (
	"context"
	"fmt"
	"io/fs"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3FSV2 struct {
	client *s3.Client
	bucket string
}

func NewS3FSV2(client *s3.Client, bucket string) fs.FS {
	return &s3FSV2{
		client: client,
		bucket: bucket,
	}
}

func (s *s3FSV2) Open(name string) (fs.File, error) {
	name, err := trimName(name)
	if err != nil {
		return nil, fmt.Errorf("could not format filename: %w", err)
	}

	if name == "" {
		return openDirV2(s, name)
	}

	// could be either a file or a directory at this point, so list with the name as a prefix.
	// if we find an exact match for either an object or a common prefix, then open that.
	// if neither match the name exactly then for our purposes it doesn't exist.
	//
	// note that because s3 isn't really a filesystem, its possible to find both an object
	// and a common prefix with the same exact name as `name`. If both match return an error.

	fileMatch := false
	dirMatch := false

	paginator := s3.NewListObjectsV2Paginator(
		s.client,
		&s3.ListObjectsV2Input{
			Bucket:    &s.bucket,
			Delimiter: aws.String("/"),
			Prefix:    aws.String(name),
		},
	)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("could not list s3 objects: %w", err)
		}

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
	}

	if fileMatch && dirMatch {
		return nil, fmt.Errorf("directory name matches file name: %s", name)
	}

	if fileMatch {
		return openFileV2(s, name)
	}

	if dirMatch {
		return openDirV2(s, name+"/")
	}

	return nil, fs.ErrNotExist
}

func openDirV2(s *s3FSV2, name string) (fs.File, error) {
	entries := []fs.DirEntry{}
	paginator := s3.NewListObjectsV2Paginator(
		s.client,
		&s3.ListObjectsV2Input{
			Bucket:    &s.bucket,
			Delimiter: aws.String("/"),
			Prefix:    aws.String(name),
		},
	)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error listing s3 dir: %w", err)
		}

		for _, obj := range page.Contents {
			if *obj.Key == name {
				return nil, fmt.Errorf("directory name matches file name: %s", name)
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

func openFileV2(s *s3FSV2, name string) (fs.File, error) {
	object, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
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
