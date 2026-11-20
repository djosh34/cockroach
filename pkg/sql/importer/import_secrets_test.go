// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package importer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestImportKeepsCredentialsSecret checks that IMPORT keeps the credentials in
// its URIs out of errors and job descriptions, and that the Avro schema_uri
// needs the same privileges as the data files.
func TestImportKeepsCredentialsSecret(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	const schema = `{"type": "record", "name": "r", "fields": [{"name": "a", "type": "long"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body string
		switch r.URL.Path {
		case "/schema.json":
			body = schema
		case "/bad.csv":
			body = "1,2,3\n"
		case "/many-bad.csv":
			body = strings.Repeat("x\n", 1100)
		default:
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing %s: %v", r.URL.Path, err)
		}
	}))
	defer srv.Close()
	withPassword := func(path string) string {
		u, err := url.Parse(srv.URL + path)
		require.NoError(t, err)
		u.User = url.UserPassword("u", "SECRETPW")
		return u.String()
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"a": 1}`+"\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(schema), 0644))

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{ExternalIODir: dir})
	defer s.Stopper().Stop(ctx)
	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE TABLE t (a INT8)`)

	t.Run("error", func(t *testing.T) {
		_, err := db.Exec(`IMPORT INTO t CSV DATA ($1)`, withPassword("/bad.csv"))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "SECRETPW")
	})

	t.Run("rejected rows error", func(t *testing.T) {
		_, err := db.Exec(`IMPORT INTO t CSV DATA ($1) WITH experimental_save_rejected`, withPassword("/many-bad.csv"))
		require.ErrorContains(t, err, "too many parsing errors")
		require.NotContains(t, err.Error(), "SECRETPW")
	})

	t.Run("job description", func(t *testing.T) {
		sqlDB.Exec(t, `IMPORT INTO t AVRO DATA ('nodelocal://1/data.json') WITH data_as_json_records, schema_uri = $1`,
			withPassword("/schema.json"))
		var desc string
		sqlDB.QueryRow(t, `SELECT description FROM [SHOW JOBS] WHERE job_type = 'IMPORT' ORDER BY created DESC LIMIT 1`).Scan(&desc)
		require.Contains(t, desc, "schema_uri")
		require.NotContains(t, desc, "SECRETPW")
	})

	t.Run("schema_uri privileges", func(t *testing.T) {
		sqlDB.Exec(t, `CREATE USER testuser`)
		sqlDB.Exec(t, `GRANT INSERT ON t TO testuser`)
		testuser := s.ApplicationLayer().SQLConn(t, serverutils.User("testuser"))
		_, err := testuser.Exec(`IMPORT INTO t AVRO DATA ('userfile:///data.json') WITH data_as_json_records, schema_uri = 'nodelocal://1/schema.json'`)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "EXTERNALIOIMPLICITACCESS"), "%v", err)
	})
}
