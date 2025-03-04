package s3fs

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

func TestS3FSV2(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	client := mustS3ClientV2()
	defer emptyBucketV2(client, bucket)

	writeFileV2(client, bucket, "top.json", `{"data":"top"}`)
	writeFileV2(client, bucket, "deep/down/top.json", `{"data":"liar"}`)
	writeFileV2(client, bucket, "dir-a/one.json", `{"data":"one"}`)
	writeFileV2(client, bucket, "dir-a/two.json", `{"data":"two"}`)
	writeFileV2(client, bucket, "dir-a/three.json", `{"data":"three"}`)
	writeFileV2(client, bucket, "dir-b/foo.json", `{"data":"foo"}`)
	writeFileV2(client, bucket, "dir-b/foo.json", `{"data":"bar"}`)

	myFS := NewS3FSV2(client, bucket)

	err := fstest.TestFS(
		myFS,
		"top.json",
		"deep/down/top.json",
		"dir-a/one.json",
		"dir-a/two.json",
		"dir-a/three.json",
		"dir-b/foo.json",
	)

	if err != nil {
		t.Fatal(err)
	}
}

func TestS3FSV2_ReadFile(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	client := mustS3ClientV2()
	defer emptyBucketV2(client, bucket)

	writeFileV2(client, bucket, "foo.json", `{"data":"foo"}`)

	myFS := NewS3FSV2(client, bucket)

	f, err := myFS.Open("foo.json")
	require.Nil(t, err)

	out := map[string]string{}
	err = json.NewDecoder(f).Decode(&out)
	require.Nil(t, err)
	require.Equal(t, "foo", out["data"])
}

func TestS3FSV2_ReadDir(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	client := mustS3ClientV2()
	defer emptyBucketV2(client, bucket)

	writeFileV2(client, bucket, "mydir/foo.json", `{"data":"foo"}`)
	writeFileV2(client, bucket, "mydir/bar.json", `{"data":"bar"}`)
	writeFileV2(client, bucket, "mydir/baz.json", `{"data":"baz"}`)

	myFS := NewS3FSV2(client, bucket)

	entries, err := fs.ReadDir(myFS, "mydir")
	require.Nil(t, err)
	require.Equal(t, 3, len(entries))
	require.True(t, dirEntriesContains(entries, "foo.json"))
	require.True(t, dirEntriesContains(entries, "bar.json"))
	require.True(t, dirEntriesContains(entries, "baz.json"))
}

func TestS3FSV2_FileAndDir(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	client := mustS3ClientV2()
	defer emptyBucketV2(client, bucket)

	writeFileV2(client, bucket, "foo", `{"data":"foo"}`)
	writeFileV2(client, bucket, "foo/bar", `{"data":"bar"}`)

	myFS := NewS3FSV2(client, bucket)

	_, err := myFS.Open("foo")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "directory name matches file name")
}

func TestS3FSV2_FileEndingWithSlash(t *testing.T) {
	bucket := os.Getenv("S3FS_TESTING_BUCKET")
	require.NotEqual(t, "", bucket, "S3FS_TESTING_BUCKET must be set")

	client := mustS3ClientV2()
	defer emptyBucketV2(client, bucket)

	writeFileV2(client, bucket, "weird/", `{"data":"weird"}`)

	myFS := NewS3FSV2(client, bucket)
	_, err := myFS.Open("weird/")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid name")
}

func mustS3ClientV2() *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	return s3.NewFromConfig(cfg)
}

func writeFileV2(client *s3.Client, bucket, key, body string) {
	_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Body:   strings.NewReader(body),
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		panic(err)
	}
}

func emptyBucketV2(client *s3.Client, bucket string) {
	keys := []string{}

	paginator := s3.NewListObjectsV2Paginator(
		client,
		&s3.ListObjectsV2Input{
			Bucket: &bucket,
		},
	)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			fmt.Println("ERROR: could not delete objects after testing. Manual fix may be required")
			panic(err)
		}

		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}

	for _, key := range keys {
		_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})

		if err != nil {
			fmt.Println("ERROR: could not delete objects after testing. Manual fix may be required")
			panic(err)
		}
	}
}
