// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvserverbase"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catalogkeys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descbuilder"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scbackup"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

// A cluster restore works like upstream's full cluster restore: into a
// cluster with no user databases or tables, it brings back every
// database, schema, table, type, function and view with its ID, the data
// of the tables as of the target time, the users, roles and grants, the
// cluster settings, zone configs, comments and table statistics. The
// pre-loaded defaultdb and postgres are replaced by those of the backup.
// Jobs, schedules and the repository settings are not restored; the
// restored cluster backs up wherever its own --config says.
//
// The job loads the full backup and the segments into a Pebble engine in
// a temporary folder and reads it as of the target time. It writes the
// table data with AddSSTable at a new timestamp, then publishes the
// catalog and the system tables in one transaction.
//
// Objects that were being dropped or created at the target time are left
// out, since they were not visible then. A table that was taken offline by
// an IMPORT INTO comes back online with the data it had before the import.
// Schema changes that were running at the target time get new jobs and
// carry on.

// restoredSystemTables are the system tables whose rows a cluster restore
// brings back, by name. descriptor and namespace are handled on their own.
var restoredSystemTables = []catconstants.SystemTableName{
	catconstants.UsersTableName,
	catconstants.RoleMembersTableName,
	catconstants.RoleOptionsTableName,
	catconstants.DatabaseRoleSettingsTableName,
	catconstants.SystemPrivilegeTableName,
	catconstants.RoleIDSequenceName,
	catconstants.DescIDSequenceTableName,
	catconstants.SettingsTableName,
	catconstants.ZonesTableName,
	catconstants.CommentsTableName,
	catconstants.TableStatisticsTableName,
	catconstants.LocationsTableName,
	catconstants.UITableName,
	catconstants.SystemExternalConnectionsTableName,
}

// cacheTables are the system tables whose descriptor version the restore
// bumps, so that the caches of users, roles and privileges are reloaded.
var cacheTables = []catconstants.SystemTableName{
	catconstants.UsersTableName,
	catconstants.RoleMembersTableName,
	catconstants.RoleOptionsTableName,
	catconstants.DatabaseRoleSettingsTableName,
	catconstants.SystemPrivilegeTableName,
}

type restoreResumer struct {
	job *jobs.Job
}

var _ jobs.Resumer = (*restoreResumer)(nil)

// Resume implements jobs.Resumer.
func (r *restoreResumer) Resume(ctx context.Context, execCtx interface{}) error {
	if r.job.FractionCompleted() >= 1 {
		// The catalog was published before the job could finish.
		return nil
	}
	execCfg := execCtx.(sql.JobExecContext).ExecCfg()
	d := r.job.Details().(jobspb.ClusterRestoreDetails)
	store, err := OpenStore(ctx, Repository{Bucket: d.Bucket, Prefix: d.Prefix, Endpoint: d.Endpoint, Region: d.Region})
	if err != nil {
		return err
	}
	c, err := findChain(ctx, store, d.Target)
	if err != nil {
		return err
	}
	target := d.Target
	if target.IsEmpty() {
		target = c.end()
	}
	if v := execCfg.Settings.Version.ActiveVersion(ctx).String(); v != c.full.ClusterVersion {
		return errors.Newf("the backup has cluster version %s and this cluster %s; they must be the same",
			c.full.ClusterVersion, v)
	}
	if err := checkEmpty(ctx, execCfg); err != nil {
		return err
	}
	log.Infof(ctx, "restoring to %s from full backup %s and %d segments",
		FormatTime(target), c.full.Name, len(c.segments))

	dir, err := os.MkdirTemp(execCfg.DistSQLSrv.ServerConfig.TempStoragePath, "backup-restore-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	eng, err := load(ctx, execCfg.Settings, store, c, dir)
	if err != nil {
		return err
	}
	defer eng.Close()

	src, err := readCatalog(ctx, eng, target)
	if err != nil {
		return err
	}
	if err := ingestTables(ctx, execCfg, eng, src); err != nil {
		return err
	}
	return publish(ctx, execCfg, eng, src, r.job)
}

// OnFailOrCancel implements jobs.Resumer. The data written so far has no
// descriptors; restore again into a new empty cluster.
func (r *restoreResumer) OnFailOrCancel(context.Context, interface{}, error) error { return nil }

// CollectProfile implements jobs.Resumer.
func (r *restoreResumer) CollectProfile(context.Context, interface{}) error { return nil }

// checkEmpty returns an error if the cluster has user objects other than
// the pre-loaded defaultdb and postgres.
func checkEmpty(ctx context.Context, execCfg *sql.ExecutorConfig) error {
	return execCfg.InternalDB.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
		all, err := txn.Descriptors().GetAllDescriptors(ctx, txn.KV())
		if err != nil {
			return err
		}
		preloaded := map[descpb.ID]bool{}
		return all.ForEachDescriptor(func(desc catalog.Descriptor) error {
			if catalog.IsSystemDescriptor(desc) {
				return nil
			}
			switch {
			case desc.DescriptorType() == catalog.Database &&
				(desc.GetName() == catalogkeys.DefaultDatabaseName || desc.GetName() == catalogkeys.PgDatabaseName):
				preloaded[desc.GetID()] = true
				return nil
			case desc.DescriptorType() == catalog.Schema && desc.GetName() == catconstants.PublicSchemaName &&
				preloaded[desc.GetParentID()]:
				return nil
			}
			return errors.Newf("the cluster is not empty: it has %s %q; restore into a new cluster",
				desc.DescriptorType(), desc.GetName())
		})
	})
}

