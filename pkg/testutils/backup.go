// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testutils

import "os"

const (
	defaultBackupBucket              = "cockroachdb-backup-testing"
	longTTLBackupTestingBucket       = "cockroachdb-backup-testing-long-ttl"
	backupTestingBucketEnvVar        = "BACKUP_TESTING_BUCKET"
	backupTestingBucketLongTTLEnvVar = "BACKUP_TESTING_BUCKET_LONG_TTL"
)

// BackupTestingBucket returns the name of the external storage bucket that
// should be used in a test run. Most times, this will be the regular public
// bucket. In private test runs, the name of the bucket is passed through an
// environment variable.
func BackupTestingBucket() string {
	if bucket := os.Getenv(backupTestingBucketEnvVar); bucket != "" {
		return bucket
	}

	return defaultBackupBucket
}

// BackupTestingBucketLongTTL returns the name of the external storage bucket
// that should be used in a test run where the bucket's content may inform a
// debugging investigation. At the time of this comment, the ttl for the s3 and
// gcs buckets is 20 days.
//
// In private test runs, the name of the bucket is passed through an environment
// variable.
func BackupTestingBucketLongTTL() string {
	if bucket := os.Getenv(backupTestingBucketLongTTLEnvVar); bucket != "" {
		return bucket
	}

	return longTTLBackupTestingBucket
}
