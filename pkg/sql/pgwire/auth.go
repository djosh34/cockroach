// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgwire

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgwirebase"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondatapb"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
)

// authOK is the pgwire auth response code for successful authentication
// during the connection handshake.
const authOK int32 = 0

type authOptions struct {
	// insecure indicates that all connections for existing users must
	// be allowed to go through.
	insecure bool
	// connType is the actual type of client connection (e.g. local,
	// hostssl, hostnossl).
	connType ConnType
	// connDetails is the event payload common to all auth/session events.
	connDetails eventpb.CommonConnectionDetails

	// The following fields are only used by tests.

	// testingSkipAuth requires to skip authentication.
	testingSkipAuth bool
	// testingAuthHook, if provided, replaces the logic in
	// handleAuthentication().
	testingAuthHook func(ctx context.Context) error
}

// Names of the ways a connection can authenticate, for the logs.
const (
	authMethodCert     = "cert"
	authMethodInsecure = "insecure"
	authMethodLoopback = "loopback"
)

// handleAuthentication checks the connection's user. Errors are sent to the
// client and also returned.
//
// In secure mode a client logs in only with a TLS client certificate. The
// TLS handshake has checked that it chains to the client CA. The user the
// client asks for must be the role of a client rule that the certificate
// matches. In insecure mode, and on the in-process loopback listener, any
// existing user may log in.
func (c *conn) handleAuthentication(
	ctx context.Context, ac AuthConn, authOpt authOptions, execCfg *sql.ExecutorConfig,
) error {
	if authOpt.testingSkipAuth {
		return nil
	}
	if authOpt.testingAuthHook != nil {
		return authOpt.testingAuthHook(ctx)
	}

	dbUser := c.sessionArgs.User
	// Tests may pass a system_identity option; otherwise the system identity
	// is the user.
	if c.sessionArgs.SystemIdentity.Undefined() {
		c.sessionArgs.SystemIdentity = dbUser
	}
	var certExpiration int64
	switch {
	case authOpt.insecure:
		ac.SetAuthMethod(authMethodInsecure)
	case authOpt.connType == ConnInternalLoopback:
		ac.SetAuthMethod(authMethodLoopback)
	default:
		ac.SetAuthMethod(authMethodCert)
		expiration, err := c.checkClientCert(ctx, execCfg, authOpt.connType, ac)
		if err != nil {
			ac.LogAuthFailed(ctx, eventpb.AuthFailReason_CREDENTIALS_INVALID, err)
			return c.sendError(ctx, pgerror.WithCandidateCode(err, pgcode.InvalidAuthorizationSpecification))
		}
		certExpiration = expiration
	}
	ac.SetDbUser(dbUser)

	// Check that the requested user exists and may log in.
	exists, canLoginSQL, _, canUseReplicationMode, isSuperuser, defaultSettings, err :=
		sql.GetUserSessionInitInfo(
			ctx,
			execCfg,
			dbUser,
			c.sessionArgs.SessionDefaults["database"],
		)
	if err != nil {
		log.Warningf(ctx, "user retrieval failed for user=%q: %+v", dbUser, err)
		ac.LogAuthFailed(ctx, eventpb.AuthFailReason_USER_RETRIEVAL_ERROR, err)
		return c.sendError(ctx, pgerror.WithCandidateCode(err, pgcode.InvalidAuthorizationSpecification))
	}
	c.sessionArgs.IsSuperuser = isSuperuser

	if !exists {
		ac.LogAuthFailed(ctx, eventpb.AuthFailReason_USER_NOT_FOUND, nil)
		return c.sendError(ctx, pgerror.Newf(pgcode.InvalidAuthorizationSpecification,
			"user %s does not exist", dbUser))
	}

	if !canLoginSQL {
		ac.LogAuthFailed(ctx, eventpb.AuthFailReason_LOGIN_DISABLED, nil)
		return c.sendError(ctx, pgerror.Newf(pgcode.InvalidAuthorizationSpecification, "%s does not have login privilege", dbUser))
	}

	if certExpiration != 0 {
		if cm, err := execCfg.RPCContext.SecurityContext.GetCertificateManager(); err == nil {
			cm.MaybeUpsertClientExpiration(ctx, dbUser, certExpiration)
		}
	}

	// Add all the defaults to this session's defaults. If there is an
	// error (e.g., a setting that no longer exists, or bad input),
	// log a warning instead of preventing login.
	// The defaultSettings array is ordered by precedence. This means that if
	// SessionDefaults already has an entry for a given setting name, then
	// it should not be replaced.
	for _, settingEntry := range defaultSettings {
		for _, setting := range settingEntry.Settings {
			keyVal := strings.SplitN(setting, "=", 2)
			if len(keyVal) != 2 {
				log.Ops.Warningf(ctx, "%s has malformed default setting: %q", dbUser, setting)
				continue
			}
			if err := sql.CheckSessionVariableValueValid(ctx, execCfg.Settings, keyVal[0], keyVal[1]); err != nil {
				log.Ops.Warningf(ctx, "%s has invalid default setting: %v", dbUser, err)
				continue
			}
			if _, ok := c.sessionArgs.SessionDefaults[keyVal[0]]; !ok {
				c.sessionArgs.SessionDefaults[keyVal[0]] = keyVal[1]
			}
		}
	}

	// Check replication privilege.
	if c.sessionArgs.SessionDefaults["replication"] != "" {
		m, err := sql.ReplicationModeFromString(c.sessionArgs.SessionDefaults["replication"])
		if err == nil && m != sessiondatapb.ReplicationMode_REPLICATION_MODE_DISABLED && !canUseReplicationMode {
			ac.LogAuthFailed(ctx, eventpb.AuthFailReason_NO_REPLICATION_ROLEOPTION, nil)
			return c.sendError(
				ctx,
				pgerror.Newf(
					pgcode.InsufficientPrivilege,
					"must be superuser or have REPLICATION role to start streaming replication",
				),
			)
		}
	}

	return nil
}

