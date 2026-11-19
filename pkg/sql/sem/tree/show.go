// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in licenses/BSD-vitess.txt.

// Portions of this file are additionally subject to the following
// license and copyright.
//
// Copyright 2015 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// This code was derived from https://github.com/youtube/vitess.

package tree

import (
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/lexbase"
)

// ShowVar represents a SHOW statement.
type ShowVar struct {
	Name string
}

// Format implements the NodeFormatter interface.
func (node *ShowVar) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	// Session var names never contain PII and should be distinguished
	// for feature tracking purposes.
	ctx.WithFlags(ctx.flags & ^FmtAnonymize & ^FmtMarkRedactionNode, func() {
		ctx.FormatNameP(&node.Name)
	})
}

// ShowClusterSetting represents a SHOW CLUSTER SETTING statement.
type ShowClusterSetting struct {
	Name string
}

// Format implements the NodeFormatter interface.
func (node *ShowClusterSetting) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CLUSTER SETTING ")
	// Cluster setting names never contain PII and should be distinguished
	// for feature tracking purposes.
	ctx.WithFlags(ctx.flags & ^FmtAnonymize & ^FmtMarkRedactionNode, func() {
		ctx.FormatNameP(&node.Name)
	})
}

// ShowClusterSettingList represents a SHOW [ALL|PUBLIC] CLUSTER SETTINGS statement.
type ShowClusterSettingList struct {
	// All indicates whether to include non-public settings in the output.
	All bool
}

// Format implements the NodeFormatter interface.
func (node *ShowClusterSettingList) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	qual := "PUBLIC"
	if node.All {
		qual = "ALL"
	}
	ctx.WriteString(qual)
	ctx.WriteString(" CLUSTER SETTINGS")
}

// ShowColumns represents a SHOW COLUMNS statement.
type ShowColumns struct {
	Table       *UnresolvedObjectName
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowColumns) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW COLUMNS FROM ")
	ctx.FormatNode(node.Table)

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowDatabases represents a SHOW DATABASES statement.
type ShowDatabases struct {
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowDatabases) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW DATABASES")

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowEnums represents a SHOW ENUMS statement.
type ShowEnums struct {
	ObjectNamePrefix
}

// Format implements the NodeFormatter interface.
func (node *ShowEnums) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ENUMS")
}

// ShowTypes represents a SHOW TYPES statement.
type ShowTypes struct{}

// Format implements the NodeFormatter interface.
func (node *ShowTypes) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW TYPES")
}

// ShowTraceType is an enum of SHOW TRACE variants.
type ShowTraceType string

// A list of the SHOW TRACE variants.
const (
	ShowTraceRaw     ShowTraceType = "TRACE"
	ShowTraceKV      ShowTraceType = "KV TRACE"
	ShowTraceReplica ShowTraceType = "EXPERIMENTAL_REPLICA TRACE"
)

// ShowTraceForSession represents a SHOW TRACE FOR SESSION statement.
type ShowTraceForSession struct {
	TraceType ShowTraceType
	Compact   bool
}

// Format implements the NodeFormatter interface.
func (node *ShowTraceForSession) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.Compact {
		ctx.WriteString("COMPACT ")
	}
	ctx.WriteString(string(node.TraceType))
	ctx.WriteString(" FOR SESSION")
}

// ShowIndexes represents a SHOW INDEX statement.
type ShowIndexes struct {
	Table       *UnresolvedObjectName
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowIndexes) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW INDEXES FROM ")
	ctx.FormatNode(node.Table)

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowDatabaseIndexes represents a SHOW INDEXES FROM DATABASE statement.
type ShowDatabaseIndexes struct {
	Database    Name
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowDatabaseIndexes) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW INDEXES FROM DATABASE ")
	ctx.FormatNode(&node.Database)

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowQueries represents a SHOW STATEMENTS statement.
type ShowQueries struct {
	All     bool
	Cluster bool
}

// Format implements the NodeFormatter interface.
func (node *ShowQueries) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.All {
		ctx.WriteString("ALL ")
	}
	if node.Cluster {
		ctx.WriteString("CLUSTER STATEMENTS")
	} else {
		ctx.WriteString("LOCAL STATEMENTS")
	}
}