// load opens a Pebble engine in dir and ingests the files of chain c.
func load(
	ctx context.Context, st *cluster.Settings, s *Store, c chain, dir string,
) (storage.Engine, error) {
	eng, err := storage.Open(ctx, storage.Filesystem(filepath.Join(dir, "engine")), st,
		storage.CacheSize(64<<20))
	if err != nil {
		return nil, err
	}
	n := 0
	write := func(sst []byte) (string, error) {
		n++
		p := filepath.Join(dir, fmt.Sprintf("%08d.sst", n))
		return p, os.WriteFile(p, sst, 0600)
	}
	var paths []string
	for _, f := range c.full.Files {
		sst, err := readFullFile(ctx, s, c.full, f)
		if err != nil {
			eng.Close()
			return nil, err
		}
		p, err := write(sst)
		if err != nil {
			eng.Close()
			return nil, err
		}
		paths = append(paths, p)
	}
	if err := ingest(ctx, eng, paths); err != nil {
		eng.Close()
		return nil, err
	}
	for _, h := range c.segments {
		paths = paths[:0]
		if _, err := readSegment(ctx, s, h.Number, false, func(_ FileInfo, sst []byte) error {
			p, err := write(sst)
			paths = append(paths, p)
			return err
		}); err != nil {
			eng.Close()
			return nil, err
		}
		if err := ingest(ctx, eng, paths); err != nil {
			eng.Close()
			return nil, err
		}
	}
	return eng, nil
}

// ingest adds files that do not overlap each other to eng.
func ingest(ctx context.Context, eng storage.Engine, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	return errors.Wrap(eng.IngestLocalFiles(ctx, paths), "loading backup files")
}

// sourceCatalog is the catalog of the backup at the target time, and what
// the restore does with it.
type sourceCatalog struct {
	target hlc.Timestamp
	// restored are the descriptors to restore, changed as the restore needs.
	restored map[descpb.ID]catalog.MutableDescriptor
	// skipped are the user descriptors at the target time that are not
	// restored.
	skipped map[descpb.ID]bool
	// systemIDs maps the system tables by name to their IDs in the backup.
	systemIDs map[string]descpb.ID
	// readAt is the time to read a table's data at: before its import for
	// a table that an IMPORT INTO had taken offline, else the target.
	readAt map[descpb.ID]hlc.Timestamp
}

var codec = keys.SystemSQLCodec

func tableSpan(id uint32) roachpb.Span {
	p := codec.TablePrefix(id)
	return roachpb.Span{Key: p, EndKey: p.PrefixEnd()}
}