// checkClientCert checks that the client certificate maps to the requested
// user through the client rules. It returns the certificate's expiration
// time in Unix seconds.
func (c *conn) checkClientCert(
	ctx context.Context, execCfg *sql.ExecutorConfig, connType ConnType, ac AuthConn,
) (int64, error) {
	if connType != ConnHostSSL {
		return 0, errors.New("a secure cluster admits SQL clients only over TLS with a client certificate")
	}
	tlsConn, ok := c.conn.(*readTimeoutConn).Conn.(*tls.Conn)
	if !ok {
		return 0, errors.AssertionFailedf("server reports hostssl conn without TLS state")
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return 0, errors.New("no client certificate")
	}
	cm, err := execCfg.RPCContext.SecurityContext.GetCertificateManager()
	if err != nil {
		return 0, err
	}
	if identity, err := username.MakeSQLUsernameFromUserInput(
		certs[0].Subject.CommonName, username.PurposeValidation); err == nil {
		ac.SetSystemIdentity(identity)
	}
	ac.LogAuthInfof(ctx, redact.Sprintf("client certificate %s", certconfig.DescribeCert(certs[0])))
	roles, err := cm.ClientRoles(certs)
	if err != nil {
		return 0, err
	}
	for _, r := range roles {
		if r == c.sessionArgs.User {
			return certs[0].NotAfter.Unix(), nil
		}
	}
	return 0, errors.Newf("no client rule maps certificate %s to user %s",
		redact.Safe(certconfig.DescribeCert(certs[0])), c.sessionArgs.User)
}

func (c *conn) authOKMessage() error {
	c.msgBuilder.initMsg(pgwirebase.ServerMsgAuth)
	c.msgBuilder.putInt32(authOK)
	return c.msgBuilder.finishMsg(c.conn)
}

// authenticatorIO is the interface used by the connection to wait for an
// authentication decision.
type authenticatorIO interface {
	// authResult blocks for an authentication decision.
	authResult() error
}

// AuthConn is the interface used by the authenticator for interacting with the
// pgwire connection.
type AuthConn interface {
	// AuthOK declares that authentication succeeded, for
	// authenticator.authResult().
	AuthOK(context.Context)
	// AuthFail declares that authentication has failed and provides an error to
	// be returned by authenticator.authResult(). The error has already been
	// written to the client connection.
	AuthFail(err error)

	// SetAuthMethod sets the authentication method for subsequent
	// logging messages.
	SetAuthMethod(method string)
	// SetDbUser updates the AuthConn with the actual database username
	// the connection has authenticated to.
	SetDbUser(dbUser username.SQLUsername)
	// SetSystemIdentity updates the AuthConn with the identity of the
	// client certificate.
	SetSystemIdentity(systemIdentity username.SQLUsername)
	// LogAuthInfof logs details about the progress of the
	// authentication.
	LogAuthInfof(ctx context.Context, msg redact.RedactableString)
	// LogAuthFailed logs details about an authentication failure.
	LogAuthFailed(ctx context.Context, reason eventpb.AuthFailReason, err error)
	// LogAuthOK logs when the authentication handshake has completed.
	LogAuthOK(ctx context.Context)
	// LogSessionEnd logs when the session is ended.
	LogSessionEnd(ctx context.Context, endTime time.Time)
	// GetTenantSpecificMetrics returns the tenant-specific metrics for the connection.
	GetTenantSpecificMetrics() *tenantSpecificMetrics
}