// ShowJobs represents a SHOW JOBS statement
type ShowJobs struct {
	// If non-nil, a select statement that provides the job ids to be shown.
	Jobs *Select

	// If Automatic is true, show only automatically-generated jobs such
	// as automatic CREATE STATISTICS jobs. If Automatic is false, show
	// only non-automatically-generated jobs.
	Automatic bool

	// Whether to block and wait for completion of all running jobs to be displayed.
	Block bool

	// If non-nil, only display jobs started by the specified
	// schedules.
	Schedules *Select

	// Options contain any options that were specified in the `SHOW JOB` query.
	Options *ShowJobOptions
}

// Format implements the NodeFormatter interface.
func (node *ShowJobs) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.Automatic {
		ctx.WriteString("AUTOMATIC ")
	}
	ctx.WriteString("JOBS")
	if node.Block {
		ctx.WriteString(" WHEN COMPLETE")
	}
	if node.Jobs != nil {
		ctx.WriteString(" ")
		ctx.FormatNode(node.Jobs)
	}
	if node.Schedules != nil {
		ctx.WriteString(" FOR SCHEDULES ")
		ctx.FormatNode(node.Schedules)
	}
	if node.Options != nil {
		ctx.WriteString(" WITH")
		ctx.FormatNode(node.Options)
	}
}

// ShowJobOptions describes options for the SHOW JOB execution.
type ShowJobOptions struct {
	// ExecutionDetails, if true, will render job specific details about the job's
	// execution. These details will provide improved observability into the
	// execution of the job.
	ExecutionDetails bool
}

func (s *ShowJobOptions) Format(ctx *FmtCtx) {
	if s.ExecutionDetails {
		ctx.WriteString(" EXECUTION DETAILS")
	}
}

func (s *ShowJobOptions) CombineWith(other *ShowJobOptions) error {
	s.ExecutionDetails = other.ExecutionDetails
	return nil
}

var _ NodeFormatter = &ShowJobOptions{}

// ShowSessions represents a SHOW SESSIONS statement
type ShowSessions struct {
	All     bool
	Cluster bool
}

// Format implements the NodeFormatter interface.
func (node *ShowSessions) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.All {
		ctx.WriteString("ALL ")
	}
	if node.Cluster {
		ctx.WriteString("CLUSTER SESSIONS")
	} else {
		ctx.WriteString("LOCAL SESSIONS")
	}
}

// ShowSchemas represents a SHOW SCHEMAS statement.
type ShowSchemas struct {
	Database Name
}

// Format implements the NodeFormatter interface.
func (node *ShowSchemas) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW SCHEMAS")
	if node.Database != "" {
		ctx.WriteString(" FROM ")
		ctx.FormatNode(&node.Database)
	}
}

// ShowSequences represents a SHOW SEQUENCES statement.
type ShowSequences struct {
	Database Name
}

// Format implements the NodeFormatter interface.
func (node *ShowSequences) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW SEQUENCES")
	if node.Database != "" {
		ctx.WriteString(" FROM ")
		ctx.FormatNode(&node.Database)
	}
}

// ShowTables represents a SHOW TABLES statement.
type ShowTables struct {
	ObjectNamePrefix
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowTables) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW TABLES")
	if node.ExplicitSchema {
		ctx.WriteString(" FROM ")
		ctx.FormatNode(&node.ObjectNamePrefix)
	}

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowRoutines represents a SHOW FUNCTIONS or SHOW PROCEDURES statement.
type ShowRoutines struct {
	ObjectNamePrefix
	Procedure bool
}

// Format implements the NodeFormatter interface.
func (node *ShowRoutines) Format(ctx *FmtCtx) {
	if node.Procedure {
		ctx.WriteString("SHOW PROCEDURES")
	} else {
		ctx.WriteString("SHOW FUNCTIONS")
	}
	if node.ExplicitSchema {
		ctx.WriteString(" FROM ")
		ctx.FormatNode(&node.ObjectNamePrefix)
	}
}

// ShowTransactions represents a SHOW TRANSACTIONS statement
type ShowTransactions struct {
	All     bool
	Cluster bool
}

// Format implements the NodeFormatter interface.
func (node *ShowTransactions) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.All {
		ctx.WriteString("ALL ")
	}
	if node.Cluster {
		ctx.WriteString("CLUSTER TRANSACTIONS")
	} else {
		ctx.WriteString("LOCAL TRANSACTIONS")
	}
}

