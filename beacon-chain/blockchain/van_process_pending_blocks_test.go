package blockchain

import (
	"context"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	vanTypes "github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/van_mock"
	"math/big"
	"sort"
	"testing"
	"time"
)

// TestService_PublishAndStorePendingBlock checks PublishAndStorePendingBlock method
func TestService_PublishBlock(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	cfg := &Config{
		BeaconDB:      beaconDB,
		StateGen:      stategen.New(beaconDB),
		BlockNotifier: &mock.MockBlockNotifier{RecordEvents: true},
		StateNotifier: &mock.MockStateNotifier{RecordEvents: true},
	}
	s, err := NewService(ctx, cfg)
	require.NoError(t, err)
	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	wrappedGenesisBlk := wrapper.WrappedPhase0SignedBeaconBlock(genesis)
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrappedGenesisBlk))
	require.NoError(t, err)
	b := testutil.NewBeaconBlock()
	wrappedBlk := wrapper.WrappedPhase0SignedBeaconBlock(b)
	s.publishBlock(wrappedBlk)
	time.Sleep(3 * time.Second)
	if recvd := len(s.blockNotifier.(*mock.MockBlockNotifier).ReceivedEvents()); recvd < 1 {
		t.Errorf("Received %d pending block notifications, expected at least 1", recvd)
	}
}

// TestService_SortedUnConfirmedBlocksFromCache checks SortedUnConfirmedBlocksFromCache method
func TestService_SortedUnConfirmedBlocksFromCache(t *testing.T) {
	ctx := context.Background()
	s, err := NewService(ctx, &Config{})
	require.NoError(t, err)
	blks := make([]interfaces.BeaconBlock, 10)
	for i := 0; i < 10; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(10 - i)
		wrappedBlk := wrapper.WrappedPhase0BeaconBlock(b.Block)
		blks[i] = wrappedBlk
		require.NoError(t, s.pendingBlockCache.AddPendingBlock(wrappedBlk))
	}
	sort.Slice(blks, func(i, j int) bool {
		return blks[i].Slot() < blks[j].Slot()
	})
	sortedBlocks, err := s.SortedUnConfirmedBlocksFromCache()
	require.NoError(t, err)
	require.DeepEqual(t, blks, sortedBlocks)
}

// TestService_fetchOrcConfirmations checks fetchOrcConfirmations
func TestService_fetchOrcConfirmations(t *testing.T) {
	ctx := context.Background()
	var mockedOrcClient *van_mock.MockClient
	ctrl := gomock.NewController(t)
	mockedOrcClient = van_mock.NewMockClient(ctrl)
	cfg := &Config{
		BlockNotifier:      &mock.MockBlockNotifier{RecordEvents: true},
		OrcRPCClient:       mockedOrcClient,
		EnableVanguardNode: true,
	}
	confirmationStatus := make([]*vanTypes.ConfirmationResData, 10)
	for i := 0; i < 10; i++ {
		confirmationStatus[i] = &vanTypes.ConfirmationResData{Slot: types.Slot(i), Status: vanTypes.Verified}
	}
	mockedOrcClient.EXPECT().ConfirmVanBlockHashes(
		gomock.Any(),
		gomock.Any(),
	).AnyTimes().Return(confirmationStatus, nil)
	s, err := NewService(ctx, cfg)
	go s.processOrcConfirmationRoutine()
	require.NoError(t, err)
	blks := make([]interfaces.BeaconBlock, 10)
	for i := 0; i < 10; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(i)
		wrappedBlk := wrapper.WrappedPhase0BeaconBlock(b.Block)
		blks[i] = wrappedBlk
		confirmationStatus[i] = &vanTypes.ConfirmationResData{Slot: types.Slot(i), Status: vanTypes.Verified}
		require.NoError(t, s.pendingBlockCache.AddPendingBlock(wrappedBlk))
	}
}

func TestService_VerifyPandoraShardInfo(t *testing.T) {
	ctx := context.Background()
	//var mockClient *van_mock.MockClient
	ctrl := gomock.NewController(t)
	mockedOrcClient := van_mock.NewMockClient(ctrl)
	cfg := &Config{
		BlockNotifier:      &mock.MockBlockNotifier{RecordEvents: true},
		OrcRPCClient:       mockedOrcClient,
		EnableVanguardNode: true,
	}
	s, err := NewService(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, s)

	t.Run("should throw an error when signed block is empty", func(t *testing.T) {
		signedBlock := &eth.SignedBeaconBlock{}
		currentErr := s.VerifyPandoraShardInfo(signedBlock)
		require.Equal(t, errInvalidPandoraShardInfo, currentErr)
	})

	t.Run("should throw an error when signed block is without sharding part", func(t *testing.T) {
		signedBlock := &eth.SignedBeaconBlock{Block: &eth.BeaconBlock{}}
		currentErr := s.VerifyPandoraShardInfo(signedBlock)
		require.Equal(t, errInvalidPandoraShardInfo, currentErr)
	})

	wrappedBlock := wrapper.WrappedPhase0SignedBeaconBlock(testutil.NewBeaconBlock())
	s.head = &head{block: wrappedBlock}

	t.Run("should throw an error when head block lacks the sharding info", func(t *testing.T) {
		pandoraShards := make([]*eth.PandoraShard, 1)
		signedBlock := &eth.SignedBeaconBlock{Block: &eth.BeaconBlock{
			Body: &eth.BeaconBlockBody{PandoraShard: pandoraShards},
		}}
		currentErr := s.VerifyPandoraShardInfo(signedBlock)
		require.Equal(t, errInvalidPandoraShardInfo, currentErr)
	})

	wrappedBlock = wrapper.WrappedPhase0SignedBeaconBlock(testutil.NewBeaconBlockWithPandoraSharding(
		&gethTypes.Header{Number: big.NewInt(25)},
		types.Slot(5),
	))
	s.head = &head{block: wrappedBlock}

	t.Run("should throw an error with invalid pandora shard", func(t *testing.T) {
		signedBlock := &eth.SignedBeaconBlock{Block: &eth.BeaconBlock{}}
		currentErr := s.VerifyPandoraShardInfo(signedBlock)
		require.Equal(t, errInvalidPandoraShardInfo, currentErr)
	})
}

