// Copyright 2020 The Cockroach Authors.
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
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/errors"
)

const (
	awsScheme    = "aws"
	awsKMSScheme = "aws-kms"
)

type awsKMS struct {
	kms                 *kms.Client
	customerMasterKeyID string
}

var _ cloud.KMS = &awsKMS{}

func init() {
	cloud.RegisterKMSFromURIFactory(MakeAWSKMS, awsScheme, awsKMSScheme)
}

type kmsURIParams struct {
	customerMasterKeyID   string
	accessKey             string
	secret                string
	tempToken             string
	endpoint              string
	region                string
	auth                  string
	roleProvider          roleProvider
	delegateRoleProviders []roleProvider
	verbose               bool
}

var reuseKMSSession = settings.RegisterBoolSetting(
	settings.ApplicationLevel,
	"cloudstorage.aws.reuse_kms_session.enabled",
	"persist the last opened AWS KMS session and reuse it when opening a new session with the same arguments",
	util.ConstantWithMetamorphicTestBool("aws-reuse-kms", true),
)

var kmsClientCache struct {
	syncutil.Mutex
	key kmsURIParams
	kms *awsKMS
}

func resolveKMSURIParams(kmsURI cloud.ConsumeURL) (kmsURIParams, error) {
	assumeRoleProto, delegateRoleProtos := cloud.ParseRoleProvidersString(kmsURI.ConsumeParam(AssumeRoleParam))
	assumeRoleProvider := makeRoleProvider(assumeRoleProto)
	delegateProviders := make([]roleProvider, len(delegateRoleProtos))
	for i := range delegateRoleProtos {
		delegateProviders[i] = makeRoleProvider(delegateRoleProtos[i])
	}

	params := kmsURIParams{
		customerMasterKeyID:   strings.TrimPrefix(kmsURI.Path, "/"),
		accessKey:             kmsURI.ConsumeParam(AWSAccessKeyParam),
		secret:                kmsURI.ConsumeParam(AWSSecretParam),
		tempToken:             kmsURI.ConsumeParam(AWSTempTokenParam),
		endpoint:              kmsURI.ConsumeParam(AWSEndpointParam),
		region:                kmsURI.ConsumeParam(KMSRegionParam),
		auth:                  kmsURI.ConsumeParam(cloud.AuthParam),
		roleProvider:          assumeRoleProvider,
		delegateRoleProviders: delegateProviders,
		verbose:               log.V(2),
	}

	// Validate that all the passed in parameters are supported.
	if unknownParams := kmsURI.RemainingQueryParams(); len(unknownParams) > 0 {
		return kmsURIParams{}, errors.Errorf(
			`unknown KMS query parameters: %s`, strings.Join(unknownParams, ", "))
	}

	// AWS secrets often contain + characters, which must be escaped when
	// included in a query string; otherwise, they represent a space character.
	// More than a few users have been bitten by this.
	//
	// Luckily, AWS secrets are base64-encoded data and thus will never actually
	// contain spaces. We can convert any space characters we see to +
	// characters to recover the original secret.
	params.secret = strings.Replace(params.secret, " ", "+", -1)
	return params, nil
}

