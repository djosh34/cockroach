// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package serverctl

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/redact"
)

type ServerStartupInterface interface {
	ServerShutdownInterface

	// ClusterSettings retrieves this server's settings.
	ClusterSettings() *cluster.Settings

	// LogicalClusterID retrieves this server's logical cluster ID.
	LogicalClusterID() uuid.UUID

	// PreStart starts the server on the specified port(s) and
	// initializes subsystems.
	// It does not activate the pgwire listener over the network / unix
	// socket, which is done by the AcceptClients() method. The separation
	// between the two exists so that SQL initialization can take place
	// before the first client is accepted.
	PreStart(ctx context.Context) error

	// AcceptClients starts listening for incoming SQL clients over the network.
	AcceptClients(ctx context.Context) error
	// AcceptInternalClients starts listening for incoming internal SQL clients over the
	// loopback interface.
	AcceptInternalClients(ctx context.Context) error

	// InitialStart returns whether this node is starting for the first time.
	// This is (currently) used when displaying the server status report
	// on the terminal & in logs. We know that some folk have automation
	// that depend on certain strings displayed from this when orchestrating
	// KV-only nodes.
	InitialStart() bool

	// RunInitialSQL runs the SQL initialization for brand new clusters,
	// if the cluster is being started for the first time.
	// The arguments are:
	// - startSingleNode is used by 'demo' and 'start-single-node'.
	RunInitialSQL(ctx context.Context, startSingleNode bool) error
}

// ServerShutdownInterface is the subset of the APIs on a server
// object that's sufficient to run a server shutdown.
type ServerShutdownInterface interface {
	AnnotateCtx(context.Context) context.Context
	Drain(ctx context.Context, verbose bool) (uint64, redact.RedactableString, error)
	ShutdownRequested() <-chan ShutdownRequest
}

// ShutdownRequest is used to signal a request to shutdown the server through
// server.stopTrigger. It carries the reason for the shutdown.
type ShutdownRequest struct {
	// Reason identifies the cause of the shutdown.
	Reason ShutdownReason
	// Err is populated for reason ServerStartupError and FatalError.
	Err error
}

// ShutdownReason identifies the reason for a ShutdownRequest.
type ShutdownReason int

const (
	// ShutdownReasonDrainRPC represents a drain RPC with the shutdown flag set.
	ShutdownReasonDrainRPC ShutdownReason = iota
	// ShutdownReasonServerStartupError means that the server startup process
	// failed.
	ShutdownReasonServerStartupError
	// ShutdownReasonFatalError identifies an error that requires the server be
	// terminated immediately.
	ShutdownReasonFatalError
	// ShutdownReasonGracefulStopRequestedByOrchestration is used when a graceful shutdown
	// was requested by orchestration.
	ShutdownReasonGracefulStopRequestedByOrchestration
)
