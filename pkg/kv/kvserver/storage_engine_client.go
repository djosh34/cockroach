// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvserver

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/rpc/nodedialer"
	"github.com/cockroachdb/cockroach/pkg/storage/enginepb"
	"github.com/cockroachdb/errors"
)

// StorageEngineClient is used to connect and make requests to a store.
type StorageEngineClient struct {
	nd *nodedialer.Dialer
}

// NewStorageEngineClient constructs a new StorageEngineClient.
func NewStorageEngineClient(nd *nodedialer.Dialer) *StorageEngineClient {
	return &StorageEngineClient{nd: nd}
}

// CompactEngineSpan is a tree.CompactEngineSpanFunc.
func (c *StorageEngineClient) CompactEngineSpan(
	ctx context.Context, nodeID, storeID int32, startKey, endKey []byte,
) error {
	conn, err := c.nd.Dial(ctx, roachpb.NodeID(nodeID), rpc.DefaultClass)
	if err != nil {
		return errors.Wrapf(err, "could not dial node ID %d", nodeID)
	}
	client := NewPerStoreClient(conn)
	req := &CompactEngineSpanRequest{
		StoreRequestHeader: StoreRequestHeader{
			NodeID:  roachpb.NodeID(nodeID),
			StoreID: roachpb.StoreID(storeID),
		},
		Span: roachpb.Span{Key: roachpb.Key(startKey), EndKey: roachpb.Key(endKey)},
	}
	_, err = client.CompactEngineSpan(ctx, req)
	return err
}

// GetTableMetrics is a tree.GetTableMetricsFunc.
func (c *StorageEngineClient) GetTableMetrics(
	ctx context.Context, nodeID, storeID int32, startKey, endKey []byte,
) ([]enginepb.SSTableMetricsInfo, error) {
	conn, err := c.nd.Dial(ctx, roachpb.NodeID(nodeID), rpc.DefaultClass)
	if err != nil {
		return []enginepb.SSTableMetricsInfo{}, errors.Wrapf(err, "could not dial node ID %d", nodeID)
	}

	client := NewPerStoreClient(conn)
	req := &GetTableMetricsRequest{
		StoreRequestHeader: StoreRequestHeader{
			NodeID:  roachpb.NodeID(nodeID),
			StoreID: roachpb.StoreID(storeID),
		},
		Span: roachpb.Span{Key: roachpb.Key(startKey), EndKey: roachpb.Key(endKey)},
	}

	resp, err := client.GetTableMetrics(ctx, req)

	if err != nil {
		return []enginepb.SSTableMetricsInfo{}, err
	}
	return resp.TableMetrics, nil
}

// ScanStorageInternalKeys is a tree.ScanStorageInternalKeys
func (c *StorageEngineClient) ScanStorageInternalKeys(
	ctx context.Context, nodeID, storeID int32, startKey, endKey []byte, megabytesPerSecond int64,
) ([]enginepb.StorageInternalKeysMetrics, error) {
	conn, err := c.nd.Dial(ctx, roachpb.NodeID(nodeID), rpc.DefaultClass)
	if err != nil {
		return []enginepb.StorageInternalKeysMetrics{}, errors.Wrapf(err, "could not dial node ID %d", nodeID)
	}

	client := NewPerStoreClient(conn)
	req := &ScanStorageInternalKeysRequest{
		StoreRequestHeader: StoreRequestHeader{
			NodeID:  roachpb.NodeID(nodeID),
			StoreID: roachpb.StoreID(storeID),
		},
		Span:               roachpb.Span{Key: roachpb.Key(startKey), EndKey: roachpb.Key(endKey)},
		MegabytesPerSecond: megabytesPerSecond,
	}

	resp, err := client.ScanStorageInternalKeys(ctx, req)

	if err != nil {
		return []enginepb.StorageInternalKeysMetrics{}, err
	}
	return resp.AdvancedPebbleMetrics, nil
}

// SetCompactionConcurrency is a tree.CompactionConcurrencyFunc.
func (c *StorageEngineClient) SetCompactionConcurrency(
	ctx context.Context, nodeID, storeID int32, compactionConcurrency uint64,
) error {
	conn, err := c.nd.Dial(ctx, roachpb.NodeID(nodeID), rpc.DefaultClass)
	if err != nil {
		return errors.Wrapf(err, "could not dial node ID %d", nodeID)
	}
	client := NewPerStoreClient(conn)
	req := &CompactionConcurrencyRequest{
		StoreRequestHeader: StoreRequestHeader{
			NodeID:  roachpb.NodeID(nodeID),
			StoreID: roachpb.StoreID(storeID),
		},
		CompactionConcurrency: compactionConcurrency,
	}
	_, err = client.SetCompactionConcurrency(ctx, req)
	if err != nil {
		return err
	}
	return nil
}
