// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package randgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

// TestStartTemplateIsNotStale checks that the hard-coded start template
// is up to date and the idempotent RunPostDeserializationChanges does
// not change it.
func TestStartTemplateIsNotStale(t *testing.T) {
	initialProto := startDescriptor()
	{
		// Populate the template with dummy values to pass validation.
		initialProto.ID = 123
		initialProto.ParentID = 456
		initialProto.UnexposedParentSchemaID = 789
		initialProto.Name = "test"
		initialProto.PrimaryIndex.Name = tabledesc.PrimaryKeyIndexName(initialProto.Name)
	}
	b := tabledesc.NewBuilder(&initialProto)
	require.NoError(t, b.RunPostDeserializationChanges())
	tbl := b.BuildCreatedMutableTable()
	require.NoError(t, desctestutils.TestingValidateSelf(tbl))
	if finalProto := tbl.TableDescriptor; !reflect.DeepEqual(initialProto, finalProto) {
		diff := strings.Join(pretty.Diff(initialProto, finalProto), "\n")
		t.Fatalf("Descriptor protobufs should be equal but aren't:\n%s", diff)
	}
}