// ShowConstraints represents a SHOW CONSTRAINTS statement.
type ShowConstraints struct {
	Table       *UnresolvedObjectName
	WithComment bool
}

// Format implements the NodeFormatter interface.
func (node *ShowConstraints) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CONSTRAINTS FROM ")
	ctx.FormatNode(node.Table)

	if node.WithComment {
		ctx.WriteString(" WITH COMMENT")
	}
}

// ShowGrants represents a SHOW GRANTS statement.
// GrantTargetList is defined in grant.go.
type ShowGrants struct {
	Targets  *GrantTargetList
	Grantees RoleSpecList
}

// Format implements the NodeFormatter interface.
func (node *ShowGrants) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.Targets != nil && node.Targets.System {
		ctx.WriteString("SYSTEM ")
	}
	ctx.WriteString("GRANTS")
	if node.Targets != nil {
		if !node.Targets.System {
			ctx.WriteString(" ON ")
			ctx.FormatNode(node.Targets)
		}
	}
	if node.Grantees != nil {
		ctx.WriteString(" FOR ")
		ctx.FormatNode(&node.Grantees)
	}
}

// ShowRoleGrants represents a SHOW GRANTS ON ROLE statement.
type ShowRoleGrants struct {
	Roles    RoleSpecList
	Grantees RoleSpecList
}

// Format implements the NodeFormatter interface.
func (node *ShowRoleGrants) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW GRANTS ON ROLE")
	if node.Roles != nil {
		ctx.WriteString(" ")
		ctx.FormatNode(&node.Roles)
	}
	if node.Grantees != nil {
		ctx.WriteString(" FOR ")
		ctx.FormatNode(&node.Grantees)
	}
}

// ShowCreateMode denotes what kind of SHOW CREATE should be used
type ShowCreateMode int

const (
	// ShowCreateModeTable represents SHOW CREATE TABLE
	ShowCreateModeTable ShowCreateMode = iota
	// ShowCreateModeView represents SHOW CREATE VIEW
	ShowCreateModeView
	// ShowCreateModeSequence represents SHOW CREATE SEQUENCE
	ShowCreateModeSequence
	// ShowCreateModeDatabase represents SHOW CREATE DATABASE
	ShowCreateModeDatabase
	// ShowCreateModeIndexes represents SHOW CREATE INDEXES FROM
	ShowCreateModeIndexes
	// ShowCreateModeSecondaryIndexes represents SHOW CREATE SECONDARY INDEXES FROM
	ShowCreateModeSecondaryIndexes
)

type ShowCreateFormatOption int

const (
	ShowCreateFormatOptionNone ShowCreateFormatOption = iota
	ShowCreateFormatOptionRedactedValues
)

// ShowCreate represents a SHOW CREATE statement.
type ShowCreate struct {
	Mode   ShowCreateMode
	Name   *UnresolvedObjectName
	FmtOpt ShowCreateFormatOption
}

// Format implements the NodeFormatter interface.
func (node *ShowCreate) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CREATE ")

	switch node.Mode {
	case ShowCreateModeDatabase:
		ctx.WriteString("DATABASE ")
	case ShowCreateModeIndexes:
		ctx.WriteString("INDEXES FROM ")
	case ShowCreateModeSecondaryIndexes:
		ctx.WriteString("SECONDARY INDEXES FROM ")
	}
	ctx.FormatNode(node.Name)

	switch node.FmtOpt {
	case ShowCreateFormatOptionRedactedValues:
		ctx.WriteString(" WITH REDACT")
	}
}

// ShowCreateAllSchemas represents a SHOW CREATE ALL SCHEMAS statement.
type ShowCreateAllSchemas struct{}

// Format implements the NodeFormatter interface.
func (node *ShowCreateAllSchemas) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CREATE ALL SCHEMAS")
}

// ShowCreateAllTables represents a SHOW CREATE ALL TABLES statement.
type ShowCreateAllTables struct{}

// Format implements the NodeFormatter interface.
func (node *ShowCreateAllTables) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CREATE ALL TABLES")
}

// ShowCreateAllTypes represents a SHOW CREATE ALL TYPES statement.
type ShowCreateAllTypes struct{}

// Format implements the NodeFormatter interface.
func (node *ShowCreateAllTypes) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW CREATE ALL TYPES")
}

