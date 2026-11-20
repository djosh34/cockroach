// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

// AlterTenantSetClusterSetting represents an ALTER VIRTUAL CLUSTER
// SET CLUSTER SETTING statement.
type AlterTenantSetClusterSetting struct {
	SetClusterSetting
	TenantSpec *TenantSpec
}

// Format implements the NodeFormatter interface.
func (n *AlterTenantSetClusterSetting) Format(ctx *FmtCtx) {
	ctx.WriteString("ALTER VIRTUAL CLUSTER ")
	ctx.FormatNode(n.TenantSpec)
	ctx.WriteByte(' ')
	ctx.FormatNode(&n.SetClusterSetting)
}

// ShowTenantClusterSetting represents a SHOW CLUSTER SETTING ... FOR VIRTUAL CLUSTER statement.
type ShowTenantClusterSetting struct {
	*ShowClusterSetting
	TenantSpec *TenantSpec
}

// Format implements the NodeFormatter interface.
func (node *ShowTenantClusterSetting) Format(ctx *FmtCtx) {
	ctx.FormatNode(node.ShowClusterSetting)
	ctx.WriteString(" FOR VIRTUAL CLUSTER ")
	ctx.FormatNode(node.TenantSpec)
}

// ShowTenantClusterSettingList represents a SHOW CLUSTER SETTINGS FOR VIRTUAL CLUSTER statement.
type ShowTenantClusterSettingList struct {
	*ShowClusterSettingList
	TenantSpec *TenantSpec
}

// Format implements the NodeFormatter interface.
func (node *ShowTenantClusterSettingList) Format(ctx *FmtCtx) {
	ctx.FormatNode(node.ShowClusterSettingList)
	ctx.WriteString(" FOR VIRTUAL CLUSTER ")
	ctx.FormatNode(node.TenantSpec)
}