// readCatalog reads the namespace and the descriptors at target and
// decides what to restore.
func readCatalog(
	ctx context.Context, eng storage.Engine, target hlc.Timestamp,
) (*sourceCatalog, error) {
	src := &sourceCatalog{
		target:    target,
		restored:  map[descpb.ID]catalog.MutableDescriptor{},
		skipped:   map[descpb.ID]bool{},
		systemIDs: map[string]descpb.ID{},
		readAt:    map[descpb.ID]hlc.Timestamp{},
	}
	span := tableSpan(keys.NamespaceTableID)
	if _, err := storage.MVCCIterate(ctx, eng, span.Key, span.EndKey, target, storage.MVCCScanOptions{},
		func(kv roachpb.KeyValue) error {
			name, err := catalogkeys.DecodeNameMetadataKey(codec, kv.Key)
			if err != nil {
				return err
			}
			id, err := kv.Value.GetInt()
			if err != nil {
				return err
			}
			if name.ParentID == keys.SystemDatabaseID && name.ParentSchemaID == keys.SystemPublicSchemaID {
				src.systemIDs[name.Name] = descpb.ID(id)
			}
			return nil
		}); err != nil {
		return nil, errors.Wrap(err, "reading the namespace of the backup")
	}
	span = tableSpan(keys.DescriptorTableID)
	if _, err := storage.MVCCIterate(ctx, eng, span.Key, span.EndKey, target, storage.MVCCScanOptions{},
		func(kv roachpb.KeyValue) error {
			b, err := descbuilder.FromSerializedValue(&kv.Value)
			if err != nil || b == nil {
				return err
			}
			desc := b.BuildCreatedMutable()
			if catalog.IsSystemDescriptor(desc) {
				return nil
			}
			if !src.restore(desc) {
				src.skipped[desc.GetID()] = true
				return nil
			}
			src.restored[desc.GetID()] = desc
			return nil
		}); err != nil {
		return nil, errors.Wrap(err, "reading the descriptors of the backup")
	}
	return src, nil
}

// restore decides whether desc is restored and changes it as needed.
func (src *sourceCatalog) restore(desc catalog.MutableDescriptor) bool {
	switch {
	case desc.Dropped(), desc.Adding():
		return false
	case desc.Offline():
		tbl, ok := desc.(*tabledesc.Mutable)
		if !ok || tbl.ImportStartWallTime == 0 {
			return false
		}
		// IMPORT INTO had taken the table offline: bring it back with the
		// data from before the import.
		src.readAt[tbl.ID] = hlc.Timestamp{WallTime: tbl.ImportStartWallTime - 1}
		tbl.State = descpb.DescriptorState_PUBLIC
		tbl.OfflineReason = ""
		tbl.ImportStartWallTime = 0
	}
	return true
}

// ingestTables writes the data of the restored tables, as of the target
// time, with AddSSTable at a new timestamp.
func ingestTables(
	ctx context.Context, execCfg *sql.ExecutorConfig, eng storage.Engine, src *sourceCatalog,
) error {
	adder, err := execCfg.DistSQLSrv.ServerConfig.BulkAdder(ctx, execCfg.DB, execCfg.Clock.Now(),
		kvserverbase.BulkAdderOptions{Name: "restore", WriteAtBatchTimestamp: true})
	if err != nil {
		return err
	}
	defer adder.Close(ctx)
	for _, desc := range orderedByID(src.restored) {
		tbl, ok := desc.(*tabledesc.Mutable)
		if !ok || tbl.IsView() || tbl.IsVirtualTable() {
			continue
		}
		if tbl.GetExcludeDataFromBackup() {
			// The table comes back empty, as upstream's did.
			continue
		}
		readAt := src.target
		if t, ok := src.readAt[tbl.ID]; ok {
			readAt = t
		}
		for _, idx := range tbl.AllIndexes() {
			span := tbl.IndexSpan(codec, idx.GetID())
			if _, err := storage.MVCCIterate(ctx, eng, span.Key, span.EndKey, readAt, storage.MVCCScanOptions{},
				func(kv roachpb.KeyValue) error {
					return adder.Add(ctx, kv.Key, kv.Value.RawBytes)
				}); err != nil {
				return errors.Wrapf(err, "restoring the data of table %q", tbl.Name)
			}
		}
	}
	return adder.Flush(ctx)
}