// ShowCreateSchedules represents a SHOW CREATE SCHEDULE statement.
type ShowCreateSchedules struct {
	ScheduleID Expr
}

// Format implements the NodeFormatter interface.
func (node *ShowCreateSchedules) Format(ctx *FmtCtx) {
	if node.ScheduleID != nil {
		ctx.WriteString("SHOW CREATE SCHEDULE ")
		ctx.FormatNode(node.ScheduleID)
		return
	}
	ctx.Printf("SHOW CREATE ALL SCHEDULES")
}

// ShowSyntax represents a SHOW SYNTAX statement.
// This the most lightweight thing that can be done on a statement
// server-side: just report the statement that was entered without
// any processing. Meant for use for syntax checking on clients,
// when the client version might differ from the server.
type ShowSyntax struct {
	Statement string
}

// Format implements the NodeFormatter interface.
func (node *ShowSyntax) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW SYNTAX ")
	if ctx.flags.HasFlags(FmtAnonymize) || ctx.flags.HasFlags(FmtHideConstants) {
		ctx.WriteString("'_'")
	} else {
		ctx.WriteString(lexbase.EscapeSQLString(node.Statement))
	}
}

// ShowTransactionStatus represents a SHOW TRANSACTION STATUS statement.
type ShowTransactionStatus struct {
}

// Format implements the NodeFormatter interface.
func (node *ShowTransactionStatus) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW TRANSACTION STATUS")
}

// ShowLastQueryStatistics represents a SHOW LAST QUERY STATS statement.
type ShowLastQueryStatistics struct {
	Columns NameList
}

// ShowLastQueryStatisticsDefaultColumns is the default list of columns
// when the USING clause is not specified.
// Note: the form that does not specify the USING clause is deprecated.
// Remove it when there are no more clients using it (22.1 or later).
var ShowLastQueryStatisticsDefaultColumns = NameList([]Name{
	"parse_latency",
	"plan_latency",
	"exec_latency",
	"service_latency",
	"post_commit_jobs_latency",
})

// Format implements the NodeFormatter interface.
func (node *ShowLastQueryStatistics) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW LAST QUERY STATISTICS RETURNING ")
	// The column names for this statement never contain PII and should
	// be distinguished for feature tracking purposes.
	ctx.WithFlags(ctx.flags & ^FmtAnonymize & ^FmtMarkRedactionNode, func() {
		ctx.FormatNode(&node.Columns)
	})
}

// ShowFullTableScans represents a SHOW FULL TABLE SCANS statement.
type ShowFullTableScans struct {
}

// Format implements the NodeFormatter interface.
func (node *ShowFullTableScans) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW FULL TABLE SCANS")
}

// ShowSavepointStatus represents a SHOW SAVEPOINT STATUS statement.
type ShowSavepointStatus struct {
}

// Format implements the NodeFormatter interface.
func (node *ShowSavepointStatus) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW SAVEPOINT STATUS")
}

// ShowUsers represents a SHOW USERS statement.
type ShowUsers struct {
}

// Format implements the NodeFormatter interface.
func (node *ShowUsers) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW USERS")
}

// ShowRoles represents a SHOW ROLES statement.
type ShowRoles struct {
}

// Format implements the NodeFormatter interface.
func (node *ShowRoles) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ROLES")
}

// ShowRanges represents a SHOW RANGES statement.
type ShowRanges struct {
	DatabaseName Name
	TableOrIndex TableIndexName
	Options      *ShowRangesOptions
	Source       ShowRangesSource
}

// ShowRangesSource represents the source of a SHOW RANGES statement.
type ShowRangesSource int8

const (
	// SHOW RANGES FROM CURRENT_CATALOG
	ShowRangesCurrentDatabase ShowRangesSource = iota
	// SHOW RANGES FROM DATABASE
	ShowRangesDatabase
	// SHOW RANGES FROM TABLE
	ShowRangesTable
	// SHOW RANGES FROM INDEX
	ShowRangesIndex
	// SHOW CLUSTER RANGES
	ShowRangesCluster
)

// ShowRangesOptions represents the WITH clause in SHOW RANGES.
type ShowRangesOptions struct {
	Details bool
	Explain bool
	Keys    bool
	Mode    ShowRangesMode
}

