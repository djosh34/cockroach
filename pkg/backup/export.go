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
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/admission/admissionpb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

// clusterSpan is what a backup covers: all of the system tenant's SQL
// data, system tables included.
var clusterSpan = roachpb.Span{Key: keys.TableDataMin, EndKey: keys.TableDataMax}

// exportTargetBytes is about how much SST data one export request returns.
const exportTargetBytes = 32 << 20

// export reads span at end and calls emit with each SST file it gets, in
// key order. With allRevisions it reads every MVCC revision in (start,
// end], tombstones included; otherwise the latest value of each key at end.
// Tables marked exclude_data_from_backup return no data.
func export(
	ctx context.Context,
	db *kv.DB,
	span roachpb.Span,
	start, end hlc.Timestamp,
	allRevisions bool,
	emit func(sst []byte) error,
) error {
	filter := kvpb.MVCCFilter_Latest
	if allRevisions {
		filter = kvpb.MVCCFilter_All
	}
	var resumeTS hlc.Timestamp
	for len(span.Key) != 0 {
		req := &kvpb.ExportRequest{
			RequestHeader: kvpb.RequestHeader{Key: span.Key, EndKey: span.EndKey},
			StartTime:     start,
			MVCCFilter:    filter,
			SplitMidKey:   true,
			ResumeKeyTS:   resumeTS,
		}
		header := kvpb.Header{
			Timestamp:                   end,
			TargetBytes:                 exportTargetBytes,
			ReturnElasticCPUResumeSpans: true,
		}
		admission := kvpb.AdmissionHeader{
			Priority:                 int32(admissionpb.BulkNormalPri),
			CreateTime:               timeutil.Now().UnixNano(),
			Source:                   kvpb.AdmissionHeader_FROM_SQL,
			NoMemoryReservedAtSource: true,
		}
		raw, pErr := kv.SendWrappedWithAdmission(ctx, db.NonTransactionalSender(), header, admission, req)
		if pErr != nil {
			return errors.Wrapf(pErr.GoError(), "exporting %s", span)
		}
		resp := raw.(*kvpb.ExportResponse)
		for _, f := range resp.Files {
			if len(f.SST) == 0 {
				continue
			}
			if err := emit(f.SST); err != nil {
				return err
			}
		}
		if resp.ResumeSpan == nil {
			return nil
		}
		if !resp.ResumeSpan.Valid() {
			return errors.Errorf("invalid resume span %s", resp.ResumeSpan)
		}
		if n := len(resp.Files); n > 0 {
			resumeTS = resp.Files[n-1].EndKeyTS
		}
		span = *resp.ResumeSpan
	}
	return nil
}

// takeFullBackup writes a full backup of the cluster at end. The manifest
// is written last.
func takeFullBackup(
	ctx context.Context,
	db *kv.DB,
	s *Store,
	clusterID, clusterVersion string,
	end hlc.Timestamp,
	afterSegment int64,
) (Manifest, error) {
	m := Manifest{
		Name:           fullName(end),
		ClusterID:      clusterID,
		ClusterVersion: clusterVersion,
		EndTime:        end,
		AfterSegment:   afterSegment,
	}
	if err := export(ctx, db, clusterSpan, hlc.Timestamp{}, end, false, func(sst []byte) error {
		f := FileInfo{Name: filepath.Base(fullFile(m.Name, len(m.Files)+1)), SHA256: sha(sst), Size: int64(len(sst))}
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(sst); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		if err := s.Put(ctx, fullFile(m.Name, len(m.Files)+1), &buf); err != nil {
			return err
		}
		m.Files = append(m.Files, f)
		return nil
	}); err != nil {
		return Manifest{}, err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := s.Put(ctx, join(fullDir, m.Name, manifestName), bytes.NewReader(b)); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// makeSegment exports every revision in (start, end] into segment number
// n. The SSTs wait in a local folder under tempDir until the upload.
func makeSegment(
	ctx context.Context,
	db *kv.DB,
	s *Store,
	tempDir string,
	clusterID string,
	n int64,
	start, end hlc.Timestamp,
) error {
	dir, err := os.MkdirTemp(tempDir, "backup-segment-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	h := SegmentHeader{ClusterID: clusterID, Number: n, Start: start, End: end}
	path := func(i int) string { return filepath.Join(dir, fmt.Sprintf("%06d.sst", i+1)) }
	if err := export(ctx, db, clusterSpan, start, end, true, func(sst []byte) error {
		i := len(h.Files)
		if err := os.WriteFile(path(i), sst, 0600); err != nil {
			return err
		}
		h.Files = append(h.Files, FileInfo{
			Name: filepath.Base(path(i)), SHA256: sha(sst), Size: int64(len(sst)),
		})
		return nil
	}); err != nil {
		return err
	}
	return writeSegment(ctx, s, h, func(i int) (io.ReadCloser, error) {
		return os.Open(path(i))
	})
}
