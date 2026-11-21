// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package backup backs a cluster up to S3 and restores it, like pgBackRest.
//
// A repository holds full backups and a continuous archive:
//
//	repository.json                the ID of the cluster that owns it
//	full/<time>/000001.sst.gz      the files of a full backup
//	full/<time>/backup.manifest    written last: the backup exists once it does
//	archive/0000000000000001.tar.gz one archive segment per object
//
// A full backup holds the latest value of every key at its end time. Each
// archive segment holds every MVCC revision in (start, end] and is one
// object, so it exists only once its upload has finished. Segments are
// numbered 1, 2, 3, ...; a segment starts where the one before it ended,
// except right after a full backup that the AUTO BACKUP job took because it
// had lost its protected timestamp: then the segment starts at that full
// backup's end time.
//
// Restore takes the last full backup at or before the target and every
// segment after it, in order, up to the target.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/errors"
)

const (
	ownerName    = "repository.json"
	fullDir      = "full"
	archiveDir   = "archive"
	manifestName = "backup.manifest"
	segmentExt   = ".tar.gz"
	segmentEntry = "segment.json"
)

// FileInfo describes one SST file of a backup or segment.
type FileInfo struct {
	Name string `json:"name"`
	// SHA256 is the hex SHA-256 of the SST, before compression.
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest describes a complete full backup.
type Manifest struct {
	// Name is the folder of the backup under full/. It is not stored.
	Name      string `json:"-"`
	ClusterID string `json:"cluster_id"`
	// ClusterVersion is the cluster version of the source; a restore needs
	// the same one.
	ClusterVersion string        `json:"cluster_version"`
	EndTime        hlc.Timestamp `json:"end_time"`
	// AfterSegment is the number of the last segment before this backup.
	// The archive continues with segment AfterSegment+1, which starts at
	// EndTime.
	AfterSegment int64      `json:"after_segment"`
	Files        []FileInfo `json:"files"`
}

// SegmentHeader is the first entry of a segment. The SST files follow it,
// in the order of Files.
type SegmentHeader struct {
	ClusterID string        `json:"cluster_id"`
	Number    int64         `json:"number"`
	Start     hlc.Timestamp `json:"start"`
	End       hlc.Timestamp `json:"end"`
	Files     []FileInfo    `json:"files"`
}

type owner struct {
	ClusterID string `json:"cluster_id"`
}

func fullName(end hlc.Timestamp) string {
	return end.GoTime().UTC().Format("20060102T150405.000000000Z")
}

func fullFile(name string, i int) string {
	return join(fullDir, name, fmt.Sprintf("%06d.sst.gz", i))
}

func segmentName(n int64) string {
	return join(archiveDir, fmt.Sprintf("%016d%s", n, segmentExt))
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// FormatTime formats an HLC timestamp for people: the UTC time and the
// exact decimal form that restore takes.
func FormatTime(ts hlc.Timestamp) string {
	return fmt.Sprintf("%s (%s)", ts.GoTime().UTC().Format(time.RFC3339Nano), ts.AsOfSystemTime())
}

// claim records clusterID as the owner of the repository if it has none,
// and fails if another cluster owns it.
func claim(ctx context.Context, s *Store, clusterID string) error {
	o, err := readOwner(ctx, s)
	if errors.Is(err, ErrNotFound) {
		b, merr := json.Marshal(owner{ClusterID: clusterID})
		if merr != nil {
			return merr
		}
		if perr := s.Put(ctx, ownerName, strings.NewReader(string(b))); perr != nil {
			return perr
		}
		o, err = readOwner(ctx, s)
	}
	if err != nil {
		return err
	}
	if o != clusterID {
		return errors.Newf("the repository belongs to cluster %s; this cluster (%s) does not write into it",
			o, clusterID)
	}
	return nil
}

func readOwner(ctx context.Context, s *Store) (string, error) {
	var o owner
	if err := getJSON(ctx, s, ownerName, &o); err != nil {
		return "", err
	}
	return o.ClusterID, nil
}

func getJSON(ctx context.Context, s *Store, name string, v interface{}) error {
	r, err := s.Get(ctx, name)
	if err != nil {
		return err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return errors.Wrapf(err, "reading %s", name)
	}
	return errors.Wrapf(json.Unmarshal(b, v), "parsing %s", name)
}

// fullBackups returns the complete full backups, oldest first, and the
// folders of full backups that have no manifest.
func fullBackups(
	ctx context.Context, s *Store,
) (complete []Manifest, incomplete []string, _ error) {
	names, err := s.List(ctx, fullDir)
	if err != nil {
		return nil, nil, err
	}
	hasManifest := map[string]bool{}
	var dirs []string
	for _, n := range names {
		dir, file, ok := strings.Cut(n, "/")
		if !ok {
			continue
		}
		if _, seen := hasManifest[dir]; !seen {
			dirs = append(dirs, dir)
			hasManifest[dir] = false
		}
		if file == manifestName {
			hasManifest[dir] = true
		}
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		if !hasManifest[dir] {
			incomplete = append(incomplete, dir)
			continue
		}
		var m Manifest
		if err := getJSON(ctx, s, join(fullDir, dir, manifestName), &m); err != nil {
			return nil, nil, err
		}
		m.Name = dir
		complete = append(complete, m)
	}
	return complete, incomplete, nil
}

// segmentNumbers returns the numbers of the segments in the repository, in
// ascending order.
func segmentNumbers(ctx context.Context, s *Store) ([]int64, error) {
	names, err := s.List(ctx, archiveDir)
	if err != nil {
		return nil, err
	}
	var nums []int64
	for _, name := range names {
		n, ok := strings.CutSuffix(name, segmentExt)
		if !ok {
			continue
		}
		num, err := strconv.ParseInt(n, 10, 64)
		if err != nil || num <= 0 {
			continue
		}
		nums = append(nums, num)
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
	return nums, nil
}

// writeSegment uploads a segment whose SST files are read, in order, by
// open.
func writeSegment(
	ctx context.Context, s *Store, h SegmentHeader, open func(i int) (io.ReadCloser, error),
) error {
	hb, err := json.Marshal(h)
	if err != nil {
		return err
	}
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(func() error {
			gz := gzip.NewWriter(pw)
			tw := tar.NewWriter(gz)
			add := func(name string, size int64, r io.Reader) error {
				if err := tw.WriteHeader(&tar.Header{
					Name: name, Size: size, Mode: 0644, Typeflag: tar.TypeReg,
				}); err != nil {
					return err
				}
				_, err := io.Copy(tw, r)
				return err
			}
			if err := add(segmentEntry, int64(len(hb)), strings.NewReader(string(hb))); err != nil {
				return err
			}
			for i, f := range h.Files {
				r, err := open(i)
				if err != nil {
					return err
				}
				err = add(f.Name, f.Size, r)
				if cerr := r.Close(); err == nil {
					err = cerr
				}
				if err != nil {
					return err
				}
			}
			if err := tw.Close(); err != nil {
				return err
			}
			return gz.Close()
		}())
	}()
	err = s.Put(ctx, segmentName(h.Number), pr)
	// Stop the writer if the upload ended early.
	_ = pr.CloseWithError(errors.New("upload ended"))
	return err
}

// readSegment reads segment n. It calls file for each SST, in order, after
// checking its checksum, unless headerOnly is set. The gzip CRC of the
// whole object is checked too.
func readSegment(
	ctx context.Context, s *Store, n int64, headerOnly bool, file func(f FileInfo, sst []byte) error,
) (SegmentHeader, error) {
	var h SegmentHeader
	name := segmentName(n)
	r, err := s.Get(ctx, name)
	if err != nil {
		return h, err
	}
	defer r.Close()
	gz, err := gzip.NewReader(r)
	if err != nil {
		return h, errors.Wrapf(err, "reading %s", name)
	}
	tr := tar.NewReader(gz)
	next := func(want string) ([]byte, error) {
		th, err := tr.Next()
		if err != nil {
			return nil, errors.Wrapf(err, "reading %s", name)
		}
		if th.Name != want {
			return nil, errors.Newf("%s: found entry %q, expected %q", name, th.Name, want)
		}
		b, err := io.ReadAll(tr)
		return b, errors.Wrapf(err, "reading %s", name)
	}
	hb, err := next(segmentEntry)
	if err != nil {
		return h, err
	}
	if err := json.Unmarshal(hb, &h); err != nil {
		return h, errors.Wrapf(err, "parsing the header of %s", name)
	}
	if h.Number != n {
		return h, errors.Newf("%s holds segment %d", name, h.Number)
	}
	if headerOnly {
		return h, nil
	}
	for _, f := range h.Files {
		b, err := next(f.Name)
		if err != nil {
			return h, err
		}
		if sha(b) != f.SHA256 {
			return h, errors.Newf("%s: checksum mismatch in %s", name, f.Name)
		}
		if err := file(f, b); err != nil {
			return h, err
		}
	}
	if _, err := tr.Next(); err != io.EOF {
		return h, errors.Newf("%s: unexpected data after the last file", name)
	}
	// Read to the end so that gzip checks its CRC.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return h, errors.Wrapf(err, "reading %s", name)
	}
	return h, nil
}

// readFullFile reads file f of full backup m and checks its checksum.
func readFullFile(ctx context.Context, s *Store, m Manifest, f FileInfo) ([]byte, error) {
	name := join(fullDir, m.Name, f.Name)
	r, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", name)
	}
	b, err := io.ReadAll(gz)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", name)
	}
	if sha(b) != f.SHA256 {
		return nil, errors.Newf("checksum mismatch in %s", name)
	}
	return b, nil
}

