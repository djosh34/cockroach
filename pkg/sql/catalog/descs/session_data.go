// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package descs

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/internal/catkv"
)

// DescriptorSessionDataProvider is the union of various interfaces which
// expose descriptor-related state from session data.
type DescriptorSessionDataProvider interface {
	TemporarySchemaProvider
	DescriptorValidationModeProvider
}

// DescriptorValidationModeProvider exports the same interface in catkv.
type DescriptorValidationModeProvider = catkv.DescriptorValidationModeProvider

// TemporarySchemaProvider encapsulates the temporary schema information in
// the current session data.
type TemporarySchemaProvider interface {

	// HasTemporarySchema returns false iff the there is no temporary schemas in
	// the underlying session data.
	HasTemporarySchema() bool

	// GetTemporarySchemaName returns the name of the temporary schema for this
	// session according to the search path, if it exists.
	GetTemporarySchemaName() string

	// GetTemporarySchemaIDForDB returns the ID of the temporary schema for this
	// session for the given database ID.
	GetTemporarySchemaIDForDB(dbID descpb.ID) descpb.ID

	// MaybeGetDatabaseForTemporarySchemaID returns the ID of the parent database
	// of the temporary schema with the given ID, if it exists, 0 otherwise.
	MaybeGetDatabaseForTemporarySchemaID(id descpb.ID) descpb.ID
}
