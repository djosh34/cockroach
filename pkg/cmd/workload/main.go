// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package main

import (
	"os"

	_ "github.com/cockroachdb/cockroach/pkg/workload/bank"       // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/bulkingest" // registers workloads
	workloadcli "github.com/cockroachdb/cockroach/pkg/workload/cli"
	_ "github.com/cockroachdb/cockroach/pkg/workload/connectionlatency" // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/debug"             // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/examples"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/geospatial"        // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/indexes"           // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/insights"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/jsonload"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/kv"                // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/ledger"            // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/movr"              // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/querybench"        // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/querylog"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/queue"             // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/rand"              // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/schemachange"      // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/sqlsmith"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/tpcc"              // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/tpccchecks"        // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/tpcds"             // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/tpch"              // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/ttlbench"          // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/ttllogger"         // registers workloads
	_ "github.com/cockroachdb/cockroach/pkg/workload/ycsb"              // registers workloads
)

func main() {
	if err := workloadcli.WorkloadCmd(false /* userFacing */).Execute(); err != nil {
		// Cobra has already printed the error message.
		os.Exit(1)
	}
}
