// Copyright 2019 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/pebble/vfs"
)

// StickyVFSOption is a config option for a sticky engine
// registry that can be passed to NewStickyVFSRegistry.
type StickyVFSOption func(cfg *stickyConfig)

type stickyConfig struct {
	newFS func() *vfs.MemFS // by default vfs.NewMem
}

// UseStrictMemFS option instructs StickyVFSRegistry to produce strict in-memory
// filesystems, i.e. to use vfs.NewStrictMem instead of vfs.NewMem.
var UseStrictMemFS = StickyVFSOption(func(cfg *stickyConfig) {
	cfg.newFS = vfs.NewStrictMem
})

// StickyVFSRegistry manages the lifecycle of sticky in-memory filesystems. It
// is intended for use in demos and/or tests, where we want in-memory storage
// nodes to persist between killed nodes.
type StickyVFSRegistry interface {
	// Get returns the named in-memory FS, constructing a new one if this is the
	// first time a FS with the provided ID has been requested.
	Get(stickyVFSID string) *vfs.MemFS
}

// stickyVFSRegistryImpl is the bookkeeper for all active sticky filesystems,
// keyed by their id. It implements the StickyVFSRegistry interface.
type stickyVFSRegistryImpl struct {
	entries map[string]*vfs.MemFS
	mu      syncutil.Mutex
	cfg     stickyConfig
}

// NewStickyVFSRegistry creates a new StickyVFSRegistry.
func NewStickyVFSRegistry(opts ...StickyVFSOption) StickyVFSRegistry {
	registry := &stickyVFSRegistryImpl{
		entries: map[string]*vfs.MemFS{},
	}
	for _, opt := range opts {
		opt(&registry.cfg)
	}
	// Use the regular in-memory filesystem, unless specified otherwise.
	if registry.cfg.newFS == nil {
		registry.cfg.newFS = vfs.NewMem
	}
	return registry
}

// Get implements the StickyVFSRegistry interface.
func (registry *stickyVFSRegistryImpl) Get(stickyVFSID string) *vfs.MemFS {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if fs, ok := registry.entries[stickyVFSID]; ok {
		return fs
	}
	fs := registry.cfg.newFS()
	registry.entries[stickyVFSID] = fs
	return fs
}
