// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

/*
Package telemetry contains helpers for capturing diagnostics information. The
counts stay on the node: they appear in the local diagnostics report and in
crdb_internal.feature_usage, and are never sent anywhere.

When trying to measure the usage of a given feature, the existing reporting of
"scrubbed" queries -- showing the structure but not the values -- can serve as a
means to measure eg. how many clusters use BACKUP or window functions, etc.
However many features cannot be easily measured just from these statements,
either because they are cannot be reliably inferred from a scrubbed query or
are simply not involved in a SQL statement execution at all.

For such features we also have light-weight `telemetry.Count("some.feature")` to
track their usage. These increment in-memory counts that are then included in
the local diagnostics report. Some notes on using these:
  - "some.feature" should always be a literal string constant -- it must not
    include any user-submitted data.
  - Contention-sensitive, high-volume callers should use an initial `GetCounter`
    to get a Counter they can then `Inc` repeatedly instead to avoid contention
    and map lookup over around the name resolution on each increment.
  - When naming a counter, by convention we use dot-separated, dashed names, eg.
    `feature-area.specific-feature`.
*/
package telemetry
