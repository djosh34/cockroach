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
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// Strftime formats t according to format, using the conversion
// specifications of POSIX strftime in the C locale, plus %f for
// microseconds and %k, %l and %s from common C libraries:
//
//	%a, %A   abbreviated and full weekday name
//	%b, %h   abbreviated month name; %B full month name
//	%c       date and time, as "Wed Oct 5 01:02:03 2016"
//	%C       century, two digits
//	%d, %e   day of the month, zero- and space-padded
//	%D, %x   date as %m/%d/%y
//	%f       microseconds, six digits
//	%F       date as %Y-%m-%d
//	%H, %k   hour (00-23), zero- and space-padded
//	%I, %l   hour (01-12), zero- and space-padded
//	%j       day of the year, three digits
//	%m       month, two digits
//	%M       minute, two digits
//	%n, %t   newline and tab
//	%p       AM or PM
//	%r       time as %I:%M:%S %p
//	%R       time as %H:%M
//	%s       seconds since the Unix epoch
//	%S       second, two digits
//	%T, %X   time as %H:%M:%S
//	%u       weekday, 1 (Monday) to 7
//	%U       week of the year, weeks starting on Sunday (00-53)
//	%w       weekday, 0 (Sunday) to 6
//	%W       week of the year, weeks starting on Monday (00-53)
//	%y       year, two digits
//	%Y       year, at least two digits
//	%z       UTC offset as +hhmm
//	%Z       time zone abbreviation
//	%%       a literal %
//
// Any other conversion specification is an error, and a % at the end of
// format is dropped. These details match the strftime of
// github.com/knz/strtime, which this function replaces.
func Strftime(t time.Time, format string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i == len(format) {
			break
		}
		if err := strftimeConversion(&b, t, format[i]); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// strftimeConversion writes the result of the conversion specification %c of
// Strftime for t to b.
func strftimeConversion(b *strings.Builder, t time.Time, c byte) error {
	switch c {
	case 'a':
		b.WriteString(t.Weekday().String()[:3])
	case 'A':
		b.WriteString(t.Weekday().String())
	case 'b', 'h':
		b.WriteString(t.Month().String()[:3])
	case 'B':
		b.WriteString(t.Month().String())
	case 'c':
		b.WriteString(t.Format("Mon Jan 2 15:04:05 2006"))
	case 'C':
		pad(b, t.Year()/100, 2, '0')
	case 'd':
		pad(b, t.Day(), 2, '0')
	case 'D', 'x':
		pad(b, int(t.Month()), 2, '0')
		b.WriteByte('/')
		pad(b, t.Day(), 2, '0')
		b.WriteByte('/')
		pad(b, t.Year()%100, 2, '0')
	case 'e':
		pad(b, t.Day(), 2, ' ')
	case 'f':
		pad(b, t.Nanosecond()/1000, 6, '0')
	case 'F':
		pad(b, t.Year(), 4, '0')
		b.WriteByte('-')
		pad(b, int(t.Month()), 2, '0')
		b.WriteByte('-')
		pad(b, t.Day(), 2, '0')
	case 'H':
		pad(b, t.Hour(), 2, '0')
	case 'k':
		pad(b, t.Hour(), 2, ' ')
	case 'I':
		pad(b, hour12(t), 2, '0')
	case 'l':
		pad(b, hour12(t), 2, ' ')
	case 'j':
		pad(b, t.YearDay(), 3, '0')
	case 'm':
		pad(b, int(t.Month()), 2, '0')
	case 'M':
		pad(b, t.Minute(), 2, '0')
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'p':
		if t.Hour() < 12 {
			b.WriteString("AM")
		} else {
			b.WriteString("PM")
		}
	case 'r':
		pad(b, hour12(t), 2, '0')
		b.WriteByte(':')
		pad(b, t.Minute(), 2, '0')
		b.WriteByte(':')
		pad(b, t.Second(), 2, '0')
		if t.Hour() < 12 {
			b.WriteString(" AM")
		} else {
			b.WriteString(" PM")
		}
	case 'R':
		pad(b, t.Hour(), 2, '0')
		b.WriteByte(':')
		pad(b, t.Minute(), 2, '0')
	case 's':
		b.WriteString(strconv.FormatInt(t.Unix(), 10))
	case 'S':
		pad(b, t.Second(), 2, '0')
	case 'T', 'X':
		pad(b, t.Hour(), 2, '0')
		b.WriteByte(':')
		pad(b, t.Minute(), 2, '0')
		b.WriteByte(':')
		pad(b, t.Second(), 2, '0')
	case 'u':
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		b.WriteString(strconv.Itoa(wd))
	case 'U':
		pad(b, (t.YearDay()-1+7-int(t.Weekday()))/7, 2, '0')
	case 'w':
		b.WriteString(strconv.Itoa(int(t.Weekday())))
	case 'W':
		pad(b, (t.YearDay()-1+7-(int(t.Weekday())+6)%7)/7, 2, '0')
	case 'y':
		pad(b, t.Year()%100, 2, '0')
	case 'Y':
		pad(b, t.Year(), 2, '0')
	case 'z':
		_, offset := t.Zone()
		sign := byte('+')
		if offset < 0 {
			sign = '-'
			offset = -offset
		}
		b.WriteByte(sign)
		pad(b, offset/3600, 2, '0')
		pad(b, offset%3600/60, 2, '0')
	case 'Z':
		name, _ := t.Zone()
		b.WriteString(name)
	case '%':
		b.WriteByte('%')
	default:
		return errors.Newf("invalid format code: %c", c)
	}
	return nil
}

// hour12 returns the hour of t on a 12-hour clock, from 1 to 12.
func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}

// pad writes n to b, padded on the left with c to at least width
// characters. A negative n keeps its sign in front of the padding.
func pad(b *strings.Builder, n int, width int, c byte) {
	if n < 0 {
		b.WriteByte('-')
		n = -n
		width--
	}
	s := strconv.Itoa(n)
	for i := len(s); i < width; i++ {
		b.WriteByte(c)
	}
	b.WriteString(s)
}
