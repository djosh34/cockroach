// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package auditlogging

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
)

// Auditor is an interface used to check and build different audit events.
type Auditor interface {
	GetQualifiedTableNameByID(ctx context.Context, id int64, requiredType tree.RequiredTableKind) (*tree.TableName, error)
	Txn() *kv.Txn
}

// AuditEventBuilder is the interface used to build different audit events.
type AuditEventBuilder interface {
	BuildAuditEvent(
		context.Context,
		Auditor,
		eventpb.CommonSQLEventDetails,
		eventpb.CommonSQLExecDetails,
	) logpb.EventPayload
}
