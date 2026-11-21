// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package backup

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/errors"
)

// ErrNotFound is returned by Store.Get for an object that does not exist.
var ErrNotFound = errors.New("object not found")

// partSize is the part size of multipart uploads. A smaller object is sent
// with one PutObject call.
const partSize = 8 << 20

// Store reads and writes the objects of one repository. It makes exactly
// these S3 calls: PutObject, CreateMultipartUpload, UploadPart,
// CompleteMultipartUpload, AbortMultipartUpload, GetObject, ListObjectsV2 and
// DeleteObject. The contract tests check each of them against a real S3
// server.
type Store struct {
	client *s3.Client
	//lint:ignore SA1019 the uploader of feature/s3/manager still works; its replacement transfermanager is new
	uploader *manager.Uploader
	bucket   string
	prefix   string
	// listPageSize is the number of keys per ListObjectsV2 call; 0 is the
	// server's default. Tests make it small to check paging.
	listPageSize int32
}

// Repository names an S3 repository: the backup section without the
// schedule.
type Repository struct {
	Bucket   string `json:"bucket"`
	Prefix   string `json:"prefix"`
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
}

// RepositoryOf returns the repository of a backup section.
func RepositoryOf(b *certconfig.Backup) Repository {
	return Repository{Bucket: b.Bucket, Prefix: b.Prefix, Endpoint: b.Endpoint, Region: b.Region}
}

// OpenStore returns a Store for the repository. Credentials come from the
// AWS SDK's default chain.
func OpenStore(ctx context.Context, r Repository) (*Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading the AWS config")
	}
	if r.Region != "" {
		cfg.Region = r.Region
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if r.Endpoint != "" {
			o.BaseEndpoint = aws.String(r.Endpoint)
			o.UsePathStyle = true
		}
		// Send and check checksums only where the API requires them. Some
		// S3-compatible servers reject the newer ones.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	//lint:ignore SA1019 the uploader of feature/s3/manager still works; its replacement transfermanager is new
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = partSize
	})
	return &Store{
		client:   client,
		uploader: uploader,
		bucket:   r.Bucket,
		prefix:   strings.Trim(r.Prefix, "/"),
	}, nil
}

func (s *Store) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

// Put writes the object name with the content of r. An upload is
// all-or-nothing: the object either has all of r or keeps what it had
// before. A large object is sent in parts; if that fails, the parts are
// aborted.
func (s *Store) Put(ctx context.Context, name string, r io.Reader) error {
	//lint:ignore SA1019 the uploader of feature/s3/manager still works; its replacement transfermanager is new
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(name)),
		Body:   r,
	})
	return errors.Wrapf(err, "uploading %s", name)
}

// Get returns the content of the object name, or ErrNotFound.
func (s *Store) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(name)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, errors.Wrapf(ErrNotFound, "%s", name)
		}
		return nil, errors.Wrapf(err, "reading %s", name)
	}
	return out.Body, nil
}

// List returns the names of all objects whose names start with dir + "/",
// in ascending byte order, without dir.
func (s *Store) List(ctx context.Context, dir string) ([]string, error) {
	full := s.key(dir) + "/"
	var names []string
	in := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(full),
	}
	if s.listPageSize > 0 {
		in.MaxKeys = aws.Int32(s.listPageSize)
	}
	p := s3.NewListObjectsV2Paginator(s.client, in)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "listing %s", dir)
		}
		for _, o := range page.Contents {
			names = append(names, strings.TrimPrefix(aws.ToString(o.Key), full))
		}
	}
	return names, nil
}

// Delete removes the object name. Removing an object that does not exist
// is not an error.
func (s *Store) Delete(ctx context.Context, name string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(name)),
	})
	return errors.Wrapf(err, "deleting %s", name)
}

// join joins object name parts.
func join(parts ...string) string {
	return path.Join(parts...)
}