// ShowRangesMode represents the WITH clause in SHOW RANGES.
type ShowRangesMode int8

const (
	// UniqueRanges tells to use just 1 row per range in the output.
	//
	// Note: The UniqueRanges constant must have value 0; otherwise,
	// the parsing logic would become incorrect.
	UniqueRanges ShowRangesMode = iota
	// ExpandTables requests one row per table in the output.
	ExpandTables
	// ExpandIndexes requests one row per index in the output.
	ExpandIndexes
)

// Format implements the NodeFormatter interface.
func (node *ShowRanges) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW ")
	if node.Source == ShowRangesCluster {
		ctx.WriteString("CLUSTER ")
	}
	ctx.WriteString("RANGES")
	if node.Source != ShowRangesCluster {
		ctx.WriteString(" FROM ")
		switch node.Source {
		case ShowRangesCurrentDatabase:
			ctx.WriteString("CURRENT_CATALOG")
		case ShowRangesDatabase:
			ctx.WriteString("DATABASE ")
			ctx.FormatNode(&node.DatabaseName)
		case ShowRangesIndex:
			ctx.WriteString("INDEX ")
			ctx.FormatNode(&node.TableOrIndex)
		case ShowRangesTable:
			ctx.WriteString("TABLE ")
			ctx.FormatNode(&node.TableOrIndex)
		}
	}
	ctx.FormatNode(node.Options)
}

// Format implements the NodeFormatter interface.
func (node *ShowRangesOptions) Format(ctx *FmtCtx) {
	noOpts := ShowRangesOptions{}
	if *node == noOpts {
		return
	}
	ctx.WriteString(" WITH ")
	comma := ""
	if node.Details {
		ctx.WriteString("DETAILS")
		comma = ", "
	}
	if node.Keys {
		ctx.WriteString(comma)
		ctx.WriteString("KEYS")
		comma = ", "
	}
	if node.Explain {
		ctx.WriteString(comma)
		ctx.WriteString("EXPLAIN")
		comma = ", "
	}
	if node.Mode != UniqueRanges {
		ctx.WriteString(comma)
		switch node.Mode {
		case ExpandTables:
			ctx.WriteString("TABLES")
		case ExpandIndexes:
			ctx.WriteString("INDEXES")
		}
	}
}

// ShowRangeForRow represents a SHOW RANGE FOR ROW statement.
type ShowRangeForRow struct {
	TableOrIndex TableIndexName
	Row          Exprs
}

// Format implements the NodeFormatter interface.
func (node *ShowRangeForRow) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW RANGE FROM ")
	if node.TableOrIndex.Index != "" {
		ctx.WriteString("INDEX ")
	} else {
		ctx.WriteString("TABLE ")
	}
	ctx.FormatNode(&node.TableOrIndex)
	ctx.WriteString(" FOR ROW (")
	ctx.FormatNode(&node.Row)
	ctx.WriteString(")")
}

// ShowFingerprints represents a SHOW EXPERIMENTAL_FINGERPRINTS statement.
type ShowFingerprints struct {
	TenantSpec *TenantSpec
	Table      *UnresolvedObjectName
}

// Format implements the NodeFormatter interface.
func (node *ShowFingerprints) Format(ctx *FmtCtx) {
	if node.Table != nil {
		ctx.WriteString("SHOW EXPERIMENTAL_FINGERPRINTS FROM TABLE ")
		ctx.FormatNode(node.Table)
	} else {
		ctx.WriteString("SHOW EXPERIMENTAL_FINGERPRINTS FROM VIRTUAL CLUSTER ")
		ctx.FormatNode(node.TenantSpec)
	}
}

// ShowTableStats represents a SHOW STATISTICS FOR TABLE statement.
type ShowTableStats struct {
	Table     *UnresolvedObjectName
	UsingJSON bool
	Options   KVOptions
}

// Format implements the NodeFormatter interface.
func (node *ShowTableStats) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW STATISTICS ")
	if node.UsingJSON {
		ctx.WriteString("USING JSON ")
	}
	ctx.WriteString("FOR TABLE ")
	ctx.FormatNode(node.Table)
	if len(node.Options) > 0 {
		ctx.WriteString(" WITH OPTIONS (")
		ctx.FormatNode(&node.Options)
		ctx.WriteString(")")
	}
}

