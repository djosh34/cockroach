// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package catsessiondata

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondatapb"
)

type sessionData sessiondata.SessionData
type sessionDataStack sessiondata.Stack

func (sds *sessionDataStack) top() *sessionData {
	if sds == nil {
		return nil
	}
	return (*sessionData)((*sessiondata.Stack)(sds).Top())
}

// DefaultDescriptorSessionDataProvider is the default implementation of
// descs.DescriptorSessionDataProvider
var DefaultDescriptorSessionDataProvider descs.DescriptorSessionDataProvider = (*sessionData)(nil)
var _ descs.DescriptorSessionDataProvider = (*sessionDataStack)(nil)

// NewDescriptorSessionDataProvider constructs a
// descs.DescriptorSessionDataProvider instance given session data.
// The pointer may be nil.
func NewDescriptorSessionDataProvider(
	sd *sessiondata.SessionData,
) descs.DescriptorSessionDataProvider {
	return (*sessionData)(sd)
}

// NewDescriptorSessionDataStackProvider constructs a
// descs.DescriptorSessionDataProvider instance given a pointer to a session
// data stack. The pointer may be nil.
func NewDescriptorSessionDataStackProvider(
	sds *sessiondata.Stack,
) descs.DescriptorSessionDataProvider {
	return (*sessionDataStack)(sds)
}

// HasTemporarySchema implements descs.TemporarySchemaProvider.
func (sd *sessionData) HasTemporarySchema() bool {
	return sd != nil
}

// HasTemporarySchema implements descs.TemporarySchemaProvider.
func (sds *sessionDataStack) HasTemporarySchema() bool {
	return sds.top() != nil
}

// GetTemporarySchemaName implements descs.TemporarySchemaProvider.
func (sd *sessionData) GetTemporarySchemaName() string {
	if sd == nil {
		return ""
	}
	return (*sessiondata.SessionData)(sd).SearchPath.GetTemporarySchemaName()
}

// GetTemporarySchemaName implements descs.TemporarySchemaProvider.
func (sds *sessionDataStack) GetTemporarySchemaName() string {
	return sds.top().GetTemporarySchemaName()
}

// GetTemporarySchemaIDForDB implements descs.TemporarySchemaProvider.
func (sd *sessionData) GetTemporarySchemaIDForDB(dbID descpb.ID) descpb.ID {
	if sd == nil {
		return descpb.InvalidID
	}
	ret, _ := (*sessiondata.SessionData)(sd).GetTemporarySchemaIDForDB(uint32(dbID))
	return descpb.ID(ret)
}

// GetTemporarySchemaIDForDB implements descs.TemporarySchemaProvider.
func (sds *sessionDataStack) GetTemporarySchemaIDForDB(dbID descpb.ID) descpb.ID {
	return sds.top().GetTemporarySchemaIDForDB(dbID)
}

// MaybeGetDatabaseForTemporarySchemaID implements descs.TemporarySchemaProvider.
func (sd *sessionData) MaybeGetDatabaseForTemporarySchemaID(id descpb.ID) descpb.ID {
	if sd == nil {
		return descpb.InvalidID
	}
	ret, _ := (*sessiondata.SessionData)(sd).MaybeGetDatabaseForTemporarySchemaID(uint32(id))
	return descpb.ID(ret)
}

// MaybeGetDatabaseForTemporarySchemaID implements descs.TemporarySchemaProvider.
func (sds *sessionDataStack) MaybeGetDatabaseForTemporarySchemaID(id descpb.ID) descpb.ID {
	return sds.top().MaybeGetDatabaseForTemporarySchemaID(id)
}

// ValidateDescriptorsOnRead implements descs.DescriptorValidationModeProvider.
func (sd *sessionData) ValidateDescriptorsOnRead() bool {
	var mode sessiondatapb.DescriptorValidationMode
	if sd != nil {
		mode = sd.DescriptorValidationMode
	}
	return mode != sessiondatapb.DescriptorValidationOff
}

// ValidateDescriptorsOnRead implements descs.DescriptorValidationModeProvider.
func (sds *sessionDataStack) ValidateDescriptorsOnRead() bool {
	return sds.top().ValidateDescriptorsOnRead()
}

// ValidateDescriptorsOnWrite implements descs.DescriptorValidationModeProvider.
func (sd *sessionData) ValidateDescriptorsOnWrite() bool {
	var mode sessiondatapb.DescriptorValidationMode
	if sd != nil {
		mode = sd.DescriptorValidationMode
	}
	return mode == sessiondatapb.DescriptorValidationOn
}

// ValidateDescriptorsOnWrite implements descs.DescriptorValidationModeProvider.
func (sds *sessionDataStack) ValidateDescriptorsOnWrite() bool {
	return sds.top().ValidateDescriptorsOnWrite()
}