func orderedIDs(m map[descpb.ID]catalog.MutableDescriptor) descpb.IDs {
	var ids descpb.IDs
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func orderedByID(m map[descpb.ID]catalog.MutableDescriptor) []catalog.MutableDescriptor {
	ids := orderedIDs(m)
	out := make([]catalog.MutableDescriptor, len(ids))
	for i, id := range ids {
		out[i] = m[id]
	}
	return out
}

// publish replaces the catalog and the restored system tables of this
// cluster with those of the backup, in one transaction.
func publish(
	ctx context.Context,
	execCfg *sql.ExecutorConfig,
	eng storage.Engine,
	src *sourceCatalog,
	job *jobs.Job,
) error {
	return execCfg.InternalDB.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
		col := txn.Descriptors()
		kvTxn := txn.KV()
		targetIDs, err := systemTableIDs(ctx, txn)
		if err != nil {
			return err
		}

		// Take out the pre-loaded databases and their names. Their IDs
		// may come back with the backup's descriptors; those then get a
		// higher version, so that leases on the old ones end.
		all, err := col.GetAllDescriptors(ctx, kvTxn)
		if err != nil {
			return err
		}
		oldVersions := map[descpb.ID]descpb.DescriptorVersion{}
		b := kvTxn.NewBatch()
		if err := all.ForEachDescriptor(func(desc catalog.Descriptor) error {
			if catalog.IsSystemDescriptor(desc) {
				return nil
			}
			oldVersions[desc.GetID()] = desc.GetVersion()
			b.Del(catalogkeys.MakeDescMetadataKey(codec, desc.GetID()))
			if desc.DescriptorType() != catalog.Schema || desc.GetName() != catconstants.PublicSchemaName {
				b.Del(catalogkeys.EncodeNameKey(codec, desc))
			} else {
				b.Del(catalogkeys.EncodeNameKey(codec, &descpb.NameInfo{
					ParentID: desc.GetParentID(), Name: desc.GetName(),
				}))
			}
			return nil
		}); err != nil {
			return err
		}
		if err := kvTxn.Run(ctx, b); err != nil {
			return err
		}

		// The namespace of the backup, without the system entries and the
		// names of objects that are not restored.
		if err := copyRows(ctx, kvTxn, eng, src.target, keys.NamespaceTableID, keys.NamespaceTableID,
			func(kv roachpb.KeyValue) bool {
				name, err := catalogkeys.DecodeNameMetadataKey(codec, kv.Key)
				if err != nil || name.ParentID == keys.SystemDatabaseID ||
					(name.ParentID == 0 && name.Name == catconstants.SystemDatabaseName) {
					return false
				}
				id, err := kv.Value.GetInt()
				return err == nil && !src.skipped[descpb.ID(id)]
			}, true /* keepExisting */); err != nil {
			return err
		}

		for _, name := range restoredSystemTables {
			srcID, ok := src.systemIDs[string(name)]
			if !ok {
				return errors.Newf("the backup has no system table %s", name)
			}
			keep, err := src.keepRow(name)
			if err != nil {
				return err
			}
			dstID, ok := targetIDs[string(name)]
			if !ok {
				return errors.Newf("this cluster has no system table %s", name)
			}
			if err := copyRows(ctx, kvTxn, eng, src.target, srcID, dstID, keep, false); err != nil {
				return errors.Wrapf(err, "restoring system.%s", name)
			}
		}

		// The descriptors are written as they are, then read back and
		// written again through the collection, which bumps their versions
		// and validates them. A pre-loaded ID that comes back gets a version
		// above its old one, so that leases on the old one end.
		b = kvTxn.NewBatch()
		for _, desc := range orderedByID(src.restored) {
			if v, ok := oldVersions[desc.GetID()]; ok && desc.GetVersion() < v {
				setVersion(desc, v)
			}
			b.Put(catalogkeys.MakeDescMetadataKey(codec, desc.GetID()), desc.DescriptorProto())
		}
		if err := kvTxn.Run(ctx, b); err != nil {
			return err
		}
		// Forget the catalog read before the writes above.
		col.ReleaseAll(ctx)
		mut := nstree.MutableCatalog{}
		var tables []*tabledesc.Mutable
		for _, id := range orderedIDs(src.restored) {
			desc, err := col.MutableByID(kvTxn).Desc(ctx, id)
			if err != nil {
				return err
			}
			mut.UpsertDescriptor(desc)
			if tbl, ok := desc.(*tabledesc.Mutable); ok {
				tables = append(tables, tbl)
			}
		}
		if _, err := scbackup.CreateDeclarativeSchemaChangeJobs(ctx, execCfg.JobRegistry, txn, mut.Catalog); err != nil {
			return err
		}
		if err := legacySchemaChangeJobs(ctx, execCfg, txn, tables); err != nil {
			return err
		}
		if err := ttlSchedules(ctx, execCfg, txn, tables); err != nil {
			return err
		}
		b = kvTxn.NewBatch()
		if err := mut.ForEachDescriptor(func(desc catalog.Descriptor) error {
			return col.WriteDescToBatch(ctx, false, desc.(catalog.MutableDescriptor), b)
		}); err != nil {
			return err
		}
		for _, name := range cacheTables {
			tbl, err := col.MutableByID(kvTxn).Table(ctx, targetIDs[string(name)])
			if err != nil {
				return err
			}
			if err := col.WriteDescToBatch(ctx, false, tbl, b); err != nil {
				return err
			}
		}
		if err := kvTxn.Run(ctx, b); err != nil {
			return err
		}
		if err := col.ValidateUncommittedDescriptors(ctx, kvTxn); err != nil {
			return err
		}
		return job.WithTxn(txn).FractionProgressed(ctx, jobs.FractionUpdater(1))
	})
}