func TestGuardPandoraShardHeader(t *testing.T) {
	pandoraBlock := &ethpb.PandoraShard{}

	t.Run("should throw an error when hash is empty", func(t *testing.T) {
		require.Equal(t, errInvalidPandoraShardInfo, GuardPandoraShardHeader(pandoraBlock))
	})

	pandoraBlock.Hash = []byte("0xde6f0b6c17077334abd585da38b251871251cb26fa3456be135825ea45c06f12")

	t.Run("should throw an error when parent hash is empty", func(t *testing.T) {
		require.Equal(t, errInvalidPandoraShardInfo, GuardPandoraShardHeader(pandoraBlock))
	})

	pandoraBlock.ParentHash = []byte("0x67b96c7bbdbf2186c868ac7565a24d250c8ecbf4f43cb50bd78f11b73681c025")

	t.Run("should pass when parent hash and hash is not empty", func(t *testing.T) {
		require.NoError(t, GuardPandoraShardHeader(pandoraBlock))
	})
}

// TestService_waitForConfirmationBlock checks waitForConfirmationBlock method
// When the confirmation result of the block is verified then waitForConfirmationBlock gives you error return
// Not delete the invalid block because, when node gets an valid block, then it will be replaced and then it will be deleted
func TestService_waitForConfirmationBlock(t *testing.T) {
	tests := []struct {
		name                 string
		pendingBlocksInQueue []interfaces.SignedBeaconBlock
		incomingBlock        interfaces.SignedBeaconBlock
		confirmationStatus   []*vanTypes.ConfirmationResData
		expectedOutput       string
	}{
		{
			name:                 "Returns nil when orchestrator sends verified status for all blocks",
			pendingBlocksInQueue: getBeaconBlocks(0, 3),
			incomingBlock:        getBeaconBlock(2),
			confirmationStatus: []*vanTypes.ConfirmationResData{
				{
					Slot:   0,
					Status: vanTypes.Verified,
				},
				{
					Slot:   1,
					Status: vanTypes.Verified,
				},
				{
					Slot:   2,
					Status: vanTypes.Verified,
				},
			},
			expectedOutput: "",
		},
		{
			name:                 "Returns error when orchestrator sends invalid status",
			pendingBlocksInQueue: getBeaconBlocks(0, 3),
			incomingBlock:        getBeaconBlock(1),
			confirmationStatus: []*vanTypes.ConfirmationResData{
				{
					Slot:   0,
					Status: vanTypes.Verified,
				},
				{
					Slot:   1,
					Status: vanTypes.Invalid,
				},
				{
					Slot:   2,
					Status: vanTypes.Verified,
				},
			},
			expectedOutput: "invalid block found in orchestrator",
		},
		{
			name:                 "Retry for the block with pending status",
			pendingBlocksInQueue: getBeaconBlocks(0, 3),
			incomingBlock:        getBeaconBlock(1),
			confirmationStatus: []*vanTypes.ConfirmationResData{
				{
					Slot:   0,
					Status: vanTypes.Verified,
				},
				{
					Slot:   1,
					Status: vanTypes.Pending,
				},
				{
					Slot:   2,
					Status: vanTypes.Verified,
				},
			},
			expectedOutput: "maximum wait is exceeded and orchestrator can not verify the block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var mockedOrcClient *van_mock.MockClient
			ctrl := gomock.NewController(t)
			mockedOrcClient = van_mock.NewMockClient(ctrl)
			cfg := &Config{
				BlockNotifier:      &mock.MockBlockNotifier{},
				OrcRPCClient:       mockedOrcClient,
				EnableVanguardNode: true,
			}
			s, err := NewService(ctx, cfg)
			require.NoError(t, err)
			go s.processOrcConfirmationRoutine()
			mockedOrcClient.EXPECT().ConfirmVanBlockHashes(
				gomock.Any(),
				gomock.Any(),
			).AnyTimes().Return(tt.confirmationStatus, nil)
			for i := 0; i < len(tt.pendingBlocksInQueue); i++ {
				require.NoError(t, s.pendingBlockCache.AddPendingBlock(tt.pendingBlocksInQueue[i].Block()))
			}
			if tt.expectedOutput == "" {
				require.NoError(t, s.waitForConfirmationBlock(ctx, tt.incomingBlock))
			} else {
				require.ErrorContains(t, tt.expectedOutput, s.waitForConfirmationBlock(ctx, tt.incomingBlock))
			}
		})
	}
}

// Helper method to generate pending queue with random blocks
func getBeaconBlocks(from, to int) []interfaces.SignedBeaconBlock {
	pendingBlks := make([]interfaces.SignedBeaconBlock, to-from)
	for i := 0; i < to-from; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(from + i)
		wrappedBlk := wrapper.WrappedPhase0SignedBeaconBlock(b)
		pendingBlks[i] = wrappedBlk
	}
	return pendingBlks
}

// Helper method to generate pending queue with random block
func getBeaconBlock(slot types.Slot) interfaces.SignedBeaconBlock {
	b := testutil.NewBeaconBlock()
	b.Block.Slot = types.Slot(slot)
	return wrapper.WrappedPhase0SignedBeaconBlock(b)
}
