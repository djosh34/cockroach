// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package externalconn

import (
	"context"
	"net/url"

	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn/connectionpb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/errors"
)

func makeExternalConnectionKMS(
	ctx context.Context, uri string, env cloud.KMSEnv,
) (cloud.KMS, error) {
	kmsURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	if kmsURI.Host == "" {
		return nil, errors.Newf("host component of an external URI must refer to an "+
			"existing External Connection object: %s", kmsURI.String())
	}
	externalConnectionName := kmsURI.Host

	// Retrieve the external connection object from the system table, once the
	// user is known to hold USAGE on it.
	var ec ExternalConnection
	if err := env.DBHandle().Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		if err := CheckUsage(ctx, txn, env.User(), externalConnectionName); err != nil {
			return err
		}
		var err error
		ec, err = LoadExternalConnection(ctx, externalConnectionName, txn)
		return err
	}); err != nil {
		return nil, errors.Wrap(err, "failed to load external connection object")
	}

	// Sanity check that we are connecting to a KMS object.
	if ec.ConnectionType() != connectionpb.TypeKMS {
		return nil, errors.Newf("KMS cannot use object of type %s", ec.ConnectionType().String())
	}

	// Construct a KMS handle for the underlying resource represented by the
	// external connection object.
	switch d := ec.ConnectionProto().Details.(type) {
	case *connectionpb.ConnectionDetails_SimpleURI:
		return cloud.KMSFromURI(ctx, d.SimpleURI.URI, env)
	default:
		return nil, errors.Newf("cannot connect to %T; unsupported resource for a KMS connection", d)
	}
}

func init() {
	cloud.RegisterKMSFromURIFactory(makeExternalConnectionKMS, scheme)
}