func setVersion(desc catalog.MutableDescriptor, v descpb.DescriptorVersion) {
	switch d := desc.DescriptorProto().Union.(type) {
	case *descpb.Descriptor_Table:
		d.Table.Version = v
	case *descpb.Descriptor_Database:
		d.Database.Version = v
	case *descpb.Descriptor_Schema:
		d.Schema.Version = v
	case *descpb.Descriptor_Type:
		d.Type.Version = v
	case *descpb.Descriptor_Function:
		d.Function.Version = v
	}
}

// systemTableIDs returns the IDs of this cluster's system tables by name.
func systemTableIDs(ctx context.Context, txn descs.Txn) (map[string]descpb.ID, error) {
	ids := map[string]descpb.ID{}
	all, err := txn.Descriptors().GetAllDescriptors(ctx, txn.KV())
	if err != nil {
		return nil, err
	}
	err = all.ForEachDescriptor(func(desc catalog.Descriptor) error {
		if desc.GetParentID() == keys.SystemDatabaseID && desc.DescriptorType() == catalog.Table {
			ids[desc.GetName()] = desc.GetID()
		}
		return nil
	})
	return ids, err
}

// keepRow returns which rows of system table name the restore keeps, or
// nil for all.
func (src *sourceCatalog) keepRow(
	name catconstants.SystemTableName,
) (func(roachpb.KeyValue) bool, error) {
	// firstColumn decodes the first primary key column, an INT, of a key.
	firstColumn := func(kv roachpb.KeyValue, skip int) (int64, bool) {
		rest, _, _, err := codec.DecodeIndexPrefix(kv.Key)
		if err != nil {
			return 0, false
		}
		var v int64
		for i := 0; i <= skip; i++ {
			if rest, v, err = encoding.DecodeVarintAscending(rest); err != nil {
				return 0, false
			}
		}
		return v, true
	}
	// object keeps the rows of objects that are restored or not user
	// objects.
	object := func(skip int) func(roachpb.KeyValue) bool {
		return func(kv roachpb.KeyValue) bool {
			id, ok := firstColumn(kv, skip)
			return ok && !src.skipped[descpb.ID(id)]
		}
	}
	switch name {
	case catconstants.SettingsTableName:
		// The cluster version stays this cluster's.
		return func(kv roachpb.KeyValue) bool {
			rest, _, _, err := codec.DecodeIndexPrefix(kv.Key)
			if err != nil {
				return false
			}
			_, setting, err := encoding.DecodeUnsafeStringAscending(rest, nil)
			return err == nil && setting != "version"
		}, nil
	case catconstants.ZonesTableName, catconstants.TableStatisticsTableName:
		return object(0), nil
	case catconstants.CommentsTableName:
		// The key is (type, object_id, sub_id).
		return object(1), nil
	}
	return nil, nil
}