// MakeAWSKMS is the factory method which returns a configured, ready-to-use
// AWS KMS object.
func MakeAWSKMS(ctx context.Context, uri string, env cloud.KMSEnv) (cloud.KMS, error) {
	if env.KMSConfig().DisableOutbound {
		return nil, errors.New("external IO must be enabled to use AWS KMS")
	}
	kmsURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}

	// Extract the URI parameters required to setup the AWS KMS session.
	kmsConsumeURL := cloud.ConsumeURL{URL: kmsURI}
	kmsURIParams, err := resolveKMSURIParams(kmsConsumeURL)
	if err != nil {
		return nil, err
	}

	region := kmsURIParams.region
	var httpClient *http.Client
	if kmsURIParams.endpoint != "" {
		if env.KMSConfig().DisableHTTP {
			return nil, errors.New(
				"custom endpoints disallowed for aws kms due to --aws-kms-disable-http flag")
		}
		if region == "" {
			// TODO(adityamaru): Think about what the correct way to handle this
			// situation is.
			region = "default-region"
		}
		httpClient, err = cloud.MakeHTTPClient(env.ClusterSettings(), cloud.NilMetrics, "aws", "KMS", "")
		if err != nil {
			return nil, err
		}
	}

	// "specified": use credentials provided in URI params; error if not present.
	// "implicit": use the credentials of the node's environment.
	// "": default to `specified`.
	switch kmsURIParams.auth {
	case "", cloud.AuthParamSpecified:
		if kmsURIParams.accessKey == "" {
			return nil, errors.Errorf(
				"%s is set to '%s', but %s is not set",
				cloud.AuthParam,
				cloud.AuthParamSpecified,
				AWSAccessKeyParam,
			)
		}
		if kmsURIParams.secret == "" {
			return nil, errors.Errorf(
				"%s is set to '%s', but %s is not set",
				cloud.AuthParam,
				cloud.AuthParamSpecified,
				AWSSecretParam,
			)
		}
	case cloud.AuthParamImplicit:
		if env.KMSConfig().DisableImplicitCredentials {
			return nil, errors.New(
				"implicit credentials disallowed for s3 due to --external-io-disable-implicit-credentials flag")
		}
	default:
		return nil, errors.Errorf("unsupported value %s for %s", kmsURIParams.auth, cloud.AuthParam)
	}

	if region == "" {
		// TODO(adityamaru): Maybe use the KeyID to get the region, similar to how
		// we infer the region from the bucket for s3_storage.
		return nil, errors.New("could not find the aws kms region")
	}

	cfg, err := loadAWSConfig(ctx, awsConfigParams{
		auth:                  kmsURIParams.auth,
		accessKey:             kmsURIParams.accessKey,
		secret:                kmsURIParams.secret,
		tempToken:             kmsURIParams.tempToken,
		assumeRoleProvider:    kmsURIParams.roleProvider,
		delegateRoleProviders: kmsURIParams.delegateRoleProviders,
		httpClient:            httpClient,
		verbose:               kmsURIParams.verbose,
	})
	if err != nil {
		return nil, cloud.KMSInaccessible(errors.Wrap(err, "new aws session"))
	}
	cfg.Region = region

	reuse := reuseKMSSession.Get(&env.ClusterSettings().SV)
	if reuse {
		kmsClientCache.Lock()
		defer kmsClientCache.Unlock()

		if reflect.DeepEqual(kmsClientCache.key, kmsURIParams) {
			return kmsClientCache.kms, nil
		}
	}

	kms := &awsKMS{
		kms: kms.NewFromConfig(cfg, func(o *kms.Options) {
			if kmsURIParams.endpoint != "" {
				o.BaseEndpoint = aws.String(kmsURIParams.endpoint)
			}
		}),
		customerMasterKeyID: kmsURIParams.customerMasterKeyID,
	}

	if reuse {
		// We already have the cache lock from reading the cached client.
		kmsClientCache.key = kmsURIParams
		kmsClientCache.kms = kms
	}
	return kms, nil
}

// MasterKeyID implements the KMS interface.
func (k *awsKMS) MasterKeyID() string {
	return k.customerMasterKeyID
}

// Encrypt implements the KMS interface.
func (k *awsKMS) Encrypt(ctx context.Context, data []byte) ([]byte, error) {
	encryptInput := &kms.EncryptInput{
		KeyId:     &k.customerMasterKeyID,
		Plaintext: data,
	}

	encryptOutput, err := k.kms.Encrypt(ctx, encryptInput)
	if err != nil {
		return nil, cloud.KMSInaccessible(err)
	}

	return encryptOutput.CiphertextBlob, nil
}

// Decrypt implements the KMS interface.
func (k *awsKMS) Decrypt(ctx context.Context, data []byte) ([]byte, error) {
	decryptInput := &kms.DecryptInput{
		KeyId:          &k.customerMasterKeyID,
		CiphertextBlob: data,
	}

	decryptOutput, err := k.kms.Decrypt(ctx, decryptInput)
	if err != nil {
		return nil, cloud.KMSInaccessible(err)
	}

	return decryptOutput.Plaintext, nil
}

// Close implements the KMS interface.
func (k *awsKMS) Close() error {
	return nil
}
