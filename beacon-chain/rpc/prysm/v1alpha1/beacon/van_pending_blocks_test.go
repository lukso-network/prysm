package beacon

import (
	"context"
	"github.com/golang/mock/gomock"
	chainMock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/mock"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	vmock "github.com/prysmaticlabs/prysm/shared/van_mock"
	"testing"
)

// TestServer_StreamNewPendingBlocks_ContextCanceled
func TestServer_StreamNewPendingBlocks_ContextCanceled(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	// Genesis block.
	genesisBlock := testutil.NewBeaconBlock()
	genesisBlockRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesisBlock)))
	require.NoError(t, db.SaveState(ctx, beaconState, genesisBlockRoot))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesisBlockRoot))

	ctx, cancel := context.WithCancel(ctx)

	chainService := &chainMock.ChainService{Block: wrapper.WrappedPhase0SignedBeaconBlock(genesisBlock)}
	server := &Server{
		Ctx:           ctx,
		HeadFetcher:   chainService,
		StateNotifier: chainService.StateNotifier(),
		BlockNotifier: chainService.BlockNotifier(),
		BeaconDB:      db,
	}

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := vmock.NewMockBeaconChain_StreamNewPendingBlocksServer(ctrl)
	mockStream.EXPECT().Context().Return(ctx)
	go func(tt *testing.T) {
		assert.ErrorContains(tt, "Context canceled", server.StreamNewPendingBlocks(&ethpb.StreamPendingBlocksRequest{}, mockStream))
		<-exitRoutine
	}(t)
	cancel()
	exitRoutine <- true
}

func TestServer_StreamNewPendingBlocks_PublishPrevBlocksBatch(t *testing.T) {
	db := dbTest.SetupDB(t)
	params.UseMainnetConfig()
	genBlock := testutil.NewBeaconBlock()
	genBlock.Block.ParentRoot = bytesutil.PadTo([]byte{'G'}, 32)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(genBlock)))
	gRoot, err := genBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), gRoot))

	finalizedBlock := testutil.NewBeaconBlock()
	finalizedBlock.Block.Slot = 32
	finalizedBlock.Block.ParentRoot = bytesutil.PadTo([]byte{'A'}, 32)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(finalizedBlock)))
	fRoot, err := finalizedBlock.Block.HashTreeRoot()
	require.NoError(t, err)

	justifiedBlock := testutil.NewBeaconBlock()
	justifiedBlock.Block.Slot = 64
	justifiedBlock.Block.ParentRoot = bytesutil.PadTo([]byte{'B'}, 32)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(justifiedBlock)))
	jRoot, err := justifiedBlock.Block.HashTreeRoot()
	require.NoError(t, err)

	prevJustifiedBlock := testutil.NewBeaconBlock()
	prevJustifiedBlock.Block.Slot = 96
	prevJustifiedBlock.Block.ParentRoot = bytesutil.PadTo([]byte{'C'}, 32)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(prevJustifiedBlock)))
	pjRoot, err := prevJustifiedBlock.Block.HashTreeRoot()
	require.NoError(t, err)

	s, err := v1.InitializeFromProto(&pbp2p.BeaconState{
		Slot:                        1,
		PreviousJustifiedCheckpoint: &ethpb.Checkpoint{Epoch: 3, Root: pjRoot[:]},
		CurrentJustifiedCheckpoint:  &ethpb.Checkpoint{Epoch: 2, Root: jRoot[:]},
		FinalizedCheckpoint:         &ethpb.Checkpoint{Epoch: 1, Root: fRoot[:]},
	})
	require.NoError(t, err)

	b := testutil.NewBeaconBlock()
	b.Block.Slot, err = helpers.StartSlot(s.PreviousJustifiedCheckpoint().Epoch)
	require.NoError(t, err)

	chainService := &chainMock.ChainService{}
	ctx := context.Background()
	server := &Server{
		Ctx:           ctx,
		HeadFetcher:   &chainMock.ChainService{Block: wrapper.WrappedPhase0SignedBeaconBlock(b), State: s},
		BeaconDB:      db,
		StateNotifier: chainService.StateNotifier(),
		FinalizationFetcher: &chainMock.ChainService{
			FinalizedCheckPoint:         s.FinalizedCheckpoint(),
			CurrentJustifiedCheckPoint:  s.CurrentJustifiedCheckpoint(),
			PreviousJustifiedCheckPoint: s.PreviousJustifiedCheckpoint()},
	}

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mock.NewMockBeaconChain_StreamNewPendingBlocksServer(ctrl)
	mockStream.EXPECT().Send(
		gomock.AssignableToTypeOf(&ethpb.StreamPendingBlockInfo{}),
	).Do(func(args interface{}) {
		exitRoutine <- true
	}).AnyTimes()
	mockStream.EXPECT().Context().Return(ctx).MaxTimes(0)

	go func(tt *testing.T) {
		assert.NoError(tt, server.StreamNewPendingBlocks(&ethpb.StreamPendingBlocksRequest{
			BlockRoot: []byte{},
			FromSlot:  0,
		}, mockStream), "Could not call RPC method")
	}(t)

	<-exitRoutine
}