// ShowTenantOptions represents the WITH clause in SHOW VIRTUAL CLUSTER.
type ShowTenantOptions struct {
	WithCapabilities bool
}

// ShowTenant represents a SHOW VIRTUAL CLUSTER statement.
type ShowTenant struct {
	TenantSpec *TenantSpec
	ShowTenantOptions
}

// Format implements the NodeFormatter interface.
func (node *ShowTenant) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW VIRTUAL CLUSTER ")
	ctx.FormatNode(node.TenantSpec)

	withs := []string{}
	if node.WithCapabilities {
		withs = append(withs, "CAPABILITIES")
	}
	if len(withs) > 0 {
		ctx.WriteString(" WITH ")
		ctx.WriteString(strings.Join(withs, ", "))
	}
}

// ShowHistogram represents a SHOW HISTOGRAM statement.
type ShowHistogram struct {
	HistogramID int64
}

// Format implements the NodeFormatter interface.
func (node *ShowHistogram) Format(ctx *FmtCtx) {
	ctx.Printf("SHOW HISTOGRAM %d", node.HistogramID)
}

// ScheduledJobExecutorType is a type identifying the names of
// the supported scheduled job executors.
type ScheduledJobExecutorType int

const (
	// InvalidExecutor is a placeholder for an invalid executor type.
	InvalidExecutor ScheduledJobExecutorType = iota

	// ScheduledSQLStatsCompactionExecutor is an executor responsible for the
	// execution of the scheduled SQL Stats compaction.
	ScheduledSQLStatsCompactionExecutor

	// ScheduledRowLevelTTLExecutor is an executor responsible for the cleanup
	// of rows on row level TTL tables.
	ScheduledRowLevelTTLExecutor

	// ScheduledSchemaTelemetryExecutor is an executor responsible for the logging
	// of schema telemetry.
	ScheduledSchemaTelemetryExecutor
)

var scheduleExecutorInternalNames = map[ScheduledJobExecutorType]string{
	InvalidExecutor:                     "unknown-executor",
	ScheduledSQLStatsCompactionExecutor: "scheduled-sql-stats-compaction-executor",
	ScheduledRowLevelTTLExecutor:        "scheduled-row-level-ttl-executor",
	ScheduledSchemaTelemetryExecutor:    "scheduled-schema-telemetry-executor",
}

// InternalName returns an internal executor name.
// This name can be used to filter matching schedules.
func (t ScheduledJobExecutorType) InternalName() string {
	return scheduleExecutorInternalNames[t]
}

// UserName returns a user friendly executor name.
func (t ScheduledJobExecutorType) UserName() string {
	switch t {
	case ScheduledSQLStatsCompactionExecutor:
		return "SQL STATISTICS"
	case ScheduledRowLevelTTLExecutor:
		return "ROW LEVEL TTL"
	case ScheduledSchemaTelemetryExecutor:
		return "SCHEMA TELEMETRY"
	}
	return "unsupported-executor"
}

// ScheduleState describes what kind of schedules to display
type ScheduleState int

const (
	// SpecifiedSchedules indicates that show schedules should
	// only show subset of schedules.
	SpecifiedSchedules ScheduleState = iota

	// ActiveSchedules indicates that show schedules should
	// only show those schedules that are currently active.
	ActiveSchedules

	// PausedSchedules indicates that show schedules should
	// only show those schedules that are currently paused.
	PausedSchedules
)

// Format implements the NodeFormatter interface.
func (s ScheduleState) Format(ctx *FmtCtx) {
	switch s {
	case ActiveSchedules:
		ctx.WriteString("RUNNING")
	case PausedSchedules:
		ctx.WriteString("PAUSED")
	default:
		// Nothing
	}
}

// ShowSchedules represents a SHOW SCHEDULES statement.
type ShowSchedules struct {
	WhichSchedules ScheduleState
	ExecutorType   ScheduledJobExecutorType
	ScheduleID     Expr
}

var _ Statement = &ShowSchedules{}

