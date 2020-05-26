// Copyright 2020 ChainSafe Systems (ON) Corp.
// This file is part of gossamer.
//
// The gossamer library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The gossamer library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the gossamer library. If not, see <http://www.gnu.org/licenses/>.

package grandpa

import (
	"math/big"
	"testing"

	"github.com/ChainSafe/gossamer/dot/state"
	"github.com/ChainSafe/gossamer/dot/types"
	"github.com/ChainSafe/gossamer/lib/keystore"

	"github.com/stretchr/testify/require"
)

func TestCheckForEquivocation_NoEquivocation(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)
	state.AddBlocksToState(t, st.Block, 3)

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	vote := NewVoteFromHeader(h)
	require.NoError(t, err)

	for _, v := range voters {
		equivocated := gs.checkForEquivocation(v, vote)
		require.False(t, equivocated)
	}
}

func TestCheckForEquivocation_WithEquivocation(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)

	var branches []*types.Header
	for {
		_, branches = state.AddBlocksToState(t, st.Block, 3)
		if len(branches) != 0 {
			break
		}
	}

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	vote := NewVoteFromHeader(h)
	require.NoError(t, err)

	voter := voters[0]

	gs.votes[voter.key.AsBytes()] = vote

	vote2 := NewVoteFromHeader(branches[0])
	require.NoError(t, err)

	equivocated := gs.checkForEquivocation(voter, vote2)
	require.True(t, equivocated)

	require.Equal(t, 0, len(gs.votes))
	require.Equal(t, 1, len(gs.equivocations))
	require.Equal(t, 2, len(gs.equivocations[voter.key.AsBytes()]))
}

func TestCheckForEquivocation_WithExistingEquivocation(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)

	var branches []*types.Header
	for {
		_, branches = state.AddBlocksToState(t, st.Block, 8)
		if len(branches) > 1 {
			break
		}
	}

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	vote := NewVoteFromHeader(h)
	require.NoError(t, err)

	voter := voters[0]

	gs.votes[voter.key.AsBytes()] = vote

	vote2 := NewVoteFromHeader(branches[0])
	require.NoError(t, err)

	equivocated := gs.checkForEquivocation(voter, vote2)
	require.True(t, equivocated)

	require.Equal(t, 0, len(gs.votes))
	require.Equal(t, 1, len(gs.equivocations))

	vote3 := NewVoteFromHeader(branches[1])
	require.NoError(t, err)

	equivocated = gs.checkForEquivocation(voter, vote3)
	require.True(t, equivocated)

	require.Equal(t, 0, len(gs.votes))
	require.Equal(t, 1, len(gs.equivocations))
	require.Equal(t, 3, len(gs.equivocations[voter.key.AsBytes()]))
}

func TestValidateMessage_Valid(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)
	state.AddBlocksToState(t, st.Block, 3)

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	msg, err := gs.CreateVoteMessage(h, kr.Alice)
	require.NoError(t, err)

	vote, err := gs.ValidateMessage(msg)
	require.NoError(t, err)
	require.Equal(t, h.Hash(), vote.hash)
}

func TestValidateMessage_InvalidSignature(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)
	state.AddBlocksToState(t, st.Block, 3)

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	msg, err := gs.CreateVoteMessage(h, kr.Alice)
	require.NoError(t, err)

	msg.message.signature[63] = 0

	_, err = gs.ValidateMessage(msg)
	require.Equal(t, err, ErrInvalidSignature)
}

func TestValidateMessage_SetIDMismatch(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)
	state.AddBlocksToState(t, st.Block, 3)

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	msg, err := gs.CreateVoteMessage(h, kr.Alice)
	require.NoError(t, err)

	gs.state.setID = 1

	_, err = gs.ValidateMessage(msg)
	require.Equal(t, err, ErrSetIDMismatch)
}

func TestValidateMessage_Equivocation(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)

	var branches []*types.Header
	for {
		_, branches = state.AddBlocksToState(t, st.Block, 8)
		if len(branches) != 0 {
			break
		}
	}

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)

	vote := NewVoteFromHeader(h)
	require.NoError(t, err)

	voter := voters[0]

	gs.votes[voter.key.AsBytes()] = vote

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	msg, err := gs.CreateVoteMessage(branches[0], kr.Alice)
	require.NoError(t, err)

	_, err = gs.ValidateMessage(msg)
	require.Equal(t, ErrEquivocation, err, gs.votes)
}

func TestValidateMessage_BlockDoesNotExist(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)
	state.AddBlocksToState(t, st.Block, 3)

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	fake := &types.Header{
		Number: big.NewInt(77),
	}

	msg, err := gs.CreateVoteMessage(fake, kr.Alice)
	require.NoError(t, err)

	_, err = gs.ValidateMessage(msg)
	require.Equal(t, err, ErrBlockDoesNotExist)
}

func TestValidateMessage_IsNotDescendant(t *testing.T) {
	st := newTestState(t)
	voters := newTestVoters(t)

	gs, err := NewService(st.Block, voters)
	require.NoError(t, err)

	var branches []*types.Header
	for {
		_, branches = state.AddBlocksToState(t, st.Block, 8)
		if len(branches) != 0 {
			break
		}
	}

	h, err := st.Block.BestBlockHeader()
	require.NoError(t, err)
	gs.head = h.Hash()

	kr, err := keystore.NewEd25519Keyring()
	require.NoError(t, err)

	msg, err := gs.CreateVoteMessage(branches[0], kr.Alice)
	require.NoError(t, err)

	_, err = gs.ValidateMessage(msg)
	require.Equal(t, ErrDescendantNotFound, err, gs.votes)
}
