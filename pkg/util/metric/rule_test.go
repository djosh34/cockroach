// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package metric

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleRegistry(t *testing.T) {
	const testRule = "test_rule"
	const help = "Test rule"
	const isKV = false
	rule := NewAggregationRule(testRule, " sum without(store) (capacity_available)", nil, help, isKV)
	registry := NewRuleRegistry()
	registry.AddRule(rule)
	require.NotNil(t, registry.GetRuleForTest(testRule))
	require.Nil(t, registry.GetRuleForTest("invalid"))
}
