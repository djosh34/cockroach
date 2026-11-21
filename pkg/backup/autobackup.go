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
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobsprotectedts"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/protectedts"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/protectedts/ptpb"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
)

// The AUTO BACKUP job runs while the cluster runs, on one node at a time.
// It uses the backup section of that node's --config. Every archive
// interval it uploads the next segment; when the full-backup schedule is
// due, it takes a full backup at the end time of the segment it just
// uploaded, while the archive goes on. After each full backup it expires
// the old ones.
//
// The job's protected timestamp keeps the history that the next segment
// and a running full backup still read. It moves forward only after an
// upload has finished. A job that starts without one, because it is new,
// cannot know whether history after the last segment was garbage
// collected; it first takes a full backup, and the archive goes on from
// there.

// autoBackupCheckInterval is how often a node with a backup section checks
// that the AUTO BACKUP job exists.
const autoBackupCheckInterval = 5 * time.Minute

// StartAutoBackup makes sure, now and every autoBackupCheckInterval, that
// the AUTO BACKUP job exists. Only nodes with a backup section call it.
func StartAutoBackup(ctx context.Context, stopper *stop.Stopper, execCfg *sql.ExecutorConfig) error {
	return stopper.RunAsyncTask(ctx, "auto-backup-creator", func(ctx context.Context) {
		ctx, cancel := stopper.WithCancelOnQuiesce(ctx)
		defer cancel()
		var timer timeutil.Timer
		defer timer.Stop()
		for {
			if err := createAutoBackupJob(ctx, execCfg); err != nil {
				log.Ops.Warningf(ctx, "creating the AUTO BACKUP job: %v", err)
			}
			timer.Reset(autoBackupCheckInterval)
			select {
			case <-timer.C:
				timer.Read = true
			case <-ctx.Done():
				return
			}
		}
	})
}

func createAutoBackupJob(ctx context.Context, execCfg *sql.ExecutorConfig) error {
	record := jobs.Record{
		JobID:         execCfg.JobRegistry.MakeJobID(),
		Description:   "backups to the repository in the backup section of --config",
		Username:      username.NodeUserName(),
		Details:       jobspb.AutoBackupDetails{},
		Progress:      jobspb.AutoBackupProgress{},
		NonCancelable: true,
	}
	created := false
	if err := execCfg.InternalDB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		created = false
		exists, err := jobs.RunningJobExists(ctx, jobspb.InvalidJobID, txn,
			execCfg.Settings.Version, jobspb.TypeAutoBackup)
		if err != nil || exists {
			return err
		}
		_, err = execCfg.JobRegistry.CreateJobWithTxn(ctx, record, record.JobID, txn)
		created = err == nil
		return err
	}); err != nil {
		return err
	}
	if created {
		execCfg.JobRegistry.NotifyToResume(ctx, record.JobID)
	}
	return nil
}

type autoBackupResumer struct {
	job *jobs.Job
}

var _ jobs.Resumer = (*autoBackupResumer)(nil)

// Resume runs until the job's context ends.
func (r *autoBackupResumer) Resume(ctx context.Context, execCtx interface{}) error {
	execCfg := execCtx.(sql.JobExecContext).ExecCfg()
	if execCfg.BackupConfig == nil {
		// Fail, so that the protected timestamp goes. A node with a backup
		// section starts a new job.
		return jobs.MarkAsPermanentJobError(errors.New(
			"the node that ran this job has no backup section in its --config"))
	}
	a := &archiver{
		job:     r.job,
		execCfg: execCfg,
		cfg:     execCfg.BackupConfig,
	}
	return a.run(ctx)
}

// OnFailOrCancel releases the protected timestamp.
func (r *autoBackupResumer) OnFailOrCancel(ctx context.Context, execCtx interface{}, _ error) error {
	execCfg := execCtx.(sql.JobExecContext).ExecCfg()
	p, ok := r.job.Progress().Details.(*jobspb.Progress_AutoBackup)
	if !ok || p.AutoBackup.ProtectedTimestampRecord == nil {
		return nil
	}
	id := p.AutoBackup.ProtectedTimestampRecord
	return execCfg.InternalDB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		err := execCfg.ProtectedTimestampProvider.WithTxn(txn).Release(ctx, *id)
		if errors.Is(err, protectedts.ErrNotExists) {
			return nil
		}
		return err
	})
}

