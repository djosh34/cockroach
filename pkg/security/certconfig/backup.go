// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package certconfig

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/robfig/cron/v3"
)

// Backup is the backup section of the --config file. It names the S3
// repository and when to back up. S3 credentials come from the node's
// environment through the AWS SDK's default chain.
//
// Example:
//
//	backup:
//	  bucket: backups
//	  prefix: prod             # optional, the repository's folder in the bucket
//	  endpoint: https://s3.example.com # optional, for S3-compatible servers
//	  region: us-east-1        # optional
//	  full-schedule: "0 3 * * *" # cron, in UTC
//	  archive-interval: 60s    # optional, default 60s
//	  keep-full: 2             # full backups to keep, with the archive they need
type Backup struct {
	Bucket string `yaml:"bucket"`
	Prefix string `yaml:"prefix"`
	// Endpoint is the URL of an S3-compatible server. When set, the client
	// uses path-style addressing.
	Endpoint     string `yaml:"endpoint"`
	Region       string `yaml:"region"`
	FullSchedule string `yaml:"full-schedule"`
	// ArchiveInterval is how often an archive segment is made. It is the
	// interval, not a bound on data loss: a segment exists only once its
	// upload has finished.
	ArchiveInterval time.Duration `yaml:"archive-interval"`
	KeepFull        int           `yaml:"keep-full"`
}

// DefaultArchiveInterval is the archive interval when the config sets none.
const DefaultArchiveInterval = 60 * time.Second

func (b *Backup) check() error {
	if b.Bucket == "" {
		return errors.New("backup: bucket is required")
	}
	if b.FullSchedule == "" {
		return errors.New("backup: full-schedule is required")
	}
	if _, err := b.Schedule(); err != nil {
		return err
	}
	if b.ArchiveInterval == 0 {
		b.ArchiveInterval = DefaultArchiveInterval
	}
	if b.ArchiveInterval < 0 {
		return errors.New("backup: archive-interval must be positive")
	}
	if b.KeepFull < 1 {
		return errors.New("backup: keep-full must be at least 1")
	}
	return nil
}

// Schedule parses the full-backup schedule.
func (b *Backup) Schedule() (cron.Schedule, error) {
	s, err := cron.ParseStandard(b.FullSchedule)
	if err != nil {
		return nil, errors.Wrapf(err, "backup: parsing full-schedule %q", b.FullSchedule)
	}
	return s, nil
}

// Lines returns the section as sorted "name: value" lines, for comparing
// the sections that the nodes have loaded.
func (b *Backup) Lines() []string {
	if b == nil {
		return nil
	}
	return []string{
		fmt.Sprintf("archive-interval: %s", b.ArchiveInterval),
		fmt.Sprintf("bucket: %s", b.Bucket),
		fmt.Sprintf("endpoint: %s", b.Endpoint),
		fmt.Sprintf("full-schedule: %s", b.FullSchedule),
		fmt.Sprintf("keep-full: %d", b.KeepFull),
		fmt.Sprintf("prefix: %s", b.Prefix),
		fmt.Sprintf("region: %s", b.Region),
	}
}
