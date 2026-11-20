// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package log

import "github.com/cockroachdb/cockroach/pkg/util/syncutil"

type loggerRegistry struct {
	mu struct {
		syncutil.Mutex
		loggers []*loggerT
	}
}

// clear erases the registry.
func (r *loggerRegistry) clear() {
	r.mu.Lock()
	r.mu.loggers = nil
	r.mu.Unlock()
}

// put adds a logger into the registry.
func (r *loggerRegistry) put(l *loggerT) {
	r.mu.Lock()
	r.mu.loggers = append(r.mu.loggers, l)
	r.mu.Unlock()
}

// del removes one logger from the registry.
func (r *loggerRegistry) del(l *loggerT) {
	// Make the registry forget about this logger. This avoids
	// stacking many secondary loggers together when there are
	// subsequent tests starting servers in the same package.
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, thatLogger := range r.mu.loggers {
		if thatLogger != l {
			continue
		}
		r.mu.loggers = append(r.mu.loggers[:i], r.mu.loggers[i+1:]...)
		return
	}
}

type sinkInfoRegistry struct {
	mu struct {
		syncutil.Mutex
		sinkInfos []*sinkInfo
	}
}

// clear erases the registry.
func (r *sinkInfoRegistry) clear() {
	r.mu.Lock()
	r.mu.sinkInfos = nil
	r.mu.Unlock()
}

// iter iterates over all the sinks infos and stops at the first error
// encountered.
func (r *sinkInfoRegistry) iter(fn func(l *sinkInfo) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.mu.sinkInfos {
		if err := fn(l); err != nil {
			return err
		}
	}
	return nil
}

// iterate iterates over all the file sinks and stops at the first
// error encountered.
func (r *sinkInfoRegistry) iterFileSinks(fn func(l *fileSink) error) error {
	return r.iter(func(si *sinkInfo) error {
		if fs, ok := si.sink.(*fileSink); ok {
			if err := fn(fs); err != nil {
				return err
			}
		}
		return nil
	})
}

// iterBufferedSinks iterates over all the buffered sinks and stops at the first
// error encountered.
func (r *sinkInfoRegistry) iterBufferedSinks(fn func(bs *bufferedSink) error) error {
	return r.iter(func(si *sinkInfo) error {
		if bs, ok := si.sink.(*bufferedSink); ok {
			if err := fn(bs); err != nil {
				return err
			}
		}
		return nil
	})
}

// iterHttpSink iterates over all the http sinks and stops at the first error
// encountered.
func (r *sinkInfoRegistry) iterHTTPSinks(fn func(hs *httpSink) error) error {
	return r.iter(func(si *sinkInfo) error {
		if hs, ok := si.sink.(*httpSink); ok {
			return fn(hs)
		}
		// Many times we buffer HTTP sinks, so be sure to check that case as well.
		if bs, isBuffered := si.sink.(*bufferedSink); isBuffered {
			if ch, ok := bs.child.(*httpSink); ok {
				return fn(ch)
			}
		}
		return nil
	})
}

// put adds a sinkInfo into the registry.
func (r *sinkInfoRegistry) put(l *sinkInfo) {
	r.mu.Lock()
	r.mu.sinkInfos = append(r.mu.sinkInfos, l)
	r.mu.Unlock()
}

// del removes one sinkInfo from the registry.
func (r *sinkInfoRegistry) del(l *sinkInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, thatSink := range r.mu.sinkInfos {
		if thatSink != l {
			continue
		}
		r.mu.sinkInfos = append(r.mu.sinkInfos[:i], r.mu.sinkInfos[i+1:]...)
		return
	}
}