// CollectProfile implements jobs.Resumer.
func (r *autoBackupResumer) CollectProfile(context.Context, interface{}) error { return nil }

type archiver struct {
	job     *jobs.Job
	execCfg *sql.ExecutorConfig
	cfg     *certconfig.Backup
	store   *Store

	clusterID      string
	clusterVersion string
	pts            uuid.UUID
	ptsTime        hlc.Timestamp
}

// wait waits for d or the end of ctx.
func wait(ctx context.Context, d time.Duration) error {
	var timer timeutil.Timer
	defer timer.Stop()
	timer.Reset(d)
	select {
	case <-timer.C:
		timer.Read = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// retry calls f until it succeeds, waiting an archive interval after each
// failure.
func (a *archiver) retry(ctx context.Context, what string, f func() error) error {
	for {
		err := f()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		a.warn(ctx, "%s failed, retrying: %v", what, err)
		if err := wait(ctx, a.cfg.ArchiveInterval); err != nil {
			return err
		}
	}
}

// warn logs a failure and shows it as the job's status.
func (a *archiver) warn(ctx context.Context, format string, args ...interface{}) {
	log.Ops.Warningf(ctx, "AUTO BACKUP: "+format, args...)
	a.status(ctx, format, args...)
}

// status sets the job's status.
func (a *archiver) status(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if err := a.job.NoTxn().RunningStatus(ctx, func(context.Context, jobspb.Details) (jobs.RunningStatus, error) {
		return jobs.RunningStatus(msg), nil
	}); err != nil && ctx.Err() == nil {
		log.Ops.Warningf(ctx, "AUTO BACKUP: updating the job status: %v", err)
	}
}

// protect moves the protected timestamp to ts, creating it if the job has
// none.
func (a *archiver) protect(ctx context.Context, ts hlc.Timestamp) error {
	if a.pts != (uuid.UUID{}) && a.ptsTime == ts {
		return nil
	}
	id := a.pts
	if err := a.execCfg.InternalDB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		pts := a.execCfg.ProtectedTimestampProvider.WithTxn(txn)
		if id != (uuid.UUID{}) {
			return pts.UpdateTimestamp(ctx, id, ts)
		}
		id = uuid.MakeV4()
		rec := jobsprotectedts.MakeRecord(id, int64(a.job.ID()), ts, nil,
			jobsprotectedts.Jobs, ptpb.MakeClusterTarget())
		if err := pts.Protect(ctx, rec); err != nil {
			return err
		}
		return a.job.WithTxn(txn).SetProgress(ctx, jobspb.AutoBackupProgress{
			ProtectedTimestampRecord: &id,
		})
	}); err != nil {
		return err
	}
	a.pts, a.ptsTime = id, ts
	return nil
}

// hasProtection reports whether the job's protected timestamp still
// exists, and loads it.
func (a *archiver) hasProtection(ctx context.Context) (bool, error) {
	p, ok := a.job.Progress().Details.(*jobspb.Progress_AutoBackup)
	if !ok || p.AutoBackup.ProtectedTimestampRecord == nil {
		return false, nil
	}
	id := *p.AutoBackup.ProtectedTimestampRecord
	var rec *ptpb.Record
	if err := a.execCfg.InternalDB.Txn(ctx, func(ctx context.Context, txn isql.Txn) (err error) {
		rec, err = a.execCfg.ProtectedTimestampProvider.WithTxn(txn).GetRecord(ctx, id)
		return err
	}); err != nil {
		if errors.Is(err, protectedts.ErrNotExists) {
			return false, nil
		}
		return false, err
	}
	a.pts, a.ptsTime = id, rec.Timestamp
	return true, nil
}

// fullResult is the outcome of a full backup that runs next to the archive.
type fullResult struct {
	m   Manifest
	err error
}

