// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package physical

import (
	"bytes"
	"sort"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/opt"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/cat"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
)

// Distribution represents the physical distribution of data for a relational
// operator. It is used to describe where the data will be physically located
// during execution, to enable more accurate costing of each operator by taking
// latency and network bandwidth into account.
type Distribution struct {
	// Regions is the set of regions that make up this Distribution. They should
	// be sorted in lexicographical order.
	// TODO(rytaft): Consider abstracting this to a list of "neighborhoods" to
	// support more different types of localities.
	// TODO(rytaft): Consider mapping the region strings to integers and storing
	// this as a intsets.Fast.
	Regions []string
}

// Any is true if this Distribution allows any set of regions.
func (d Distribution) Any() bool {
	return len(d.Regions) == 0
}

func (d Distribution) String() string {
	var buf bytes.Buffer
	d.format(&buf)
	return buf.String()
}

func (d Distribution) format(buf *bytes.Buffer) {
	for i, r := range d.Regions {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(r)
	}
}

// Equals returns true if the two Distributions are identical.
func (d Distribution) Equals(rhs Distribution) bool {
	if len(d.Regions) != len(rhs.Regions) {
		return false
	}

	for i := range d.Regions {
		if d.Regions[i] != rhs.Regions[i] {
			return false
		}
	}
	return true
}

// Union unions the other distribution with the given distribution,
// removing duplicates. It assumes both distributions are sorted.
func (d Distribution) Union(rhs Distribution) Distribution {
	regions := make([]string, 0, len(d.Regions)+len(rhs.Regions))
	l, r := 0, 0
	for l < len(d.Regions) && r < len(rhs.Regions) {
		if d.Regions[l] == rhs.Regions[r] {
			regions = append(regions, d.Regions[l])
			l++
			r++
		} else if d.Regions[l] < rhs.Regions[r] {
			regions = append(regions, d.Regions[l])
			l++
		} else {
			regions = append(regions, rhs.Regions[r])
			r++
		}
	}
	if l < len(d.Regions) {
		regions = append(regions, d.Regions[l:]...)
	} else if r < len(rhs.Regions) {
		regions = append(regions, rhs.Regions[r:]...)
	}
	return Distribution{Regions: regions}
}

const regionKey = "region"

// FromLocality sets the Distribution with the region from the given locality
// (if any).
func (d *Distribution) FromLocality(locality roachpb.Locality) {
	if region, ok := locality.Find(regionKey); ok {
		d.Regions = []string{region}
	}
}

// FromIndexScan sets the Distribution that results from scanning the given
// index.
func (d *Distribution) FromIndexScan(tabMeta *opt.TableMeta, ord cat.IndexOrdinal) {
	index := tabMeta.Table.Index(ord)
	if index.Table().IsVirtualTable() {
		// Virtual tables do not have zone configurations.
		return
	}

	// Use the leaseholder region(s) from the index's zone configuration.
	regions := getRegionsFromZone(index.Zone())

	// Convert to a slice and sort regions.
	d.Regions = make([]string, 0, len(regions))
	for r := range regions {
		d.Regions = append(d.Regions, r)
	}
	sort.Strings(d.Regions)
}

// GetSingleRegion returns the single region name of the distribution,
// if there is exactly one.
func (d *Distribution) GetSingleRegion() (region string, ok bool) {
	if d == nil {
		return "", false
	}
	if len(d.Regions) == 1 {
		return d.Regions[0], true
	}
	return "", false
}

// IsLocal returns true if this distribution matches
// the gateway region of the connection.
func (d *Distribution) IsLocal(evalCtx *eval.Context) bool {
	if d == nil {
		return false
	}
	gatewayRegion, foundLocalRegion := evalCtx.Locality.Find("region")
	if foundLocalRegion {
		if distributionRegion, ok := d.GetSingleRegion(); ok {
			return distributionRegion == gatewayRegion
		}
	}
	return false
}

// getRegionsFromZone returns the regions of the given zone config, if any. It
// attempts to find the smallest set of regions likely to hold the leaseholder.
func getRegionsFromZone(zone cat.Zone) map[string]struct{} {
	// First find any regional replica constraints. If there is exactly one, we
	// can return early.
	regions := getReplicaRegionsFromZone(zone)
	if len(regions) == 1 {
		return regions
	}

	// Next check the voter replica constraints. Once again, if there is exactly
	// one regional constraint, we can return early.
	var voterRegions map[string]struct{}
	for i, n := 0, zone.VoterConstraintsCount(); i < n; i++ {
		replicaConstraint := zone.VoterConstraint(i)
		for j, m := 0, replicaConstraint.ConstraintCount(); j < m; j++ {
			constraint := replicaConstraint.Constraint(j)
			if region, ok := getRegionFromConstraint(constraint); ok {
				if voterRegions == nil {
					voterRegions = make(map[string]struct{})
				}
				voterRegions[region] = struct{}{}
			}
		}
	}
	if len(voterRegions) == 1 {
		return voterRegions
	}

	// Use the lease preferences as a tie breaker. We only really care about the
	// first one, since subsequent lease preferences only apply in edge cases.
	if zone.LeasePreferenceCount() > 0 {
		leasePref := zone.LeasePreference(0)
		for i, n := 0, leasePref.ConstraintCount(); i < n; i++ {
			constraint := leasePref.Constraint(i)
			if region, ok := getRegionFromConstraint(constraint); ok {
				return map[string]struct{}{region: {}}
			}
		}
	}

	if len(voterRegions) > 0 {
		return voterRegions
	}
	return regions
}

// getRegionFromConstraint returns the region and ok=true if the given
// constraint is a required region constraint. Otherwise, returns ok=false.
func getRegionFromConstraint(constraint cat.Constraint) (region string, ok bool) {
	if constraint.GetKey() != regionKey {
		// We only care about constraints on the region.
		return "", false /* ok */
	}
	if constraint.IsRequired() {
		// The region is required.
		return constraint.GetValue(), true /* ok */
	}
	// The region is prohibited.
	return "", false /* ok */
}

// getReplicaRegionsFromZone returns the replica regions of the given zone
// config, if any.
func getReplicaRegionsFromZone(zone cat.Zone) map[string]struct{} {
	var regions map[string]struct{}
	for i, n := 0, zone.ReplicaConstraintsCount(); i < n; i++ {
		replicaConstraint := zone.ReplicaConstraints(i)
		for j, m := 0, replicaConstraint.ConstraintCount(); j < m; j++ {
			constraint := replicaConstraint.Constraint(j)
			if region, ok := getRegionFromConstraint(constraint); ok {
				if regions == nil {
					regions = make(map[string]struct{})
				}
				regions[region] = struct{}{}
			}
		}
	}
	return regions
}
