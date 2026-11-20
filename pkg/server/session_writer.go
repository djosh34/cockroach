// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"bytes"
	"net/http"
)

// sessionWriter implements http.ResponseWriter. It is used
// to extract cookies written to the header when passed in to
// a ServeHTTP function.
type sessionWriter struct {
	header http.Header
	buf    bytes.Buffer
	code   int
}

var _ http.ResponseWriter = &sessionWriter{}

func (sw *sessionWriter) Header() http.Header {
	return sw.header
}

func (sw *sessionWriter) WriteHeader(statusCode int) {
	sw.code = statusCode
}

func (sw *sessionWriter) Write(data []byte) (int, error) {
	return sw.buf.Write(data)
}
