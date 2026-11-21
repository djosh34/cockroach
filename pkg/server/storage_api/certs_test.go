// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package storage_api_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
	"github.com/cockroachdb/cockroach/pkg/server/srvtestutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

func TestCertificatesResponse(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	srv := serverutils.StartServerOnly(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(context.Background())
	ts := srv.ApplicationLayer()

	var response serverpb.CertificatesResponse
	if err := srvtestutils.GetStatusJSONProto(ts, "certificates/local", &response); err != nil {
		t.Fatal(err)
	}

	// The test config names a node CA, a client CA, a node cert and a
	// client cert, in that order.
	want := []serverpb.CertificateDetails_CertificateType{
		serverpb.CertificateDetails_CA,
		serverpb.CertificateDetails_CLIENT_CA,
		serverpb.CertificateDetails_NODE,
		serverpb.CertificateDetails_CLIENT,
	}
	if a, e := len(response.Certificates), len(want); a != e {
		t.Fatalf("expected %d certificates, found %d", e, a)
	}
	for i, cert := range response.Certificates {
		if a, e := cert.Type, want[i]; a != e {
			t.Errorf("wrong type %s, expected %s", a, e)
		} else if cert.ErrorMessage != "" {
			t.Errorf("expected cert without error, got %v", cert.ErrorMessage)
		}
	}
}
