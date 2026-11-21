// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package security

import (
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/metric/aggmetric"
)

const SQLUserLabel = "sql_user"

// Metrics is a metric.Struct for certificates.
type Metrics struct {
	// Expiration times. Each is 0 when the file is not configured.
	CAExpiration       *metric.Gauge
	ClientCAExpiration *metric.Gauge
	NodeExpiration     *metric.Gauge
	// ClientExpiration is an aggregate metric, containing child metrics for all
	// users that have done cert auth with this node.
	// The children are labeled by SQL user.
	// The top-level aggregated value for this metric is not meaningful
	// (it sums up all the minimum expirations of all users).
	ClientExpiration *aggmetric.AggGauge
	// ReloadFailures counts reloads that kept the old files because a new
	// file did not parse or did not match.
	ReloadFailures *metric.Counter
	// RulesMismatchedNodes is the number of live nodes whose identity rules
	// differ from this node's.
	RulesMismatchedNodes *metric.Gauge
}

var _ metric.Struct = (*Metrics)(nil)

// MetricStruct indicates that Metrics is a metric.Struct.
func (m *Metrics) MetricStruct() {}

var (
	metaCAExpiration = metric.Metadata{
		Name:        "security.certificate.expiration.ca",
		Help:        "Earliest expiration of the node CA certificates. 0 means none.",
		Measurement: "Certificate Expiration",
		Unit:        metric.Unit_TIMESTAMP_SEC,
	}
	metaClientCAExpiration = metric.Metadata{
		Name:        "security.certificate.expiration.client-ca",
		Help:        "Earliest expiration of the client CA certificates. 0 means none.",
		Measurement: "Certificate Expiration",
		Unit:        metric.Unit_TIMESTAMP_SEC,
	}
	metaClientExpiration = metric.Metadata{
		Name: "security.certificate.expiration.client",
		Help: "Minimum expiration for client certificates, labeled by SQL user. 0 means no " +
			"certificate or error. ",
		Measurement: "Certificate Expiration",
		Unit:        metric.Unit_TIMESTAMP_SEC,
	}
	metaNodeExpiration = metric.Metadata{
		Name:        "security.certificate.expiration.node",
		Help:        "Expiration for the node certificate. 0 means no certificate.",
		Measurement: "Certificate Expiration",
		Unit:        metric.Unit_TIMESTAMP_SEC,
	}
	metaRulesMismatchedNodes = metric.Metadata{
		Name:        "security.identity.rules.mismatched_nodes",
		Help:        "Number of live nodes whose loaded identity rules differ from this node's",
		Measurement: "Nodes",
		Unit:        metric.Unit_COUNT,
	}
	metaReloadFailures = metric.Metadata{
		Name:        "security.certificate.reload.failures",
		Help:        "Reloads of the certificate, key, CA and rules files that kept the old files because of an error",
		Measurement: "Reloads",
		Unit:        metric.Unit_COUNT,
	}
)

func makeMetrics() Metrics {
	b := aggmetric.MakeBuilder(SQLUserLabel)
	return Metrics{
		CAExpiration:         metric.NewGauge(metaCAExpiration),
		ClientCAExpiration:   metric.NewGauge(metaClientCAExpiration),
		ClientExpiration:     b.Gauge(metaClientExpiration),
		NodeExpiration:       metric.NewGauge(metaNodeExpiration),
		ReloadFailures:       metric.NewCounter(metaReloadFailures),
		RulesMismatchedNodes: metric.NewGauge(metaRulesMismatchedNodes),
	}
}
