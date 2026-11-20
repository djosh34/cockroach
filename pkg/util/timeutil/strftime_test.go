// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStrftime(t *testing.T) {
	// Wednesday, 2016-10-05 01:02:03.123456 UTC, day 279 of the year.
	wed := time.Date(2016, 10, 5, 1, 2, 3, 123456789, time.UTC)
	evening := time.Date(2006, 10, 12, 23, 2, 3, 0, time.UTC)
	zone := time.FixedZone("ZZZ", -(6*3600 + 55*60))

	for _, tc := range []struct {
		t        time.Time
		format   string
		expected string
	}{
		{wed, `%a %b %d %Y`, `Wed Oct 05 2016`},
		{wed, `%A %B %d %Y`, `Wednesday October 05 2016`},
		{wed, `%c`, `Wed Oct 5 01:02:03 2016`},
		{wed, `%C %y %m %d`, `20 16 10 05`},
		{wed, `%D`, `10/05/16`},
		{wed, `%x`, `10/05/16`},
		{wed, `%Y %m %e`, `2016 10  5`},
		{wed, `.%f`, `.123456`},
		{wed, `%F`, `2016-10-05`},
		{wed, `%Y %h %d`, `2016 Oct 05`},
		{wed, `%H:%M:%S`, `01:02:03`},
		{wed, `%k|%l|%I`, ` 1| 1|01`},
		{evening, `%k %l %I %p`, `23 11 11 PM`},
		{wed, `%p`, `AM`},
		{wed, `%j`, `279`},
		{wed, `%Y%n%m%t%d`, "2016\n10\t05"},
		{evening, `%r`, `11:02:03 PM`},
		{evening, `%R`, `23:02`},
		{evening, `%T %X`, `23:02:03 23:02:03`},
		{time.Unix(1491920586, 0).UTC(), `%s`, `1491920586`},
		{time.Date(2018, 3, 15, 0, 0, 0, 0, time.UTC), `%Y %U %u %w`, `2018 10 4 4`},
		{time.Date(2018, 3, 8, 0, 0, 0, 0, time.UTC), `%Y %W %w`, `2018 10 4`},
		{time.Date(2018, 3, 11, 0, 0, 0, 0, time.UTC), `%u %w %U %W`, `7 0 10 10`},
		{time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), `%G %g %V`, `2020 20 53`},
		{time.Date(2068, 1, 1, 0, 0, 0, 0, time.UTC), `%y%m%d`, `680101`},
		{time.Date(900, 1, 1, 0, 0, 0, 0, time.UTC), `%Y`, `0900`},
		{time.Date(2016, 1, 1, 6, 5, 0, 0, time.UTC), `%Y%m%d %H:%M %z`, `20160101 06:05 +0000`},
		{time.Date(2016, 1, 1, 6, 5, 0, 0, zone), `%z %Z`, `-0655 ZZZ`},
		{wed, `100%%`, `100%`},
	} {
		t.Run(tc.format, func(t *testing.T) {
			s, err := Strftime(tc.t, tc.format)
			require.NoError(t, err)
			require.Equal(t, tc.expected, s)
		})
	}

	for _, format := range []string{`%Q`, `%Ey`, `abc%`} {
		_, err := Strftime(wed, format)
		require.Error(t, err, format)
	}
}
