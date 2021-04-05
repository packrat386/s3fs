package s3fs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/require"
)

func TestS3FS(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)

	defer emptyBucket(client, bucket)

	writeFile(client, bucket, "top.json", `{"data":"top"}`)
	writeFile(client, bucket, "deep/down/top.json", `{"data":"liar"}`)
	writeFile(client, bucket, "dir-a/one.json", `{"data":"one"}`)
	writeFile(client, bucket, "dir-a/two.json", `{"data":"two"}`)
	writeFile(client, bucket, "dir-a/three.json", `{"data":"three"}`)
	writeFile(client, bucket, "dir-b/foo.json", `{"data":"foo"}`)
	writeFile(client, bucket, "dir-b/foo.json", `{"data":"bar"}`)

	myFS := NewS3FS(client, bucket)

	if err := fstest.TestFS(myFS, "dir-b/foo.json"); err != nil {
		t.Fatal(err)
	}
}

func TestS3FS_ReadFile(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)
	defer emptyBucket(client, bucket)

	writeFile(client, bucket, "foo.json", `{"data":"foo"}`)

	myFS := NewS3FS(client, bucket)

	f, err := myFS.Open("foo.json")
	require.Nil(t, err)

	out := map[string]string{}
	err = json.NewDecoder(f).Decode(&out)
	require.Nil(t, err)
	require.Equal(t, "foo", out["data"])
}

func TestS3FS_ReadDir(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)
	defer emptyBucket(client, bucket)

	writeFile(client, bucket, "mydir/foo.json", `{"data":"foo"}`)
	writeFile(client, bucket, "mydir/bar.json", `{"data":"bar"}`)
	writeFile(client, bucket, "mydir/baz.json", `{"data":"baz"}`)

	myFS := NewS3FS(client, bucket)

	entries, err := fs.ReadDir(myFS, "mydir")
	require.Nil(t, err)
	require.Equal(t, 3, len(entries))
	require.True(t, dirEntriesContains(entries, "foo.json"))
	require.True(t, dirEntriesContains(entries, "bar.json"))
	require.True(t, dirEntriesContains(entries, "baz.json"))
}

func TestS3FS_FileAndDir(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)
	defer emptyBucket(client, bucket)

	writeFile(client, bucket, "foo", `{"data":"foo"}`)
	writeFile(client, bucket, "foo/bar", `{"data":"bar"}`)

	myFS := NewS3FS(client, bucket)

	_, err = myFS.Open("foo")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "directory name matches file name")
}

func TestS3FS_FileEndingWithSlash(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)
	defer emptyBucket(client, bucket)

	writeFile(client, bucket, "weird/", `{"data":"weird"}`)

	myFS := NewS3FS(client, bucket)
	_, err = myFS.Open("weird/")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid name")
}

func dirEntriesContains(entries []fs.DirEntry, name string) bool {
	for _, e := range entries {
		if e.Name() == name {
			return true
		}
	}

	return false
}

func writeFile(client *s3.S3, bucket, key, body string) {
	_, err := client.PutObject(&s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(strings.NewReader(body)),
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		panic(err)
	}
}

func emptyBucket(client *s3.S3, bucket string) {
	keys := []string{}

	err := client.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket: &bucket,
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				keys = append(keys, *obj.Key)
			}

			return true
		},
	)
	if err != nil {
		fmt.Println("ERROR: could not delete objects after testing. Manual fix may be required")
		panic(err)
	}

	for _, key := range keys {
		_, err := client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})

		if err != nil {
			fmt.Println("ERROR: could not delete objects after testing. Manual fix may be required")
			panic(err)
		}
	}
}