// chain is a full backup and the continuous segments after it.
type chain struct {
	full     Manifest
	segments []SegmentHeader
}

// end returns the last time the chain can restore to.
func (c chain) end() hlc.Timestamp {
	if len(c.segments) == 0 {
		return c.full.EndTime
	}
	return c.segments[len(c.segments)-1].End
}

// findChain returns the chain that restores to target: the last full
// backup at or before target and the segments after it up to the first
// one that ends at or after target. An empty target means the latest
// restorable time: the last full backup and all continuous segments after
// it. It stops at the first missing segment number or the first segment
// that does not start where the one before ended.
func findChain(ctx context.Context, s *Store, target hlc.Timestamp) (chain, error) {
	fulls, _, err := fullBackups(ctx, s)
	if err != nil {
		return chain{}, err
	}
	var c chain
	found := false
	for _, m := range fulls {
		if target.IsEmpty() || m.EndTime.LessEq(target) {
			c.full, found = m, true
		}
	}
	if !found {
		if len(fulls) == 0 {
			return chain{}, errors.New("the repository has no full backup")
		}
		return chain{}, errors.Newf("no full backup ends at or before %s; the first ends at %s",
			FormatTime(target), FormatTime(fulls[0].EndTime))
	}
	nums, err := segmentNumbers(ctx, s)
	if err != nil {
		return chain{}, err
	}
	have := make(map[int64]bool, len(nums))
	for _, n := range nums {
		have[n] = true
	}
	prevEnd := c.full.EndTime
	for n := c.full.AfterSegment + 1; have[n]; n++ {
		if !target.IsEmpty() && target.LessEq(prevEnd) {
			break
		}
		h, err := readSegment(ctx, s, n, true /* headerOnly */, nil)
		if err != nil {
			return chain{}, err
		}
		if h.Start != prevEnd {
			break
		}
		c.segments = append(c.segments, h)
		prevEnd = h.End
	}
	if !target.IsEmpty() && prevEnd.Less(target) {
		return chain{}, errors.Newf("%s is after the last restorable time, %s",
			FormatTime(target), FormatTime(prevEnd))
	}
	return c, nil
}
