// Copyright 2023 The Cockroach Authors.
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
	"net/http"
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/multitenant"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

const (
	// TenantSelectCookieName is the name of the HTTP cookie used to select a
	// particular tenant, if the custom header is not specified.
	TenantSelectCookieName = `tenant`

	// TenantSelectHeader is the HTTP header used to select a particular tenant.
	TenantSelectHeader = `X-Cockroach-Tenant`

	// ClusterNameParamInQueryURL is the HTTP query URL parameter used
	// to select a particular virtual cluster.
	ClusterNameParamInQueryURL = "cluster"

	NoFallbackParam = "nofallback"

	// AcceptHeader is the canonical header name for accept.
	AcceptHeader = "Accept"

	// ContentTypeHeader is the canonical header name for content type.
	ContentTypeHeader = "Content-Type"

	// JSONContentType is the JSON content type.
	JSONContentType = "application/json"
)

// httpMux redirects incoming HTTP requests to the server selected by
// the special HTTP request header.
// If no tenant is specified, the default tenant is used.
func (c *serverController) httpMux(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantName := getTenantNameFromHTTPRequest(c.st, r)
	noFallback := false
	if noFallbackValue := r.URL.Query().Get(NoFallbackParam); noFallbackValue != "" {
		noFallback, _ = strconv.ParseBool(noFallbackValue)
	}

	s, _, err := c.getServer(ctx, tenantName)
	if err != nil {
		if noFallback {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		log.Warningf(ctx, "unable to find server for tenant %q: %v", tenantName, err)
		// Clear the tenant cookie since it appears to reference invalid state.
		http.SetCookie(w, &http.Cookie{
			Name:     TenantSelectCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: false,
			Secure:   !c.disableTLSForHTTP,
			Expires:  timeutil.Unix(0, 0),
		})
		// Fall back to serving requests from the default tenant.
		defaultTenantName := roachpb.TenantName(multitenant.DefaultTenantSelect.Get(&c.st.SV))
		s, _, err = c.getServer(ctx, defaultTenantName)
		if err != nil {
			log.Warningf(ctx, "unable to find server for default tenant %q: %v", defaultTenantName, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.getHTTPHandlerFn()(w, r)
		return
	}
	s.getHTTPHandlerFn()(w, r)
}

func getTenantNameFromHTTPRequest(st *cluster.Settings, r *http.Request) roachpb.TenantName {
	// Highest priority is manual override on the URL query parameters.
	if tenantName := r.URL.Query().Get(ClusterNameParamInQueryURL); tenantName != "" {
		return roachpb.TenantName(tenantName)
	}

	// If not in parameters, try an explicit header.
	if tenantName := r.Header.Get(TenantSelectHeader); tenantName != "" {
		return roachpb.TenantName(tenantName)
	}

	// No parameter, no explicit header. Is there a cookie?
	if c, _ := r.Cookie(TenantSelectCookieName); c != nil && c.Value != "" {
		return roachpb.TenantName(c.Value)
	}

	// No luck so far. Use the configured default.
	return roachpb.TenantName(multitenant.DefaultTenantSelect.Get(&st.SV))
}
