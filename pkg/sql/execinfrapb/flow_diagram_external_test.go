// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package execinfrapb_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
)

func TestPlanDiagramTableReaderWrapColumns(t *testing.T) {
	defer leaktest.AfterTest(t)()

	desc, err := sql.CreateTestTableDescriptor(
		context.Background(),
		1, 100,
		`CREATE TABLE test (
		  this_is_a_very_long_name_01 INT PRIMARY KEY,
		  this_is_a_very_long_name_02 INT,
		  this_is_a_very_long_name_03 INT,
		  this_is_a_very_long_name_04 INT,
		  this_is_a_very_long_name_05 INT,
		  this_is_a_very_long_name_06 INT,
		  this_is_a_very_long_name_07 INT,
		  this_is_a_very_long_name_08 INT,
		  this_is_a_very_long_name_09 INT,
		  this_is_a_very_long_name_10 INT,
		  this_is_a_very_long_name_11 INT,
		  this_is_a_very_long_name_12 INT,
		  this_is_a_very_long_name_13 INT,
		  this_is_a_very_long_name_14 INT,
		  this_is_a_very_long_name_15 INT,
		  this_is_a_very_long_name_16 INT,
		  x INT,
		  y INT,
		  this_is_a_super_duper_long_name_that_is_longer_than_any_reasonable_wrapping_limit_and_must_appear_on_its_own_line INT,
		  z INT
		)`,
		catpb.NewBasePrivilegeDescriptor(username.NodeUserName()),
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	flows := make(map[base.SQLInstanceID]*execinfrapb.FlowSpec)

	var tr execinfrapb.TableReaderSpec
	if err := rowenc.InitIndexFetchSpec(
		&tr.FetchSpec, keys.SystemSQLCodec, desc, desc.GetPrimaryIndex(),
		[]descpb.ColumnID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
	); err != nil {
		t.Fatal(err)
	}

	flows[1] = &execinfrapb.FlowSpec{
		Processors: []execinfrapb.ProcessorSpec{{
			Core: execinfrapb.ProcessorCoreUnion{TableReader: &tr},
			Output: []execinfrapb.OutputRouterSpec{{
				Type:    execinfrapb.OutputRouterSpec_PASS_THROUGH,
				Streams: []execinfrapb.StreamEndpointSpec{{Type: execinfrapb.StreamEndpointSpec_SYNC_RESPONSE}},
			}},
			StageID:     1,
			ProcessorID: 0,
		}},
	}

	_, url, err := execinfrapb.GeneratePlanDiagramURL("SOME SQL HERE", flows, execinfrapb.DiagramFlags{})
	if err != nil {
		t.Fatal(err)
	}
	expectedURL := "https://cockroachdb.github.io/distsqlplan/decode.html#eJyMU99v0z4Qf__-FdU9e_omGxuQJyRWiUqMQbs3VJ2O5tpac2zju9CVKv87cso0mMAlkU7KfX7cxfr4APLVQQOL25vpZPHp_eTddD4FAz60_IE6Fmg-Qw1LAzGFFYuElFuHkTBrH6CpDFgfe83tpYFVSAzNAdSqY2jgjr44njO1nP6vwEDLStZlMiiLvskF4z3vwcDb4PrOSzPRrRW0goTfOO3RBb9BTx1jVZsCeF4CLwwYKOAvSuLLEnh1wvllSfyqBL4uO9dVQVyXjqo-P-F8URKXjqq-POF8ZSYPZrJ_xpI-csJ2rE9k3ZLmFXKHE-qWPJLfY2KS4HOycJcoRus36GxnFcm32PWiSDEyJQwerQqGnUdnPY9Tv8NyMBB6fUqtKG0YmvqXmM-uoakG8-9Jn7PE4IV_C_nfJlXPJp3Vw9IAt5vxxh1AQp9W_DGF1cg9ft6OK4-NlkWPaJ3Hic78IySamLrj-ksDaxd2aFtooPr5nP2hPL6QBbSR_GOLbdiNtnf7mNdakxM2cEP3fM3KqbPeitoVNGtywsPw348BADHIgoU="
	if url.String() != expectedURL {
		t.Errorf("expected `%s` got `%s`", expectedURL, url.String())
	}
}
