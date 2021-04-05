s3fs
----

[![Go Reference](https://pkg.go.dev/badge/github.com/packrat386/s3fs.svg)](https://pkg.go.dev/github.com/packrat386/s3fs)

`io/fs` implementation backed by S3.

## How it works

This allows you to essentially treat S3 as a readable filsystem. `/` delimited common prefixes of keys are treated as "directories" with "files" at the base. So if you had an object with the key `some/long/key.json`, this would see a directory named `some` that contains a directory named `long` that contains a file named `key.json`. Implements the full `io/fs.FS` interface, so you can do all that fun stuff.

### Example

Reading a file

```go
package main

import (
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/packrat386/s3fs"
)

func main() {
	bucket := os.Getenv("S3FS_BUCKET")
	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)

	writeFile(client, bucket, "some/file.json", `{"my":"data"}`)

	myFS := s3fs.NewS3FS(client, bucket)

	f, err := myFS.Open("some/file.json")
	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, f)
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
```

Reading a directory

```go
package main

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/packrat386/s3fs"
)

func main() {
	bucket := os.Getenv("S3FS_BUCKET")
	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	client := s3.New(sess)

	writeFile(client, bucket, "mydir/foo.json", `{"data":"foo"}`)
	writeFile(client, bucket, "mydir/bar.json", `{"data":"bar"}`)
	writeFile(client, bucket, "mydir/baz.json", `{"data":"baz"}`)

	myFS := s3fs.NewS3FS(client, bucket)

	entries, err := fs.ReadDir(myFS, "mydir")
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		fmt.Println(entry.Name())
	}
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
```

### Caveats

S3 is not actually a filesystem, so there are some possible cases where you can have a "file" that has the same name as a "directory". For example if you have two keys name `some/file` and `some/file/or_is_it` then `some/file` is both a "file" and a "directory". This can also happen if you name a key with a trailing slash, for example `some/file/`. In both of those cases an attempt to open `some/file` or `some/file/` will return an error.

Also the concept of relative paths doesn't really exist. Your "working directory" is essentially the root of the bucket. `myfs.Open("/some/file.txt")` doesn't work, only `myfs.Open("some/file.txt")`, and you can't use `..` to change directories.

Finally S3 is not free, and this implementation isn't built to optimize for cost. If you have very large buckets it might be very expensive to walk them.

## Testing

Tests require AWS credentials and configuration to be provided in one of the normal ways consumed by the SDK (see: https://docs.aws.amazon.com/sdk-for-go/api/aws/session/). Additionally it requires that the `S3FS_TESTING_BUCKET` environment variable be set to the name of the bucket used for testing. The credentials and configuration available must be able to read and write to arbitrary keys in that bucket. 

As long as that configuration is available, you should be able to test with `go test`.

## Should I Use This?

This is pre v1, and not in any way battle tested, so I wouldn't for anything important. Really just making it open source so I can have a public implementation to look at.
