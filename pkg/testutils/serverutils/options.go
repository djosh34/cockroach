// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package serverutils

import (
	"net/url"

	"github.com/cockroachdb/cockroach/pkg/security/username"
)

// SQLConnOptions contains options for opening a SQL connection.
type SQLConnOptions struct {
	DBName         string
	User           *url.Userinfo
	ClientCerts    bool
	CertsDirPrefix string
}

// SQLConnOption is an option for opening a SQL connection.
type SQLConnOption func(result *SQLConnOptions)

// DefaultSQLConnOptions returns the default options for opening a SQL connection.
func DefaultSQLConnOptions() *SQLConnOptions {
	return &SQLConnOptions{
		DBName:         "",
		User:           url.User(username.RootUser),
		ClientCerts:    true,
		CertsDirPrefix: "openTestSQLConn",
	}
}

// DBName sets the database name to use when connecting to the server.
// Use catalogkeys.DefaultDatabaseName or catconstants.SystemDatabaseName when
// in doubt. An empty string results in DefaultDatabaseName being used.
func DBName(dbName string) SQLConnOption {
	return func(result *SQLConnOptions) {
		result.DBName = dbName
	}
}

// User sets the username to use when connecting to the server.
func User(username string) SQLConnOption {
	return func(result *SQLConnOptions) {
		result.User = url.User(username)
	}
}

// ClientCerts set whether the client certificates are loaded on-disk and in the
// connection URL.
func ClientCerts(clientCerts bool) SQLConnOption {
	return func(result *SQLConnOptions) {
		result.ClientCerts = clientCerts
	}
}

// CertsDirPrefix sets the prefix for the directory where the client certificates
// are stored.
func CertsDirPrefix(certsDirPrefix string) SQLConnOption {
	return func(result *SQLConnOptions) {
		result.CertsDirPrefix = certsDirPrefix
	}
}