func (a *archiver) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.clusterID = a.execCfg.NodeInfo.LogicalClusterID().String()
	a.clusterVersion = a.execCfg.Settings.Version.ActiveVersion(ctx).String()
	schedule, err := a.cfg.Schedule()
	if err != nil {
		return jobs.MarkAsPermanentJobError(err)
	}
	if err := a.retry(ctx, "opening the repository", func() (err error) {
		a.store, err = OpenStore(ctx, RepositoryOf(a.cfg))
		if err != nil {
			return err
		}
		return claim(ctx, a.store, a.clusterID)
	}); err != nil {
		return err
	}
	var protected bool
	if err := a.retry(ctx, "reading the protected timestamp", func() (err error) {
		protected, err = a.hasProtection(ctx)
		return err
	}); err != nil {
		return err
	}

	// Find where the archive goes on.
	var fulls []Manifest
	var nums []int64
	var next int64 = 1
	var start hlc.Timestamp
	if err := a.retry(ctx, "reading the repository", func() (err error) {
		if fulls, _, err = fullBackups(ctx, a.store); err != nil {
			return err
		}
		if nums, err = segmentNumbers(ctx, a.store); err != nil {
			return err
		}
		next, start = 1, hlc.Timestamp{}
		if len(nums) > 0 {
			h, err := readSegment(ctx, a.store, nums[len(nums)-1], true /* headerOnly */, nil)
			if err != nil {
				return err
			}
			next, start = h.Number+1, h.End
		}
		if n := len(fulls); n > 0 && fulls[n-1].AfterSegment+1 >= next {
			next, start = fulls[n-1].AfterSegment+1, fulls[n-1].EndTime
		}
		return nil
	}); err != nil {
		return err
	}

	if !protected || len(fulls) == 0 {
		// History after the last segment may be gone. Protect from now
		// and start again with a full backup.
		now := a.execCfg.Clock.Now()
		if err := a.retry(ctx, "protecting history", func() error {
			return a.protect(ctx, now)
		}); err != nil {
			return err
		}
		var m Manifest
		if err := a.retry(ctx, "the first full backup", func() (err error) {
			a.status(ctx, "taking a full backup")
			end := a.execCfg.Clock.Now()
			m, err = takeFullBackup(ctx, a.execCfg.DB, a.store, a.clusterID, a.clusterVersion, end, next-1)
			return err
		}); err != nil {
			return err
		}
		fulls = append(fulls, m)
		start = m.EndTime
	}
	nextFull := schedule.Next(fulls[len(fulls)-1].EndTime.GoTime())

	fullDone := make(chan fullResult, 1)
	var running *hlc.Timestamp
	defer func() {
		if running != nil {
			cancel()
			<-fullDone
		}
	}()
	tempDir := a.execCfg.DistSQLSrv.ServerConfig.TempStoragePath
	for {
		protectAt := start
		if running != nil {
			protectAt = *running
		}
		if err := a.protect(ctx, protectAt); err != nil {
			log.Ops.Warningf(ctx, "AUTO BACKUP: moving the protected timestamp: %v", err)
		}
		a.status(ctx, "the archive ends at %s with segment %d; the next full backup is due at %s",
			FormatTime(start), next-1, nextFull.UTC().Format(time.RFC3339))

		var timer timeutil.Timer
		timer.Reset(a.cfg.ArchiveInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case res := <-fullDone:
			timer.Stop()
			running = nil
			if res.err != nil {
				a.warn(ctx, "full backup failed, retrying: %v", res.err)
				continue
			}
			nextFull = schedule.Next(res.m.EndTime.GoTime())
			var buf bytes.Buffer
			if err := Expire(ctx, a.store, a.cfg.KeepFull, &buf); err != nil {
				log.Ops.Warningf(ctx, "AUTO BACKUP: expiring old backups: %v", err)
			}
			if buf.Len() > 0 {
				log.Ops.Infof(ctx, "AUTO BACKUP: %s", buf.String())
			}
		case <-timer.C:
			timer.Read = true
			end := a.execCfg.Clock.Now()
			if err := makeSegment(ctx, a.execCfg.DB, a.store, tempDir, a.clusterID, next, start, end); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				a.warn(ctx, "segment %d failed, retrying: %v", next, err)
				continue
			}
			start, next = end, next+1
			if running == nil && !timeutil.Now().Before(nextFull) {
				at, after := start, next-1
				running = &at
				go func() {
					m, err := takeFullBackup(ctx, a.execCfg.DB, a.store, a.clusterID, a.clusterVersion, at, after)
					fullDone <- fullResult{m: m, err: err}
				}()
			}
		}
	}
}

func init() {
	jobs.RegisterConstructor(jobspb.TypeAutoBackup,
		func(job *jobs.Job, _ *cluster.Settings) jobs.Resumer {
			return &autoBackupResumer{job: job}
		},
		jobs.UsesTenantCostControl,
	)
}
