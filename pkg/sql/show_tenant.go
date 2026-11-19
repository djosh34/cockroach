// Copyright 2022 The Cockroach Authors.
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
	"context"

	"github.com/cockroachdb/cockroach/pkg/multitenant/mtinfopb"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/syntheticprivilege"
)

type tenantValues struct {
	tenantInfo   *mtinfopb.TenantInfo
	dataState    string
	capabilities []showTenantNodeCapability
}

type showTenantNodeCapability struct {
	name  string
	value string
}

type showTenantNode struct {
	tenantSpec       tenantSpec
	withCapabilities bool
	columns          colinfo.ResultColumns
	tenantIDIndex    int
	tenantIds        []roachpb.TenantID
	initTenantValues bool
	values           *tenantValues
	capabilityIndex  int
	capability       showTenantNodeCapability
}

// ShowTenant constructs a showTenantNode.
func (p *planner) ShowTenant(ctx context.Context, n *tree.ShowTenant) (planNode, error) {
	if err := CanManageTenant(ctx, p); err != nil {
		return nil, err
	}

	if err := rejectIfCantCoordinateMultiTenancy(p.execCfg.Codec, "show", p.execCfg.Settings); err != nil {
		return nil, err
	}

	tspec, err := p.planTenantSpec(ctx, n.TenantSpec, "SHOW VIRTUAL CLUSTER")
	if err != nil {
		return nil, err
	}

	node := &showTenantNode{
		tenantSpec:       tspec,
		withCapabilities: n.WithCapabilities,
		initTenantValues: true,
	}

	node.columns = colinfo.TenantColumns
	if n.WithCapabilities {
		node.columns = append(node.columns, colinfo.TenantColumnsWithCapabilities...)
	}

	return node, nil
}

func CanManageTenant(ctx context.Context, p AuthorizationAccessor) error {
	isAdmin, err := p.HasAdminRole(ctx)
	if err != nil {
		return err
	}
	if isAdmin {
		return nil
	}

	return p.CheckPrivilege(ctx, syntheticprivilege.GlobalPrivilegeObject, privilege.MANAGEVIRTUALCLUSTER)
}

func (n *showTenantNode) startExec(params runParams) error {
	if _, ok := n.tenantSpec.(tenantSpecAll); ok {
		ids, err := GetAllNonDropTenantIDs(params.ctx, params.p.InternalSQLTxn(), params.p.ExecCfg().Settings)
		if err != nil {
			return err
		}
		n.tenantIds = ids
	} else {
		tenantRecord, err := n.tenantSpec.getTenantInfo(params.ctx, params.p)
		if err != nil {
			return err
		}
		n.tenantIds = []roachpb.TenantID{roachpb.MustMakeTenantID(tenantRecord.ID)}
	}
	return nil
}

func (n *showTenantNode) getTenantValues(
	params runParams, tenantInfo *mtinfopb.TenantInfo,
) (*tenantValues, error) {
	// Common fields.
	var values tenantValues
	values.tenantInfo = tenantInfo

	// Add capabilities if requested.
	if n.withCapabilities {
		showTenantNodeCapabilities := make([]showTenantNodeCapability, 0, len(tenantcapabilities.IDs))
		for _, id := range tenantcapabilities.IDs {
			value := tenantcapabilities.MustGetValueByID(&tenantInfo.Capabilities, id)
			showTenantNodeCapabilities = append(showTenantNodeCapabilities, showTenantNodeCapability{
				name:  id.String(),
				value: value.String(),
			})
		}
		values.capabilities = showTenantNodeCapabilities
	}

	values.dataState = values.tenantInfo.DataState.String()

	return &values, nil
}

func (n *showTenantNode) Next(params runParams) (bool, error) {
	if n.tenantIDIndex >= len(n.tenantIds) {
		return false, nil
	}

	if n.initTenantValues {
		tenantInfo, err := GetTenantRecordByID(params.ctx, params.p.InternalSQLTxn(), n.tenantIds[n.tenantIDIndex], params.p.ExecCfg().Settings)
		if err != nil {
			return false, err
		}
		values, err := n.getTenantValues(params, tenantInfo)
		if err != nil {
			return false, err
		}
		n.values = values
		n.initTenantValues = false
	}

	if n.withCapabilities {
		capabilities := n.values.capabilities
		n.capability = capabilities[n.capabilityIndex]
		if n.capabilityIndex == len(capabilities)-1 {
			n.capabilityIndex = 0
			n.tenantIDIndex++
			n.initTenantValues = true
		} else {
			n.capabilityIndex++
		}
	} else {
		n.tenantIDIndex++
		n.initTenantValues = true
	}

	return true, nil
}

func (n *showTenantNode) Values() tree.Datums {
	v := n.values
	tenantInfo := v.tenantInfo
	result := tree.Datums{
		tree.NewDInt(tree.DInt(tenantInfo.ID)),
		tree.NewDString(string(tenantInfo.Name)),
		tree.NewDString(v.dataState),
		tree.NewDString(tenantInfo.ServiceMode.String()),
	}

	if n.withCapabilities {
		capability := n.capability
		result = append(result,
			tree.NewDString(capability.name),
			tree.NewDString(capability.value),
		)
	}

	return result
}

func (n *showTenantNode) Close(_ context.Context) {}