// authPipe is the implementation for the authenticator and AuthConn interfaces.
// A single authPipe will serve as both an AuthConn and an authenticator; the
// two represent the two "ends" of the pipe and we'll pass data between them.
type authPipe struct {
	c             *conn // Only used for writing, not for reading.
	log           bool
	loggedFailure bool

	connDetails eventpb.CommonConnectionDetails
	authDetails eventpb.CommonSessionDetails
	authMethod  string

	readerDone chan authRes
}

type authRes struct {
	err error
}

func newAuthPipe(
	c *conn, logAuthn bool, authOpt authOptions, systemIdentity username.SQLUsername,
) *authPipe {
	ap := &authPipe{
		c:           c,
		log:         logAuthn,
		connDetails: authOpt.connDetails,
		authDetails: eventpb.CommonSessionDetails{
			SystemIdentity: systemIdentity.Normalized(),
			Transport:      authOpt.connType.String(),
		},
		readerDone: make(chan authRes, 1),
	}
	return ap
}

var _ authenticatorIO = &authPipe{}
var _ AuthConn = &authPipe{}

// AuthOK is part of the AuthConn interface.
func (p *authPipe) AuthOK(ctx context.Context) {
	p.readerDone <- authRes{err: nil}
}

func (p *authPipe) AuthFail(err error) {
	p.readerDone <- authRes{err: err}
}

func (p *authPipe) SetAuthMethod(method string) {
	p.authMethod = method
}

func (p *authPipe) SetDbUser(dbUser username.SQLUsername) {
	p.authDetails.User = dbUser.Normalized()
}

func (p *authPipe) SetSystemIdentity(systemIdentity username.SQLUsername) {
	p.authDetails.SystemIdentity = systemIdentity.Normalized()
}

func (p *authPipe) LogAuthOK(ctx context.Context) {
	// Logged unconditionally.
	ev := &eventpb.ClientAuthenticationOk{
		CommonConnectionDetails: p.connDetails,
		CommonSessionDetails:    p.authDetails,
		Method:                  p.authMethod,
	}
	log.StructuredEvent(ctx, ev)
}

func (p *authPipe) LogAuthInfof(ctx context.Context, msg redact.RedactableString) {
	if p.log {
		ev := &eventpb.ClientAuthenticationInfo{
			CommonConnectionDetails: p.connDetails,
			CommonSessionDetails:    p.authDetails,
			Info:                    msg,
			Method:                  p.authMethod,
		}
		log.StructuredEvent(ctx, ev)
	}
}

func (p *authPipe) LogSessionEnd(ctx context.Context, endTime time.Time) {
	// Logged unconditionally.
	ev := &eventpb.ClientSessionEnd{
		CommonEventDetails:      logpb.CommonEventDetails{Timestamp: endTime.UnixNano()},
		CommonConnectionDetails: p.connDetails,
		Duration:                endTime.Sub(p.c.startTime).Nanoseconds(),
	}
	log.StructuredEvent(ctx, ev)
}

func (p *authPipe) LogAuthFailed(
	ctx context.Context, reason eventpb.AuthFailReason, detailedErr error,
) {
	if p.log && !p.loggedFailure {
		// If a failure was already logged, then don't log another one. The
		// assumption is that if an error is logged deeper in the call stack, the
		// reason is likely to be more specific than at a higher point in the stack.
		p.loggedFailure = true
		var errStr redact.RedactableString
		if detailedErr != nil {
			errStr = redact.Sprint(detailedErr)
		}
		ev := &eventpb.ClientAuthenticationFailed{
			CommonConnectionDetails: p.connDetails,
			CommonSessionDetails:    p.authDetails,
			Reason:                  reason,
			Detail:                  errStr,
			Method:                  p.authMethod,
		}
		log.StructuredEvent(ctx, ev)
	}
}

// authResult is part of the authenticator interface.
func (p *authPipe) authResult() error {
	res := <-p.readerDone
	return res.err
}

// GetTenantSpecificMetrics is part of the AuthConn interface.
func (p *authPipe) GetTenantSpecificMetrics() *tenantSpecificMetrics {
	return p.c.metrics
}
