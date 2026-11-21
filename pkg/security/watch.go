// Copyright 2026 The Cockroach Authors.
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
	"context"
	"path/filepath"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/sysutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"github.com/fsnotify/fsnotify"
)

// reloadDelay is how long Watch waits after a file event for more events
// before it reloads. A Kubernetes secret update, for example, is several
// events in a row.
const reloadDelay = 200 * time.Millisecond

// Watch reloads the files on SIGHUP and whenever something changes in the
// directory of any of them. It watches directories, not files, so that it
// sees a symlink swap such as a Kubernetes secret update. Network file
// systems that send no events need a SIGHUP.
func (cm *CertificateManager) Watch(ctx context.Context, stopper *stop.Stopper) error {
	var dirs []string
	seen := map[string]bool{}
	for _, p := range []string{
		cm.cfg.Cert, cm.cfg.Key, cm.cfg.NodeCA, cm.cfg.ClientCA,
		cm.cfg.Rules, cm.cfg.ClientCert, cm.cfg.ClientKey,
	} {
		if p == "" || securityassets.GetLoader().InMemory(p) {
			continue
		}
		if dir := filepath.Dir(p); !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) == 0 {
		// The files are served from memory, as in tests.
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "watching certificate files")
	}
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			_ = watcher.Close()
			return errors.Wrapf(err, "watching %s", dir)
		}
	}
	// Ask for SIGHUP before returning, so that a SIGHUP sent once Watch
	// returns reloads instead of stopping the process.
	sighup := sysutil.RefreshSignaledChan()
	err = stopper.RunAsyncTask(ctx, "watch-certs", func(ctx context.Context) {
		defer func() { _ = watcher.Close() }()
		var timer timeutil.Timer
		defer timer.Stop()
		for {
			select {
			case <-stopper.ShouldQuiesce():
				return
			case sig := <-sighup:
				log.Ops.Infof(ctx, "received signal %q, reloading certificates and rules", sig)
				_ = cm.Reload(ctx)
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Ops.VInfof(ctx, 1, "certificate directory event: %s", ev)
				timer.Reset(reloadDelay)
			case <-timer.C:
				timer.Read = true
				log.Ops.Infof(ctx, "files changed, reloading certificates and rules")
				_ = cm.Reload(ctx)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Ops.Errorf(ctx, "watching certificate files: %v", err)
			}
		}
	})
	if err != nil {
		_ = watcher.Close()
	}
	return err
}
