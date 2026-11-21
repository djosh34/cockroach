// Copyright 2026 The Cockroach Authors.
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
	"context"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

// Each node gossips the identity rules it has loaded. `patchroaddb debug
// gossip-values` shows them for every node. A node logs a warning and sets
// the security.identity.rules.mismatched_nodes metric when a live node has
// loaded different rules. Nothing is enforced: each node admits peers by
// its own rules.

const (
	// identityRulesGossipInterval is how often a node gossips its rules
	// again, so that its entry does not expire while it runs.
	identityRulesGossipInterval = 30 * time.Second
	// identityRulesGossipTTL lets the entry of a node that stopped expire,
	// so that only live nodes are compared.
	identityRulesGossipTTL = 3 * identityRulesGossipInterval
)

type identityRulesGossip struct {
	nodeID roachpb.NodeID
	g      *gossip.Gossip
	cm     *security.CertificateManager

	mu struct {
		syncutil.Mutex
		// warned holds, per node, the rules it was last warned about, so
		// that each difference is logged once.
		warned map[roachpb.NodeID]string
	}
}

// startIdentityRulesGossip gossips this node's rules now, after each
// reload and every identityRulesGossipInterval, and compares them with the
// rules of the other nodes whenever either changes.
func startIdentityRulesGossip(
	ctx context.Context,
	stopper *stop.Stopper,
	g *gossip.Gossip,
	nodeID roachpb.NodeID,
	cm *security.CertificateManager,
) error {
	r := &identityRulesGossip{nodeID: nodeID, g: g, cm: cm}
	r.mu.warned = map[roachpb.NodeID]string{}
	reloaded := make(chan struct{}, 1)
	cm.OnReload(func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	})
	g.RegisterCallback(gossip.MakePrefixPattern(gossip.KeyIdentityRulesPrefix),
		func(string, roachpb.Value) { r.compare(ctx) })
	return stopper.RunAsyncTask(ctx, "identity-rules-gossip", func(ctx context.Context) {
		var timer timeutil.Timer
		defer timer.Stop()
		for {
			r.publish(ctx)
			r.compare(ctx)
			timer.Reset(identityRulesGossipInterval)
			select {
			case <-timer.C:
				timer.Read = true
			case <-reloaded:
			case <-stopper.ShouldQuiesce():
				return
			}
		}
	})
}

func (r *identityRulesGossip) publish(ctx context.Context) {
	rules := strings.Join(r.cm.Rules().Lines(), "\n")
	if err := r.g.AddInfo(
		gossip.MakeIdentityRulesKey(r.nodeID), []byte(rules), identityRulesGossipTTL,
	); err != nil {
		log.Ops.Warningf(ctx, "gossiping the identity rules: %v", err)
	}
}

// compare counts the live nodes whose rules differ from ours and logs the
// differing lines of each such node once.
func (r *identityRulesGossip) compare(ctx context.Context) {
	ours := r.cm.Rules().Lines()
	ourText := strings.Join(ours, "\n")
	now := timeutil.Now().UnixNano()
	mismatched := 0

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.g.IterateInfos(gossip.KeyIdentityRulesPrefix, func(key string, info gossip.Info) error {
		if info.TTLStamp <= now {
			// The node stopped gossiping: it is not live.
			return nil
		}
		nodeID, err := gossip.DecodeNodeDescKey(key, gossip.KeyIdentityRulesPrefix)
		if err != nil {
			return err
		}
		if nodeID == r.nodeID {
			return nil
		}
		b, err := info.Value.GetBytes()
		if err != nil {
			return err
		}
		theirText := string(b)
		if theirText == ourText {
			delete(r.mu.warned, nodeID)
			return nil
		}
		mismatched++
		if r.mu.warned[nodeID] == theirText {
			return nil
		}
		r.mu.warned[nodeID] = theirText
		var theirs []string
		if theirText != "" {
			theirs = strings.Split(theirText, "\n")
		}
		onlyTheirs, onlyOurs := diffLines(theirs, ours), diffLines(ours, theirs)
		if len(onlyTheirs) == 0 && len(onlyOurs) == 0 {
			log.Ops.Warningf(ctx, "n%d has loaded the same identity rules in a different order", nodeID)
		} else {
			log.Ops.Warningf(ctx, "n%d has loaded different identity rules; only on n%d: %q; only on this node: %q",
				nodeID, nodeID, onlyTheirs, onlyOurs)
		}
		return nil
	}); err != nil {
		log.Ops.Warningf(ctx, "comparing the identity rules of the nodes: %v", err)
		return
	}
	r.cm.Metrics().RulesMismatchedNodes.Update(int64(mismatched))
}

// diffLines returns the lines of a that are not in b.
func diffLines(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, l := range b {
		inB[l] = true
	}
	var out []string
	for _, l := range a {
		if !inB[l] {
			out = append(out, l)
		}
	}
	return out
}
