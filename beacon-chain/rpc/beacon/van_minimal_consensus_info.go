package beacon

import (
	"encoding/hex"
	"fmt"
	types2 "github.com/gogo/protobuf/types"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	alreadySendEpochInfos map[types.Epoch]bool
)

// StreamMinimalConsensusInfo to orchestrator client every single time an unconfirmed block is received by the beacon node.
func (bs *Server) StreamMinimalConsensusInfo(
	req *ethpb.MinimalConsensusInfoRequest,
	stream ethpb.BeaconChain_StreamMinimalConsensusInfoServer,
) error {

	sender := func(epoch types.Epoch, state iface.BeaconState) error {
		if !alreadySendEpochInfos[epoch] {
			epochInfo, err := bs.prepareEpochInfo(epoch, state)
			if err != nil {
				return status.Errorf(codes.Internal,
					"Could not send over stream: %v", err)
			}
			if err := stream.Send(epochInfo); err != nil {
				return status.Errorf(codes.Unavailable,
					"Could not send over stream: %v  err: %v", epoch, err)
			}
			log.WithField("epoch", epoch).Info("Sent epoch info to orchestrator")
			alreadySendEpochInfos[epochInfo.Epoch] = true
		}
		return nil
	}

	batchSender := func(start, end types.Epoch) error {
		for i := start; i <= end; i++ {
			startSlot, err := helpers.StartSlot(i)
			if err != nil {
				return status.Errorf(codes.Internal,
					"Could not send over stream: %v", err)
			}
			state, err := bs.StateGen.StateBySlot(bs.Ctx, startSlot)
			if err != nil {
				return status.Errorf(codes.Internal,
					"Could not send over stream: %v", err)
			}
			if state != nil {
				if err := sender(i, state); err != nil {
					return err
				}
			}
		}
		return nil
	}

	cp, err := bs.BeaconDB.FinalizedCheckpoint(bs.Ctx)
	if err != nil {
		return status.Errorf(codes.Internal,
			"Could not send over stream: %v", err)
	}

	startEpoch := req.FromEpoch
	endEpoch := cp.Epoch
	alreadySendEpochInfos = make(map[types.Epoch]bool)
	hasSendPrevEpochs := false
	log.WithField("startEpoch", startEpoch).
		WithField("endEpoch", endEpoch).
		Debug("Sending previous epoch infos")
	if startEpoch <= endEpoch {
		if err := batchSender(startEpoch, endEpoch); err != nil {
			return err
		}
		hasSendPrevEpochs = true
	}

	stateChannel := make(chan *feed.Event, 1)
	stateSub := bs.StateNotifier.StateFeed().Subscribe(stateChannel)
	firstTime := true
	defer stateSub.Unsubscribe()

	for {
		select {
		case stateEvent := <-stateChannel:
			if stateEvent.Type == statefeed.BlockVerified {
				blockVerifiedData, ok := stateEvent.Data.(*statefeed.BlockPreVerifiedData)
				if !ok {
					log.Warn("Failed to send epoch info to orchestrator")
					continue
				}
				curEpoch := helpers.SlotToEpoch(blockVerifiedData.Slot)
				nextEpoch := curEpoch + 1
				// Executes for a single time
				if firstTime && hasSendPrevEpochs {
					firstTime = false
					log.WithField("startEpoch", endEpoch+1).
						WithField("endEpoch", curEpoch).
						Debug("Sending left over epoch infos")
					if endEpoch+1 < curEpoch {
						startEpoch = endEpoch + 1
						endEpoch = curEpoch
						curState := blockVerifiedData.CurrentState
						for i := startEpoch; i <= endEpoch; i++ {
							if err := sender(i, curState); err != nil {
								return err
							}
						}
					}
				}
				if hasSendPrevEpochs {
					if err := sender(nextEpoch, blockVerifiedData.CurrentState); err != nil {
						return err
					}
				}
			}
		case <-stateSub.Err():
			return status.Error(codes.Aborted, "Subscriber closed, exiting go routine")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Stream context canceled")
		case <-bs.Ctx.Done():
			return status.Error(codes.Canceled, "RPC context canceled")
		}
	}
}

// prepareEpochInfo
func (bs *Server) prepareEpochInfo(epoch types.Epoch, s iface.BeaconState) (*ethpb.MinimalConsensusInfo, error) {
	// Advance state with empty transitions up to the requested epoch start slot.
	startSlot, err := helpers.StartSlot(epoch)
	if err != nil {
		return nil, err
	}
	if s.Slot() < startSlot {
		s, err = state.ProcessSlots(bs.Ctx, s, startSlot)
		if err != nil {
			return nil, err
		}
	}
	proposerAssignmentInfo, err := helpers.ProposerAssignments(s, epoch)
	if err != nil {
		return nil, err
	}

	epochStartTime, err := helpers.SlotToTime(uint64(bs.GenesisTimeFetcher.GenesisTime().Unix()), startSlot)
	if nil != err {
		return nil, err
	}

	validatorList, err := prepareSortedValidatorList(epoch, proposerAssignmentInfo)
	if err != nil {
		return nil, err
	}

	return &ethpb.MinimalConsensusInfo{
		Epoch:            epoch,
		ValidatorList:    validatorList,
		EpochTimeStart:   uint64(epochStartTime.Unix()),
		SlotTimeDuration: &types2.Duration{Seconds: int64(params.BeaconConfig().SecondsPerSlot)},
	}, nil
}

// prepareEpochInfo
func prepareSortedValidatorList(
	epoch types.Epoch,
	proposerAssignmentInfo []*ethpb.ValidatorAssignments_CommitteeAssignment,
) ([]string, error) {

	publicKeyList := make([]string, 0)
	slotToPubKeyMapping := make(map[types.Slot]string)

	for _, assignment := range proposerAssignmentInfo {
		for _, slot := range assignment.ProposerSlots {
			slotToPubKeyMapping[slot] = fmt.Sprintf("0x%s", hex.EncodeToString(assignment.PublicKey))
		}
	}

	if epoch == 0 {
		publicKeyBytes := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		emptyPubKey := fmt.Sprintf("0x%s", hex.EncodeToString(publicKeyBytes))
		slotToPubKeyMapping[0] = emptyPubKey
	}

	startSlot, err := helpers.StartSlot(epoch)
	if err != nil {
		return []string{}, err
	}

	endSlot, err := helpers.EndSlot(epoch)
	if err != nil {
		return []string{}, err
	}

	for slot := startSlot; slot <= endSlot; slot++ {
		publicKeyList = append(publicKeyList, slotToPubKeyMapping[slot])
	}
	return publicKeyList, nil
}
