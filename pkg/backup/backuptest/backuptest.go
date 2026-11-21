// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package backuptest gives the backup tests an S3 bucket on a real S3
// server. The tests run only when PATCHROADDB_TEST_S3_ENDPOINT names the
// server; the s3 job of CI starts versitygw and sets it, with the
// credentials in AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. S3 is never
// mocked.
package backuptest

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
)

// EndpointEnv names the S3 server that the tests use.
const EndpointEnv = "PATCHROADDB_TEST_S3_ENDPOINT"

// Region is the region the tests use.
const Region = "us-east-1"

// Endpoint returns the S3 server's URL, or skips the test if none is set.
func Endpoint(t testing.TB) string {
	endpoint, _ := envutil.ExternalEnvString(EndpointEnv, 1)
	if endpoint == "" {
		skip.IgnoreLintf(t, "%s is not set; the s3 job of CI runs this test against versitygw", EndpointEnv)
	}
	return endpoint
}

// Client returns an S3 client for the server, for test setup only.
func Client(t testing.TB) *s3.Client {
	endpoint := Endpoint(t)
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(Region))
	if err != nil {
		t.Fatal(err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
}

// NewBucket creates an empty bucket with a random name and returns the
// name and the server's URL.
func NewBucket(t testing.TB) (bucket, endpoint string) {
	c := Client(t)
	bucket = fmt.Sprintf("test-%016x", rand.Uint64())
	if _, err := c.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		t.Fatal(err)
	}
	return bucket, Endpoint(t)
}
