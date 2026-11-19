// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sql

import (
	"github.com/cockroachdb/cockroach/pkg/sql/auditlogging/auditevents"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
)

func (p *planner) maybeAuditSensitiveTableAccessEvent(
	privilegeObject privilege.Object, priv privilege.Kind,
) {
	// Check if we can add this event.
	tableDesc, ok := privilegeObject.(catalog.TableDescriptor)
	if !ok || tableDesc.GetAuditMode() == descpb.TableDescriptor_DISABLED {
		return
	}
	writing := false
	switch priv {
	case privilege.INSERT, privilege.DELETE, privilege.UPDATE:
		writing = true
	}
	p.curPlan.auditEventBuilders = append(p.curPlan.auditEventBuilders,
		&auditevents.SensitiveTableAccessEvent{TableDesc: tableDesc, Writing: writing},
	)
}
