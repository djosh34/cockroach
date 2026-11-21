// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

//go:build !sim

package sim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

// procs is the env of the nightly chaos run (build/ci/chaos.sh): real
// patchroaddb processes, each in a network namespace of its own, and a
// versitygw container for S3. Crashes are SIGKILL; a killed process keeps
// what it wrote, since the kernel does not lose it. Partitions and delays
// are iptables rules and netem in the node's namespace, and an S3 outage
// pauses the container.
type procs struct {
	bin      string
	dir      string
	endpoint string
	s3       string // the versitygw container
	user     string

	mu       syncutil.Mutex
	nodes    []*proc
	restored *proc
	failures []string
}

type proc struct {
	idx    int
	ns     string
	addr   string
	args   []string
	killed bool
}

// The namespaces chaos.sh makes: n1 to n4 at 10.9.0.1 to 10.9.0.4 on eth0,
// bridged to the host at 10.9.0.254. n4 is for the restore.
func chaosNS(i int) (ns, addr string) { return fmt.Sprintf("n%d", i+1), fmt.Sprintf("10.9.0.%d", i+1) }

const chaosHost = "10.9.0.254"

func (p *procs) root(args ...string) error {
	out, err := exec.Command("sudo", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

func (p *procs) nodeDir(n *proc) string { return filepath.Join(p.dir, n.ns) }

// start starts n's process and records it if it ends without a kill.
func (p *procs) start(n *proc) error {
	dir := p.nodeDir(n)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfg := filepath.Join(dir, "sim.yaml")
	if err := os.WriteFile(cfg, []byte(backupConfig(p.endpoint)), 0o644); err != nil {
		return err
	}
	args := append([]string{"ip", "netns", "exec", n.ns, "sudo", "-u", p.user,
		"env", "AWS_ACCESS_KEY_ID=" + s3Key, "AWS_SECRET_ACCESS_KEY=" + s3Secret, "AWS_REGION=" + s3Region,
		p.bin}, n.args...)
	args = append(args, "--store=path="+filepath.Join(dir, "data")+",ballast-size=0",
		"--config="+cfg, "--log-dir="+filepath.Join(dir, "logs"), "--pid-file="+filepath.Join(dir, "pid"))
	cmd := exec.Command("sudo", args...)
	f, err := os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}
	p.mu.Lock()
	n.killed = false
	p.mu.Unlock()
	go func() {
		err := cmd.Wait()
		f.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		if !n.killed {
			p.failures = append(p.failures, fmt.Sprintf("%s stopped on its own: %v", n.ns, err))
			n.killed = true
		}
	}()
	return nil
}

func (p *procs) addrs() []string {
	var addrs []string
	for _, n := range p.nodes {
		addrs = append(addrs, n.addr)
	}
	return addrs
}

func (p *procs) s3Endpoint() string { return p.endpoint }

func (p *procs) crash(i int) bool { return p.kill(p.nodes[i]) }

func (p *procs) kill(n *proc) bool {
	p.mu.Lock()
	if n.killed {
		p.mu.Unlock()
		return false
	}
	n.killed = true
	p.mu.Unlock()
	pid, err := os.ReadFile(filepath.Join(p.nodeDir(n), "pid"))
	if err == nil {
		err = p.root("kill", "-KILL", strings.TrimSpace(string(pid)))
	}
	if err != nil {
		p.mu.Lock()
		p.failures = append(p.failures, fmt.Sprintf("killing %s: %v", n.ns, err))
		p.mu.Unlock()
	}
	return true
}

func (p *procs) restart(i int, _ bool) {
	if err := p.start(p.nodes[i]); err != nil {
		p.mu.Lock()
		p.failures = append(p.failures, fmt.Sprintf("restarting %s: %v", p.nodes[i].ns, err))
		p.mu.Unlock()
	}
}

// partition drops the node's traffic to and from the other nodes; the
// host, where the workload runs, still reaches it.
func (p *procs) partition(i int, on bool) {
	n := p.nodes[i]
	op := "-D"
	if on {
		op = "-I"
	}
	_ = p.root("ip", "netns", "exec", n.ns, "iptables", op, "INPUT", "-s", "10.9.0.0/24", "!", "-s", chaosHost, "-j", "DROP")
	_ = p.root("ip", "netns", "exec", n.ns, "iptables", op, "OUTPUT", "-d", "10.9.0.0/24", "!", "-d", chaosHost, "-j", "DROP")
}

// delay delays everything node i sends; j does not matter here.
func (p *procs) delay(i, _ int, d time.Duration) {
	n := p.nodes[i]
	if d == 0 {
		_ = p.root("ip", "netns", "exec", n.ns, "tc", "qdisc", "del", "dev", "eth0", "root")
		return
	}
	_ = p.root("ip", "netns", "exec", n.ns, "tc", "qdisc", "replace", "dev", "eth0", "root", "netem",
		"delay", strconv.Itoa(int(d.Milliseconds()))+"ms")
}

func (p *procs) s3Faults(on bool) {
	op := "unpause"
	if on {
		op = "pause"
	}
	_ = exec.Command("docker", op, p.s3).Run()
}

func (p *procs) syncFaults(int, bool) bool { return false }

func (p *procs) startRestoreNode(ctx context.Context) (string, error) {
	ns, addr := chaosNS(len(p.nodes))
	n := &proc{idx: len(p.nodes), ns: ns, addr: addr, args: []string{
		"start-single-node", "--insecure", "--listen-addr=" + addr + ":26257", "--http-addr=" + addr + ":8080",
		"--cache=64MiB", "--max-sql-memory=128MiB",
	}}
	if err := p.start(n); err != nil {
		return "", err
	}
	p.restored = n
	return addr, waitSQL(ctx, addr, 2*time.Minute)
}

func (p *procs) failed() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failures...)
}

// stop kills every process at the end of the run.
func (p *procs) stop() {
	for _, n := range p.nodes {
		p.kill(n)
	}
	if p.restored != nil {
		p.kill(p.restored)
	}
}