// copyRows replaces the rows of table dstID that keep accepts (all if
// keep is nil) with the rows of table srcID in the backup at target that
// keep accepts. The keys move to the prefix of dstID. With
// keepExisting, no row of dstID is deleted.
func copyRows(
	ctx context.Context,
	txn *kv.Txn,
	eng storage.Engine,
	target hlc.Timestamp,
	srcID, dstID descpb.ID,
	keep func(roachpb.KeyValue) bool,
	keepExisting bool,
) error {
	if keep == nil {
		keep = func(roachpb.KeyValue) bool { return true }
	}
	const batchSize = 1000
	b := txn.NewBatch()
	flush := func(force bool) error {
		if len(b.Results) == 0 && !force {
			return nil
		}
		if !force && b.ApproximateMutationBytes() < batchSize*100 {
			return nil
		}
		err := txn.Run(ctx, b)
		b = txn.NewBatch()
		return err
	}
	dst := tableSpan(uint32(dstID))
	if !keepExisting {
		rows, err := txn.Scan(ctx, dst.Key, dst.EndKey, 0)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if keep(roachpb.KeyValue{Key: row.Key, Value: *row.Value}) {
				b.Del(row.Key)
			}
		}
		if err := flush(true); err != nil {
			return err
		}
	}
	srcPrefix := codec.TablePrefix(uint32(srcID))
	dstPrefix := codec.TablePrefix(uint32(dstID))
	src := tableSpan(uint32(srcID))
	if _, err := storage.MVCCIterate(ctx, eng, src.Key, src.EndKey, target, storage.MVCCScanOptions{},
		func(kv roachpb.KeyValue) error {
			if !keep(kv) {
				return nil
			}
			key := append(dstPrefix[:len(dstPrefix):len(dstPrefix)], kv.Key[len(srcPrefix):]...)
			// The checksum covers the key, so a new one is made.
			v := roachpb.Value{RawBytes: append([]byte(nil), kv.Value.RawBytes...)}
			v.ClearChecksum()
			b.Put(key, &v)
			return flush(false)
		}); err != nil {
		return err
	}
	return flush(true)
}

// legacySchemaChangeJobs makes a job for each mutation of a restored table
// that the legacy schema changer was running at the target time.
func legacySchemaChangeJobs(
	ctx context.Context, execCfg *sql.ExecutorConfig, txn descs.Txn, tables []*tabledesc.Mutable,
) error {
	sd := sql.NewInternalSessionData(ctx, execCfg.Settings, "restore")
	var records []*jobs.Record
	for _, tbl := range tables {
		if len(tbl.MutationJobs) == 0 {
			continue
		}
		tbl.MutationJobs = nil
		seen := map[descpb.MutationID]bool{}
		for _, m := range tbl.Mutations {
			if m.MutationID == descpb.InvalidMutationID || seen[m.MutationID] {
				continue
			}
			seen[m.MutationID] = true
			id := execCfg.JobRegistry.MakeJobID()
			records = append(records, &jobs.Record{
				JobID:         id,
				Description:   fmt.Sprintf("restored schema change on table %q", tbl.Name),
				Username:      username.NodeUserName(),
				DescriptorIDs: descpb.IDs{tbl.ID},
				Details: jobspb.SchemaChangeDetails{
					DescID:          tbl.ID,
					TableMutationID: m.MutationID,
					FormatVersion:   jobspb.DatabaseJobFormatVersion,
					SessionData:     &sd.SessionData,
				},
				Progress: jobspb.SchemaChangeProgress{},
			})
			tbl.MutationJobs = append(tbl.MutationJobs, descpb.TableDescriptor_MutationJob{
				MutationID: m.MutationID, JobID: id,
			})
		}
	}
	_, err := execCfg.JobRegistry.CreateJobsWithTxn(ctx, txn, records)
	return err
}

// ttlSchedules makes a new row-level TTL schedule for each restored table
// that has a TTL, since schedules are not restored.
func ttlSchedules(
	ctx context.Context, execCfg *sql.ExecutorConfig, txn descs.Txn, tables []*tabledesc.Mutable,
) error {
	for _, tbl := range tables {
		if !tbl.HasRowLevelTTL() {
			continue
		}
		sj, err := sql.CreateRowLevelTTLScheduledJob(ctx, execCfg.JobsKnobs(), jobs.ScheduledJobTxn(txn),
			tbl.GetPrivileges().Owner(), tbl, execCfg.NodeInfo.LogicalClusterID(),
			execCfg.Settings.Version.ActiveVersion(ctx))
		if err != nil {
			return err
		}
		tbl.RowLevelTTL.ScheduleID = sj.ScheduleID()
	}
	return nil
}

func init() {
	jobs.RegisterConstructor(jobspb.TypeClusterRestore,
		func(job *jobs.Job, _ *cluster.Settings) jobs.Resumer {
			return &restoreResumer{job: job}
		},
		jobs.UsesTenantCostControl,
	)
}
