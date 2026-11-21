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
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

// Each node gossips the identity rules and the backup section of --config
// that it has loaded. `patchroaddb debug gossip-values` shows them for
// every node. A node logs a warning and sets a metric when a live node has
// loaded something different. Nothing is enforced: each node uses its own.

const (
	// configGossipInterval is how often a node gossips its config again,
	// so that its entry does not expire while it runs.
	configGossipInterval = 30 * time.Second
	// configGossipTTL lets the entry of a node that stopped expire, so
	// that only live nodes are compared.
	configGossipTTL = 3 * configGossipInterval
)

var metaBackupConfigMismatchedNodes = metric.Metadata{
	Name:        "backup.config.mismatched_nodes",
	Help:        "Number of live nodes whose loaded backup section of --config differs from this node's",
	Measurement: "Nodes",
	Unit:        metric.Unit_COUNT,
}

// configGossip gossips and compares one part of the config.
type configGossip struct {
	// what names the part in log messages.
	what   string
	prefix string
	lines  func() []string
	// mismatched counts the live nodes whose part differs.
	mismatched *metric.Gauge

	nodeID roachpb.NodeID
	g      *gossip.Gossip

	mu struct {
		syncutil.Mutex
		// warned holds, per node, the two versions it was last warned
		// about, so that each difference is logged once.
		warned map[roachpb.NodeID]string
	}
}

// startIdentityRulesGossip gossips this node's identity rules.
func startIdentityRulesGossip(
	ctx context.Context,
	stopper *stop.Stopper,
	g *gossip.Gossip,
	nodeID roachpb.NodeID,
	cm *security.CertificateManager,
) error {
	reloaded := make(chan struct{}, 1)
	cm.OnReload(func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	})
	c := &configGossip{
		what:       "identity rules",
		prefix:     gossip.KeyIdentityRulesPrefix,
		lines:      func() []string { return cm.Rules().Lines() },
		mismatched: cm.Metrics().RulesMismatchedNodes,
	}
	return c.start(ctx, stopper, g, nodeID, reloaded)
}

// startBackupConfigGossip gossips this node's backup section, which does
// not change while the node runs. b is nil if the node has none.
func startBackupConfigGossip(
	ctx context.Context,
	stopper *stop.Stopper,
	g *gossip.Gossip,
	nodeID roachpb.NodeID,
	b *certconfig.Backup,
	mismatched *metric.Gauge,
) error {
	c := &configGossip{
		what:       "backup section",
		prefix:     gossip.KeyBackupConfigPrefix,
		lines:      b.Lines,
		mismatched: mismatched,
	}
	return c.start(ctx, stopper, g, nodeID, nil)
}

// start gossips the part now, after each signal on changed and every
// configGossipInterval, and compares it with the other nodes' whenever
// either changes.
func (c *configGossip) start(
	ctx context.Context,
	stopper *stop.Stopper,
	g *gossip.Gossip,
	nodeID roachpb.NodeID,
	changed <-chan struct{},
) error {
	c.nodeID, c.g = nodeID, g
	c.mu.warned = map[roachpb.NodeID]string{}
	g.RegisterCallback(gossip.MakePrefixPattern(c.prefix),
		func(string, roachpb.Value) { c.compare(ctx) })
	return stopper.RunAsyncTask(ctx, c.prefix+"-gossip", func(ctx context.Context) {
		var timer timeutil.Timer
		defer timer.Stop()
		for {
			c.publish(ctx)
			c.compare(ctx)
			timer.Reset(configGossipInterval)
			select {
			case <-timer.C:
				timer.Read = true
			case <-changed:
			case <-stopper.ShouldQuiesce():
				return
			}
		}
	})
}

func (c *configGossip) publish(ctx context.Context) {
	text := strings.Join(c.lines(), "\n")
	if err := c.g.AddInfo(
		gossip.MakeKey(c.prefix, strconv.Itoa(int(c.nodeID))), []byte(text), configGossipTTL,
	); err != nil {
		log.Ops.Warningf(ctx, "gossiping the %s: %v", c.what, err)
	}
}

// compare counts the live nodes whose part differs from ours and logs the
// differing lines of each such node once.
func (c *configGossip) compare(ctx context.Context) {
	ours := c.lines()
	ourText := strings.Join(ours, "\n")
	now := timeutil.Now().UnixNano()
	mismatched := 0

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.g.IterateInfos(c.prefix, func(key string, info gossip.Info) error {
		if info.TTLStamp <= now {
			// The node stopped gossiping: it is not live.
			return nil
		}
		nodeID, err := gossip.DecodeNodeDescKey(key, c.prefix)
		if err != nil {
			return err
		}
		if nodeID == c.nodeID {
			return nil
		}
		b, err := info.Value.GetBytes()
		if err != nil {
			return err
		}
		theirText := string(b)
		if theirText == ourText {
			delete(c.mu.warned, nodeID)
			return nil
		}
		mismatched++
		pair := theirText + "\x00" + ourText
		if warned, ok := c.mu.warned[nodeID]; ok && warned == pair {
			return nil
		}
		c.mu.warned[nodeID] = pair
		var theirs []string
		if theirText != "" {
			theirs = strings.Split(theirText, "\n")
		}
		onlyTheirs, onlyOurs := diffLines(theirs, ours), diffLines(ours, theirs)
		if len(onlyTheirs) == 0 && len(onlyOurs) == 0 {
			log.Ops.Warningf(ctx, "n%d has loaded the same %s in a different order", nodeID, c.what)
		} else {
			log.Ops.Warningf(ctx, "n%d has loaded a different %s; only on n%d: %q; only on this node: %q",
				nodeID, c.what, nodeID, onlyTheirs, onlyOurs)
		}
		return nil
	}); err != nil {
		log.Ops.Warningf(ctx, "comparing the %s of the nodes: %v", c.what, err)
		return
	}
	c.mismatched.Update(int64(mismatched))
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
