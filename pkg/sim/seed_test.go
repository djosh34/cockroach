// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

//go:build sim

package sim

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/sim/fakes3"
	"github.com/jellevandenhooff/gosim"
)

// TestSeed is one seed of the simulation (see doc.go). gosim runs it with
// the seed, which decides everything: the generated schema, data, SQL and
// faults below, the order of the goroutines and the simulated time.
func TestSeed(t *testing.T) {
	gosim.SetSimulationTimeout(30 * time.Minute)
	cpu0, _ := gosim.CPUTime()
	// The run's random stream is the seed's; the generator's own stream
	// starts from it before anything else draws, so it depends on the seed
	// alone.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	newRNG := func() *rand.Rand { return rand.New(rand.NewPCG(rng.Uint64(), rng.Uint64())) }
	s3Env()
	ctx := context.Background()

	s := &scenario{
		rng:      rng,
		tables:   genSchema(rng),
		workers:  2 + rng.IntN(4),
		accounts: 3 + rng.IntN(5),
		runFor:   time.Duration(20+rng.IntN(40)) * time.Second,
		ledger:   make(map[int]*ledgerKey),
		stats:    make(map[string]int),
	}
	s.keys = s.workers * (2 + rng.IntN(8))
	defer func() {
		t.Logf("seed summary:\n%s", s.report())
		if cpu1, err := gosim.CPUTime(); err == nil {
			gosim.WriteArtifact("cpu-seconds", []byte(fmt.Sprintf("%.3f", (cpu1-cpu0).Seconds())))
		}
	}()

	c, err := startCluster(ctx, 3, fakes3.New(newRNG()))
	if err != nil {
		t.Fatalf("starting the cluster: %v", err)
	}
	s.c = c
	if err := s.setup(ctx); err != nil {
		t.Fatalf("setup: %v", err)
	}

	until := time.Now().Add(s.runFor)
	var wg sync.WaitGroup
	for w := 0; w < s.workers; w++ {
		wrng := newRNG()
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.worker(ctx, w, wrng, until)
		}()
	}
	frng := newRNG()
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.faults(frng, until)
	}()
	wg.Wait()

	if err := s.heal(ctx); err != nil {
		t.Fatalf("after the faults: %v", err)
	}
	if err := s.check(ctx); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := s.checkPITR(ctx); err != nil {
		t.Fatalf("PITR check: %v", err)
	}
	for _, f := range c.failed() {
		t.Error(f)
	}
	for _, f := range s.failures {
		t.Error(f)
	}
}