// Format implements the NodeFormatter interface.
func (n *ShowSchedules) Format(ctx *FmtCtx) {
	if n.ScheduleID != nil {
		ctx.WriteString("SHOW SCHEDULE ")
		ctx.FormatNode(n.ScheduleID)
		return
	}
	ctx.Printf("SHOW")

	if n.WhichSchedules != SpecifiedSchedules {
		ctx.WriteString(" ")
		ctx.FormatNode(&n.WhichSchedules)
	}

	ctx.Printf(" SCHEDULES")

	if n.ExecutorType != InvalidExecutor {
		// TODO(knz): beware of using ctx.FormatNode here if
		// FOR changes to support expressions.
		ctx.Printf(" FOR %s", n.ExecutorType.UserName())
	}
}

// ShowDefaultPrivileges represents a SHOW DEFAULT PRIVILEGES statement.
type ShowDefaultPrivileges struct {
	Roles       RoleSpecList
	ForAllRoles bool
	ForGrantee  bool
	// If Schema is not specified, SHOW DEFAULT PRIVILEGES is being
	// run on the current database.
	Schema Name
}

var _ Statement = &ShowDefaultPrivileges{}

// Format implements the NodeFormatter interface.
func (n *ShowDefaultPrivileges) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW DEFAULT PRIVILEGES ")
	if len(n.Roles) > 0 {
		if n.ForGrantee {
			ctx.WriteString("FOR GRANTEE ")
		} else {
			ctx.WriteString("FOR ROLE ")
		}

		for i := range n.Roles {
			if i > 0 {
				ctx.WriteString(", ")
			}
			ctx.FormatNode(&n.Roles[i])
		}
		ctx.WriteString(" ")
	} else if n.ForAllRoles {
		ctx.WriteString("FOR ALL ROLES ")
	}
	if n.Schema != Name("") {
		ctx.WriteString("IN SCHEMA ")
		ctx.FormatNode(&n.Schema)
	}
}

// ShowCompletions represents a SHOW COMPLETIONS statement.
type ShowCompletions struct {
	Statement *StrVal
	Offset    *NumVal
}

// Format implements the NodeFormatter interface.
func (s ShowCompletions) Format(ctx *FmtCtx) {
	ctx.WriteString("SHOW COMPLETIONS AT OFFSET ")
	s.Offset.Format(ctx)
	ctx.WriteString(" FOR ")
	ctx.FormatNode(s.Statement)
}

var _ Statement = &ShowCompletions{}

// ShowCreateRoutine represents a SHOW CREATE FUNCTION or SHOW CREATE PROCEDURE
// statement.
type ShowCreateRoutine struct {
	Name      ResolvableFunctionReference
	Procedure bool
}

// Format implements the NodeFormatter interface.
func (node *ShowCreateRoutine) Format(ctx *FmtCtx) {
	if node.Procedure {
		ctx.WriteString("SHOW CREATE PROCEDURE ")
	} else {
		ctx.WriteString("SHOW CREATE FUNCTION ")
	}
	ctx.FormatNode(&node.Name)
}

var _ Statement = &ShowCreateRoutine{}

// ShowCreateExternalConnections represents a SHOW CREATE EXTERNAL CONNECTION
// statement.
type ShowCreateExternalConnections struct {
	ConnectionLabel Expr
}

// Format implements the NodeFormatter interface.
func (node *ShowCreateExternalConnections) Format(ctx *FmtCtx) {
	if node.ConnectionLabel != nil {
		ctx.WriteString("SHOW CREATE EXTERNAL CONNECTION ")
		ctx.FormatNode(node.ConnectionLabel)
		return
	}
	ctx.Printf("SHOW CREATE ALL EXTERNAL CONNECTIONS")
}

var _ Statement = &ShowCreateExternalConnections{}

// ShowCommitTimestamp represents a SHOW COMMIT TIMESTAMP statement.
//
// If the current session is in an open transaction state, this statement will
// implicitly commit the underlying kv transaction and return the HLC timestamp
// at which it committed. The transaction state machine will be left in a state
// such that only COMMIT or RELEASE cockroach_savepoint; COMMIT are acceptable.
// The statement may also be sent after RELEASE cockroach_savepoint; and before
// COMMIT.
//
// If the current session is not in an open transaction state, this statement
// will return the commit timestamp of the previous transaction, assuming there
// was one.
type ShowCommitTimestamp struct{}

func (s ShowCommitTimestamp) Format(ctx *FmtCtx) {
	ctx.Printf("SHOW COMMIT TIMESTAMP")
}

var _ Statement = (*ShowCommitTimestamp)(nil)
