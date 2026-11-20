// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package azure

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn"
	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn/connectionpb"
	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn/utils"
	"github.com/cockroachdb/errors"
)

func validateAzureConnectionURI(
	ctx context.Context, env externalconn.ExternalConnEnv, uri string,
) error {
	if err := utils.CheckExternalStorageConnection(ctx, env, uri); err != nil {
		return errors.Wrap(err, "failed to create azure external connection")
	}

	return nil
}

func init() {
	for _, s := range []string{scheme, deprecatedScheme, deprecatedExternalConnectionScheme} {
		externalconn.RegisterConnectionDetailsFromURIFactory(
			s,
			connectionpb.ConnectionProvider_azure_storage,
			externalconn.SimpleURIFactory,
		)

		externalconn.RegisterDefaultValidation(s, validateAzureConnectionURI)
	}
}
