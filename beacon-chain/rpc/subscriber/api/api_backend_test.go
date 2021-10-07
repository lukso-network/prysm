package api

import (
	"context"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/beacon"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/subscriber/api/events"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestAPIBackend_SubscribeNewEpochEvent(t *testing.T) {
	consensusChannel := make(chan interface{})
	helpers.ClearCache()
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	config := params.BeaconConfig().Copy()
	oldConfig := config.Copy()
	config.SlotsPerEpoch = 32
	params.OverrideBeaconConfig(config)

	defer func() {
		params.OverrideBeaconConfig(oldConfig)
	}()
	testStartTime := time.Now()

	stateNotifier := new(mock.ChainService).StateNotifier()
	state, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, state.SetSlot(0))

	bs := &beacon.Server{
		BeaconDB: db,
		Ctx:      context.Background(),
		FinalizationFetcher: &mock.ChainService{
			Genesis: testStartTime,
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: testStartTime,
		},
		StateGen:      stategen.New(db),
		StateNotifier: stateNotifier,
		HeadFetcher: &mock.ChainService{
			State: state,
		},
	}

	stateFeed := stateNotifier.StateFeed()

	apiBackend := APIBackend{
		BeaconChain: *bs,
	}

	apiBackend.SubscribeNewEpochEvent(ctx, types.Epoch(0), consensusChannel)
	shouldGather := 1
	received := make([]*events.MinimalEpochConsensusInfo, 0)

	var sendWaitGroup sync.WaitGroup
	sendWaitGroup.Add(shouldGather)

	go func() {
		for {
			consensusInfo := <-consensusChannel

			if nil != consensusInfo {
				received = append(received, consensusInfo.(*events.MinimalEpochConsensusInfo))
				sendWaitGroup.Done()
			}
		}
	}()

	ticker := time.NewTicker(time.Second * 5)
	sent := 0
	sendUntilTimeout := func() {
		for sent == 0 {
			select {
			case <-ticker.C:
				t.FailNow()
			default:
				sent = stateFeed.Send(&feed.Event{
					Type: statefeed.BlockProcessed,
					Data: &statefeed.BlockProcessedData{},
				})
			}
		}
	}

	// Should not send because epoch not increased
	sendUntilTimeout()
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, len(received))

	// Should send because epoch increased
	require.NoError(t, state.SetSlot(params.BeaconConfig().SlotsPerEpoch))
	sendUntilTimeout()
	sendWaitGroup.Wait()
	assert.Equal(t, 1, sent)
	assert.Equal(t, shouldGather, len(received))
}
