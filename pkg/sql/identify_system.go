// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/lsn"
	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/lsnutil"
	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/pgrepltree"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

type identifySystemNode struct {
	optColumnsSlot
	clusterID string
	database  string
	lsn       lsn.LSN
	shown     bool
}

func (s *identifySystemNode) startExec(params runParams) error {
	return nil
}

func (s *identifySystemNode) Next(params runParams) (bool, error) {
	if s.shown {
		return false, nil
	}
	s.shown = true
	return true, nil
}

func (s *identifySystemNode) Values() tree.Datums {
	db := tree.DNull
	if s.database != "" {
		db = tree.NewDString(s.database)
	}
	return tree.Datums{
		tree.NewDString(s.clusterID),
		tree.NewDInt(1), // timeline
		tree.NewDString(s.lsn.String()),
		db,
	}
}

func (s *identifySystemNode) Close(ctx context.Context) {}

func (p *planner) IdentifySystem(
	ctx context.Context, n *pgrepltree.IdentifySystem,
) (planNode, error) {
	return &identifySystemNode{
		// TODO(#105130): correctly populate this field.
		lsn:       lsnutil.HLCToLSN(p.Txn().ReadTimestamp()),
		clusterID: p.ExecCfg().NodeInfo.LogicalClusterID().String(),
		database:  p.SessionData().Database,
	}, nil
}
