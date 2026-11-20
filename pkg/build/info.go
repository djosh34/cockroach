// Copyright 2015 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package build

import (
	"bytes"
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/buildutil"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/version"
)

// TimeFormat is the reference format for build.Time. Make sure it stays in sync
// with the string passed to the linker in the root Makefile.
const TimeFormat = "2006/01/02 15:04:05"

var (
	// These variables are initialized via the linker -X flag when compiling
	// release binaries.
	utcTime          string // Build time in UTC (year/month/day hour:min:sec)
	rev              string // SHA-1 of this build (git rev-parse)
	buildTagOverride string
	cgoTargetTriple  string
	platform         = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
	// Distribution is always "OSS".
	Distribution      = "OSS"
	typ               string // Type of this build: <empty>, "development", or "release"
	channel           string
	envChannel        = envutil.EnvOrDefaultString("COCKROACH_CHANNEL", "unknown")
	enabledAssertions = buildutil.CrdbTestBuild
	//go:embed version.txt
	cockroachVersion string
	binaryVersion    = computeBinaryVersion(cockroachVersion, rev)
)

const (
	DefaultTelemetryChannel = "official-binary"
	FIPSTelemetryChannel    = "official-fips-binary"
)

// IsRelease returns true if the binary was produced by a "release" build.
func IsRelease() bool {
	return typ == "release"
}

// SeemsOfficial reports whether this binary is likely to have come from an
// official release channel.
func SeemsOfficial() bool {
	return channel == DefaultTelemetryChannel || channel == FIPSTelemetryChannel
}

func computeBinaryVersion(versionTxt, revision string) string {
	if buildTagOverride != "" {
		return buildTagOverride
	}
	txt := strings.TrimSuffix(versionTxt, "\n")
	v, err := version.Parse(txt)
	if err != nil {
		panic(fmt.Errorf("could not parse version.txt: %w", err))
	}
	if IsRelease() {
		return v.String()
	}
	if revision != "" {
		return fmt.Sprintf("%s-dev-%s", v.String(), revision)
	}
	return fmt.Sprintf("%s-dev", v.String())
}

// BinaryVersion returns the version prefix, patch number and metadata of the current build.
func BinaryVersion() string {
	return binaryVersion
}

func init() {
	// Allow tests to override the version.txt contents. Only test builds
	// read this variable, so a release shows only its own version.
	if !buildutil.CrdbTestBuild {
		return
	}
	if versionOverride := envutil.EnvOrDefaultString(
		"COCKROACH_TESTING_VERSION_OVERRIDE", ""); versionOverride != "" {
		TestingOverrideVersion(versionOverride)
	}
}

// Short returns a pretty printed build and version summary.
func (b Info) Short() string {
	plat := b.Platform
	if b.CgoTargetTriple != "" {
		plat = b.CgoTargetTriple
	}
	return fmt.Sprintf("patchroaddb %s (%s, built %s, %s)",
		b.Tag, plat, b.Time, b.GoVersion)
}

// Long returns a pretty printed build summary
func (b Info) Long() string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 2, 1, 2, ' ', 0)
	fmt.Fprintf(tw, "Build Tag:        %s\n", b.Tag)
	fmt.Fprintf(tw, "Build Time:       %s\n", b.Time)
	fmt.Fprintf(tw, "Platform:         %s", b.Platform)
	if b.CgoTargetTriple != "" {
		fmt.Fprintf(tw, " (%s)", b.CgoTargetTriple)
	}
	fmt.Fprintln(tw)
	fmt.Fprintf(tw, "Go Version:       %s\n", b.GoVersion)
	fmt.Fprintf(tw, "Build Commit ID:  %s\n", b.Revision)
	fmt.Fprintf(tw, "Build Type:       %s\n", b.Type)
	fmt.Fprintf(tw, "Enabled Assertions: %t", b.EnabledAssertions) // No final newline: cobra prints one for us.
	_ = tw.Flush()
	return buf.String()
}

// GoTime parses the utcTime string and returns a time.Time.
func (b Info) GoTime() time.Time {
	val, err := time.Parse(TimeFormat, b.Time)
	if err != nil {
		return time.Time{}
	}
	return val
}

// Timestamp parses the utcTime string and returns the number of seconds since epoch.
func (b Info) Timestamp() (int64, error) {
	val, err := time.Parse(TimeFormat, b.Time)
	if err != nil {
		return 0, err
	}
	return val.Unix(), nil
}

// GetInfo returns an Info struct populated with the build information.
func GetInfo() Info {
	ch := channel
	if ch == "" {
		ch = "unknown"
	}
	return Info{
		GoVersion:         runtime.Version(),
		Tag:               binaryVersion,
		Time:              utcTime,
		Revision:          rev,
		CgoTargetTriple:   cgoTargetTriple,
		Platform:          platform,
		Distribution:      Distribution,
		Type:              typ,
		Channel:           ch,
		EnvChannel:        envChannel,
		EnabledAssertions: enabledAssertions,
	}
}

// TestingOverrideVersion allows tests to override the binary version
// reported by patchroaddb.
func TestingOverrideVersion(v string) func() {
	prevBinaryVersion := binaryVersion
	binaryVersion = v
	return func() { binaryVersion = prevBinaryVersion }
}

// MakeIssueURL produces a URL to a CockroachDB issue. patchroaddb is a fork of
// CockroachDB v23.2, so the link names that version.
func MakeIssueURL(issue int) string {
	return fmt.Sprintf("https://go.crdb.dev/issue-v/%d/v23.2", issue)
}
