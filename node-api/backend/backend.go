// SPDX-License-Identifier: BUSL-1.1
//
// Copyright (C) 2025, Berachain Foundation. All rights reserved.
// Use of this software is governed by the Business Source License included
// in the LICENSE file of this repository and at www.mariadb.com/bsl11.
//
// ANY USE OF THE LICENSED WORK IN VIOLATION OF THIS LICENSE WILL AUTOMATICALLY
// TERMINATE YOUR RIGHTS UNDER THIS LICENSE FOR THE CURRENT AND ALL OTHER
// VERSIONS OF THE LICENSED WORK.
//
// THIS LICENSE DOES NOT GRANT YOU ANY RIGHT IN ANY TRADEMARK OR LOGO OF
// LICENSOR OR ITS AFFILIATES (PROVIDED THAT YOU MAY USE A TRADEMARK OR LOGO OF
// LICENSOR AS EXPRESSLY REQUIRED BY THIS LICENSE).
//
// TO THE EXTENT PERMITTED BY APPLICABLE LAW, THE LICENSED WORK IS PROVIDED ON
// AN “AS IS” BASIS. LICENSOR HEREBY DISCLAIMS ALL WARRANTIES AND CONDITIONS,
// EXPRESS OR IMPLIED, INCLUDING (WITHOUT LIMITATION) WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, NON-INFRINGEMENT, AND
// TITLE.

package backend

import (
	"github.com/berachain/beacon-kit/chain"
	cometbft "github.com/berachain/beacon-kit/consensus/cometbft/service"
	"github.com/berachain/beacon-kit/node-core/components/storage"
	"github.com/berachain/beacon-kit/primitives/common"
	"github.com/berachain/beacon-kit/primitives/math"
	statedb "github.com/berachain/beacon-kit/state-transition/core/state"
)

// Backend is the db access layer for the beacon node-api.
// It serves as a wrapper around the storage backend and provides an abstraction
// over building the query context for a given state.
type Backend struct {
	sb   *storage.Backend
	cs   chain.Spec
	node *cometbft.Service
	sp   StateProcessor
}

// New creates and returns a new Backend instance.
func New(
	storageBackend *storage.Backend,
	cs chain.Spec,
	sp StateProcessor,
) *Backend {
	return &Backend{
		sb: storageBackend,
		cs: cs,
		sp: sp,
	}
}

// AttachQueryBackend sets the node on the backend for
// querying historical heights.
func (b *Backend) AttachQueryBackend(node *cometbft.Service) {
	b.node = node
}

// ChainSpec returns the chain spec from the backend.
func (b *Backend) ChainSpec() chain.Spec {
	return b.cs
}

// GetSlotByBlockRoot retrieves the slot by a block root from the block store.
func (b *Backend) GetSlotByBlockRoot(root common.Root) (math.Slot, error) {
	return b.sb.BlockStore().GetSlotByBlockRoot(root)
}

// GetSlotByStateRoot retrieves the slot by a state root from the block store.
func (b *Backend) GetSlotByStateRoot(root common.Root) (math.Slot, error) {
	return b.sb.BlockStore().GetSlotByStateRoot(root)
}

// GetParentSlotByTimestamp retrieves the parent slot by a given timestamp from
// the block store.
func (b *Backend) GetParentSlotByTimestamp(timestamp math.U64) (math.Slot, error) {
	return b.sb.BlockStore().GetParentSlotByTimestamp(timestamp)
}

// stateFromSlot returns the state at the given slot, after also processing the
// next slot to ensure the returned beacon state is up to date.
func (b *Backend) stateFromSlot(slot math.Slot) (*statedb.StateDB, math.Slot, error) {
	var (
		st  *statedb.StateDB
		err error
	)
	if st, slot, err = b.stateFromSlotRaw(slot); err != nil {
		return st, slot, err
	}

	// Process the slot to update the latest state and block roots.
	if _, err = b.sp.ProcessSlots(st, slot+1); err != nil {
		return st, slot, err
	}

	// We need to set the slot on the state back since ProcessSlot will update
	// it to slot + 1.
	err = st.SetSlot(slot)
	return st, slot, err
}

// stateFromSlotRaw returns the state at the given slot using query context,
// resolving an input slot of 0 to the latest slot. It does not process the
// next slot on the beacon state.
func (b *Backend) stateFromSlotRaw(slot math.Slot) (*statedb.StateDB, math.Slot, error) {
	var st *statedb.StateDB
	queryCtx, err := b.node.CreateQueryContext(int64(slot), false) // #nosec G115 -- not an issue in practice.
	if err != nil {
		return st, slot, err
	}
	st = b.sb.StateFromContext(queryCtx)

	// If using height 0 for the query context, make sure to return the latest
	// slot.
	if slot == 0 {
		slot, err = st.GetSlot()
		if err != nil {
			return st, slot, err
		}
	}
	return st, slot, err
}
