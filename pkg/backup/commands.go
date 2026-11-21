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
	"fmt"
	"io"
	"path"

	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/errors"
)

// The functions in this file are the `patchroaddb backup` commands. They
// work on the repository alone, with no cluster.

// Info writes what the repository holds: its owner, the full backups, the
// archive and the last restorable time.
func Info(ctx context.Context, s *Store, w io.Writer) error {
	o, err := readOwner(ctx, s)
	if errors.Is(err, ErrNotFound) {
		fmt.Fprintln(w, "The repository is empty.")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "owner cluster: %s\n", o)
	fulls, incomplete, err := fullBackups(ctx, s)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "full backups:")
	for _, m := range fulls {
		fmt.Fprintf(w, "  %s  end %s, %d files, archive from segment %d\n",
			m.Name, FormatTime(m.EndTime), len(m.Files), m.AfterSegment+1)
	}
	for _, name := range incomplete {
		fmt.Fprintf(w, "  %s  incomplete (no manifest)\n", name)
	}
	nums, err := segmentNumbers(ctx, s)
	if err != nil {
		return err
	}
	if len(nums) == 0 {
		fmt.Fprintln(w, "archive: no segments")
	} else {
		last, err := readSegment(ctx, s, nums[len(nums)-1], true /* headerOnly */, nil)
		if err != nil {
			return err
		}
		missing := nums[len(nums)-1] - nums[0] + 1 - int64(len(nums))
		fmt.Fprintf(w, "archive: segments %d to %d, %d missing; the last ends at %s\n",
			nums[0], nums[len(nums)-1], missing, FormatTime(last.End))
	}
	if len(fulls) == 0 {
		fmt.Fprintln(w, "last restorable time: none, there is no full backup")
		return nil
	}
	c, err := findChain(ctx, s, hlc.Timestamp{})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "last restorable time: %s\n", FormatTime(c.end()))
	return nil
}

// Check reads the full backup named name, or the last one if name is
// empty, and every segment of its chain, and verifies their checksums and
// that the segments follow each other. The chain may end only at the last
// segment, or where a later full backup starts the archive again; any
// other gap is an error.
func Check(ctx context.Context, s *Store, name string, w io.Writer) error {
	fulls, _, err := fullBackups(ctx, s)
	if err != nil {
		return err
	}
	if len(fulls) == 0 {
		return errors.New("the repository has no full backup")
	}
	m := fulls[len(fulls)-1]
	if name != "" {
		found := false
		for _, f := range fulls {
			if f.Name == name {
				m, found = f, true
			}
		}
		if !found {
			return errors.Newf("no complete full backup named %q", name)
		}
	}
	for _, f := range m.Files {
		if _, err := readFullFile(ctx, s, m, f); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "full backup %s: %d files OK\n", m.Name, len(m.Files))
	nums, err := segmentNumbers(ctx, s)
	if err != nil {
		return err
	}
	have := make(map[int64]bool, len(nums))
	for _, n := range nums {
		have[n] = true
	}
	// restart reports whether a full backup starts the archive again at
	// segment n, at time start, after a gap that follows prevEnd.
	restart := func(n int64, start, prevEnd hlc.Timestamp) bool {
		for _, f := range fulls {
			if f.AfterSegment == n-1 && f.EndTime == start && prevEnd.Less(start) {
				return true
			}
		}
		return false
	}
	prevEnd := m.EndTime
	n := m.AfterSegment + 1
	for ; have[n]; n++ {
		h, err := readSegment(ctx, s, n, false, func(FileInfo, []byte) error { return nil })
		if err != nil {
			return err
		}
		if h.Start != prevEnd {
			if !restart(n, h.Start, prevEnd) {
				return errors.Newf("segment %d starts at %s, but segment %d ended at %s",
					n, FormatTime(h.Start), n-1, FormatTime(prevEnd))
			}
			break
		}
		prevEnd = h.End
	}
	if !have[n] && len(nums) > 0 && n < nums[len(nums)-1] {
		return errors.Newf("segment %d is missing; segments up to %d exist", n, nums[len(nums)-1])
	}
	if n == m.AfterSegment+1 {
		fmt.Fprintf(w, "no segments after it; restorable up to %s\n", FormatTime(prevEnd))
	} else {
		fmt.Fprintf(w, "segments %d to %d OK; restorable up to %s\n", m.AfterSegment+1, n-1, FormatTime(prevEnd))
	}
	return nil
}

// Expire deletes the full backups older than the keep newest ones, the
// segments that only those needed, and the files of full backups that
// never finished and are older than the newest complete one. The manifest
// of a full backup is deleted first, so a backup stops existing before its
// files go. Expire finishes the work of one that failed half way.
func Expire(ctx context.Context, s *Store, keep int, w io.Writer) error {
	if keep < 1 {
		return errors.New("keep at least one full backup")
	}
	fulls, incomplete, err := fullBackups(ctx, s)
	if err != nil {
		return err
	}
	if len(fulls) == 0 {
		return nil
	}
	deleteDir := func(name string, manifestFirst bool) error {
		files, err := s.List(ctx, path.Join(fullDir, name))
		if err != nil {
			return err
		}
		if manifestFirst {
			if err := s.Delete(ctx, path.Join(fullDir, name, manifestName)); err != nil {
				return err
			}
		}
		for _, f := range files {
			if f == manifestName && manifestFirst {
				continue
			}
			if err := s.Delete(ctx, path.Join(fullDir, name, f)); err != nil {
				return err
			}
		}
		return nil
	}
	expired := 0
	if len(fulls) > keep {
		expired = len(fulls) - keep
	}
	for _, m := range fulls[:expired] {
		if err := deleteDir(m.Name, true); err != nil {
			return err
		}
		fmt.Fprintf(w, "deleted full backup %s\n", m.Name)
	}
	newest := fulls[len(fulls)-1].Name
	for _, name := range incomplete {
		if name < newest {
			if err := deleteDir(name, false); err != nil {
				return err
			}
			fmt.Fprintf(w, "deleted incomplete full backup %s\n", name)
		}
	}
	// No full backup left needs the segments up to the oldest one.
	oldestKept := fulls[expired]
	nums, err := segmentNumbers(ctx, s)
	if err != nil {
		return err
	}
	deleted := 0
	for _, n := range nums {
		if n > oldestKept.AfterSegment {
			break
		}
		if err := s.Delete(ctx, segmentName(n)); err != nil {
			return err
		}
		deleted++
	}
	if deleted > 0 {
		fmt.Fprintf(w, "deleted %d segments up to %d\n", deleted, oldestKept.AfterSegment)
	}
	return nil
}

// CheckTarget returns an error unless the repository can restore to
// target; an empty target means the last restorable time.
func CheckTarget(ctx context.Context, s *Store, target hlc.Timestamp) (hlc.Timestamp, error) {
	c, err := findChain(ctx, s, target)
	if err != nil {
		return hlc.Timestamp{}, err
	}
	if target.IsEmpty() {
		return c.end(), nil
	}
	return target, nil
}
