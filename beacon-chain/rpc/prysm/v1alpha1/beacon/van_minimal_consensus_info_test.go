package beacon

import (
	"context"
	"github.com/golang/mock/gomock"
	mockChain "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/mock"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"testing"
	"time"
)

func TestServer_StreamMinimalConsensusInfo_ContextCanceled(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	chainService := &mockChain.ChainService{}
	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		Ctx:                 ctx,
		StateNotifier:       chainService.StateNotifier(),
		HeadFetcher:         chainService,
		BeaconDB:            db,
		StateGen:            stategen.New(db),
		PendingQueueFetcher: chainService,
	}

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mock.NewMockBeaconChain_StreamMinimalConsensusInfoServer(ctrl)
	mockStream.EXPECT().Context().Return(ctx)
	go func(tt *testing.T) {
		assert.ErrorContains(tt, "Canceled", server.StreamMinimalConsensusInfo(&ethpb.MinimalConsensusInfoRequest{
			FromEpoch: 1,
		}, mockStream))
		<-exitRoutine
	}(t)
	cancel()
	exitRoutine <- true
}

func TestServer_StreamMinimalConsensusInfo_PreviousEpochInfos(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(64)
	stateWithValidators, _ := testutil.DeterministicGenesisState(t, validators)
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))

	// Genesis block.
	genesisBlock := testutil.NewBeaconBlock()
	genesisBlockRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesisBlock)))
	require.NoError(t, db.SaveState(ctx, beaconState, genesisBlockRoot))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesisBlockRoot))

	c := &mockChain.ChainService{
		Genesis: time.Now(),
	}
	chainService := &mockChain.ChainService{}
	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		Ctx:                 ctx,
		StateNotifier:       chainService.StateNotifier(),
		HeadFetcher:         chainService,
		BeaconDB:            db,
		StateGen:            stategen.New(db),
		GenesisTimeFetcher:  c,
		PendingQueueFetcher: chainService,
	}
	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mock.NewMockBeaconChain_StreamMinimalConsensusInfoServer(ctrl)
	mockStream.EXPECT().Send(gomock.Any()).Do(func(arg0 interface{}) {
		exitRoutine <- true
	})
	mockStream.EXPECT().Context().Return(ctx).AnyTimes()
	go func(tt *testing.T) {
		assert.ErrorContains(tt, "Canceled", server.StreamMinimalConsensusInfo(&ethpb.MinimalConsensusInfoRequest{
			FromEpoch: 0,
		}, mockStream))
	}(t)
	<-exitRoutine
	cancel()
}

func TestServer_StreamMinimalConsensusInfo_PublishCurEpochInfo(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(64)
	stateWithValidators, _ := testutil.DeterministicGenesisState(t, validators)
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))

	// Genesis block.
	genesisBlock := testutil.NewBeaconBlock()
	genesisBlockRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesisBlock)))
	require.NoError(t, db.SaveState(ctx, beaconState, genesisBlockRoot))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesisBlockRoot))

	c := &mockChain.ChainService{
		Genesis: time.Now(),
	}
	chainService := &mockChain.ChainService{}
	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		Ctx:                 ctx,
		StateNotifier:       chainService.StateNotifier(),
		HeadFetcher:         chainService,
		BeaconDB:            db,
		StateGen:            stategen.New(db),
		GenesisTimeFetcher:  c,
		PendingQueueFetcher: chainService,
	}

	// retrieve proposer
	proposerIndices, pubKeys, err := helpers.ProposerIndicesInCache(beaconState, 0)
	require.NoError(t, err)

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mock.NewMockBeaconChain_StreamMinimalConsensusInfoServer(ctrl)
	mockStream.EXPECT().Send(gomock.Any()).Do(func(arg0 interface{}) {
		exitRoutine <- true
	})
	mockStream.EXPECT().Context().Return(ctx).AnyTimes()
	go func(tt *testing.T) {
		assert.ErrorContains(tt, "Canceled", server.StreamMinimalConsensusInfo(&ethpb.MinimalConsensusInfoRequest{
			FromEpoch: 10,
		}, mockStream))
	}(t)

	// Fire a reorg event. This needs to trigger
	// a recomputation and resending of duties over the stream.
	for sent := 0; sent == 0; {
		sent = server.StateNotifier.StateFeed().Send(&feed.Event{
			Type: statefeed.EpochInfo,
			Data: &statefeed.EpochInfoData{
				Slot:            63,
				ProposerIndices: proposerIndices,
				PublicKeys:      pubKeys,
			},
		})
	}
	<-exitRoutine
	cancel()
}
