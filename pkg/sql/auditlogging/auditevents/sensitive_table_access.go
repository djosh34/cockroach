// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package auditevents

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/auditlogging"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
	"github.com/cockroachdb/errors"
)

var errTxnIsNotOpen = errors.New("txn is already committed or rolled back")

// SensitiveTableAccessEvent identifies accesses on tables that have been configured
// for auditing. See EXPERIMENTAL_AUDIT.
type SensitiveTableAccessEvent struct {
	TableDesc catalog.TableDescriptor
	Writing   bool
}

// BuildAuditEvent implements the auditlogging.AuditEventBuilder interface
func (s *SensitiveTableAccessEvent) BuildAuditEvent(
	ctx context.Context,
	assembler auditlogging.Auditor,
	details eventpb.CommonSQLEventDetails,
	exec eventpb.CommonSQLExecDetails,
) logpb.EventPayload {
	var err error
	var tn *tree.TableName
	tableName := ""
	mode := "r"
	if s.Writing {
		mode = "rw"
	}
	if assembler.Txn() != nil && assembler.Txn().IsOpen() {
		// Only open txn accepts further commands.
		tn, err = assembler.GetQualifiedTableNameByID(ctx, int64(s.TableDesc.GetID()), tree.ResolveRequireTableDesc)
	} else {
		err = errTxnIsNotOpen
	}
	if err != nil {
		log.Warningf(ctx, "name for audited table ID %d not found: %v", s.TableDesc.GetID(), err)
	} else {
		tableName = tn.FQString()
	}
	details.DescriptorID = uint32(s.TableDesc.GetID())
	return &eventpb.SensitiveTableAccess{
		CommonSQLEventDetails: details,
		CommonSQLExecDetails:  exec,
		TableName:             tableName,
		AccessMode:            mode,
	}
}
