// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package amazon

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/logtags"
)

// awsConfigParams holds what loadAWSConfig needs from an S3 or KMS URI.
type awsConfigParams struct {
	auth, accessKey, secret, tempToken string
	assumeRoleProvider                 roleProvider
	delegateRoleProviders              []roleProvider
	// httpClient, if set, replaces the SDK's HTTP client.
	httpClient *http.Client
	verbose    bool
}

// loadAWSConfig returns the SDK configuration for p: credentials from the URI
// or, with implicit auth, from the node's environment, wrapped in the chain of
// roles to assume.
func loadAWSConfig(ctx context.Context, p awsConfigParams) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRetryer(newRetryer),
		config.WithLogger(newLogAdapter(ctx)),
	}
	if p.httpClient != nil {
		opts = append(opts, config.WithHTTPClient(p.httpClient))
	}
	if p.verbose {
		opts = append(opts, config.WithClientLogMode(aws.LogRetries|aws.LogResponse))
	}
	switch p.auth {
	case "", cloud.AuthParamSpecified:
		// Only the credentials in the URI count: the node's AWS files are not
		// read.
		opts = append(opts,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(p.accessKey, p.secret, p.tempToken)),
			config.WithSharedConfigFiles([]string{}),
			config.WithSharedCredentialsFiles([]string{}),
		)
	case cloud.AuthParamImplicit:
	default:
		return aws.Config{}, errors.Errorf("unsupported value %s for %s", p.auth, cloud.AuthParam)
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, errors.Wrap(err, "loading aws config")
	}
	if p.assumeRoleProvider.roleARN != "" {
		// Each role in the chain gets its credentials from the one before.
		for _, delegate := range p.delegateRoleProviders {
			cfg.Credentials = assumeRole(cfg, delegate)
		}
		cfg.Credentials = assumeRole(cfg, p.assumeRoleProvider)
	}
	return cfg, nil
}

// assumeRole returns credentials for role, obtained with the credentials of
// cfg.
func assumeRole(cfg aws.Config, role roleProvider) aws.CredentialsProvider {
	client := sts.NewFromConfig(cfg, func(o *sts.Options) {
		// Without a region, use the global endpoint as the first SDK did.
		if o.Region == "" {
			o.Region = "us-east-1"
		}
	})
	return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(client, role.roleARN,
		func(o *stscreds.AssumeRoleOptions) {
			if role.externalID != "" {
				o.ExternalID = aws.String(role.externalID)
			}
		}))
}

// newRetryer retries a request up to 10 times, also when reading its
// response hit a connection reset. All operations this package sends are
// idempotent, so that is safe.
func newRetryer() aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = 11
		o.RateLimiter = ratelimit.None
		o.Retryables = append(o.Retryables, retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
			if isErrReadConnectionReset(err) {
				return aws.TrueTernary
			}
			return aws.UnknownTernary
		}))
	})
}

// isErrReadConnectionReset returns true if the underlying error is a read
// connection reset error, which the SDK does not retry by itself since it
// does not know whether the request is idempotent.
func isErrReadConnectionReset(err error) bool {
	return err != nil && strings.Contains(err.Error(), "read: connection reset")
}

type awsLogAdapter struct {
	ctx context.Context
}

var _ logging.Logger = (*awsLogAdapter)(nil)

// Logf implements the logging.Logger interface.
func (l *awsLogAdapter) Logf(_ logging.Classification, format string, v ...interface{}) {
	log.Infof(l.ctx, "aws: %s", fmt.Sprintf(format, v...))
}

func newLogAdapter(ctx context.Context) *awsLogAdapter {
	return &awsLogAdapter{
		ctx: logtags.AddTags(context.Background(), logtags.FromContext(ctx)),
	}
}
