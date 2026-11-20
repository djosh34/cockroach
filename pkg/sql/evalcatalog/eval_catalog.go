// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package evalcatalog provides the concrete implementation of
// eval.CatalogBuiltins.
package evalcatalog

import (
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
)

// Builtins implements methods to evaluate logic that depends on having
// catalog access. It implements the eval.Catalog interface. Note that it
// importantly is not the planner directly.
//
// NOTE: The hope is that many of the methods of the planner will make their
// way to this object and that this object may subsume awareness of session
// information.
//
// TODO(ajwerner): Extract the sql.schemaResolver and consider unifying with
// this thing or wrapping that thing.
type Builtins struct {
	codec keys.SQLCodec
	dc    *descs.Collection
	txn   *kv.Txn
}

// Init initializes the fields of a Builtins. The object should not be used
// before being initialized.
func (ec *Builtins) Init(codec keys.SQLCodec, txn *kv.Txn, descriptors *descs.Collection) {
	ec.codec = codec
	ec.txn = txn
	ec.dc = descriptors
}
