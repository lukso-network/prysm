package beacon

import (
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamNewPendingBlocks to orchestrator client every single time an unconfirmed block is received by the beacon node.
func (bs *Server) StreamNewPendingBlocks(request *ethpb.StreamPendingBlocksRequest, stream ethpb.BeaconChain_StreamNewPendingBlocksServer) error {
	batchSender := func(start, end types.Epoch) error {
		for i := start; i <= end; i++ {
			blks, _, err := bs.BeaconDB.Blocks(bs.Ctx, filters.NewFilter().SetStartEpoch(i).SetEndEpoch(i))

			log.WithField("blocksLen", len(blks)).
				WithField("startEpoch", start).WithField("endEpoch", end).
				Debug("blocks from batch sender")

			if err != nil {
				return status.Errorf(codes.Internal,
					"Could not send over stream: %v", err)
			}
			for _, blk := range blks {
				// we do not send block #0 to orchestrator
				if blk.Block.Slot == 0 {
					continue
				}
				if err := stream.Send(blk.Block); err != nil {
					return status.Errorf(codes.Unavailable,
						"Could not send over stream: %v", err)
				}
			}
		}
		return nil
	}

	sender := func(start, end types.Slot) error {
		blks, _, err := bs.BeaconDB.Blocks(bs.Ctx, filters.NewFilter().SetStartSlot(start).SetEndSlot(end))
		if err != nil {
			return err
		}

		log.WithField("blocksLen", len(blks)).
			WithField("startEpoch", start).WithField("endEpoch", end).
			Debug("blocks from sender")

		for _, blk := range blks {
			if err := stream.Send(blk.Block); err != nil {
				return status.Errorf(codes.Unavailable,
					"Could not send over stream: %v", err)
			}
		}
		return nil
	}

	cp, err := bs.BeaconDB.FinalizedCheckpoint(bs.Ctx)
	if err != nil {
		return status.Errorf(codes.Internal,
			"Could not retrieve finalize epoch: %v", err)
	}

	epochStart := helpers.SlotToEpoch(request.FromSlot)
	epochEnd := cp.Epoch

	if epochStart <= epochEnd {
		if err := batchSender(epochStart, epochEnd); err != nil {
			return err
		}
	}
	// Getting un-confirmed blocks from cache and sends those blocks to orchestrator
	pBlocks, err := bs.UnconfirmedBlockFetcher.SortedUnConfirmedBlocksFromCache()
	if err != nil {
		return status.Errorf(codes.Internal,
			"Could not send over stream: %v", err)
	}

	startSlot, err := helpers.EndSlot(epochEnd)
	if err != nil {
		return status.Errorf(codes.Internal,
			"Could not retrieve end slot number: %v", err)
	}

	endSlot := types.Slot(0)

	if len(pBlocks) > 0 {
		for _, blk := range pBlocks {
			if err := stream.Send(blk); err != nil {
				return status.Errorf(codes.Unavailable,
					"Could not send over stream: %v", err)
			}
		}
		endSlot = pBlocks[0].Slot
		if startSlot+1 < endSlot {
			if err := sender(startSlot, endSlot); err != nil {
				return err
			}
		}
	}

	pBlockCh := make(chan *feed.Event, 1)
	pBlockSub := bs.BlockNotifier.BlockFeed().Subscribe(pBlockCh)
	firstTime := true
	defer pBlockSub.Unsubscribe()

	for {
		select {
		case blockEvent := <-pBlockCh:
			if blockEvent.Type == blockfeed.UnConfirmedBlock {
				data, ok := blockEvent.Data.(*blockfeed.UnConfirmedBlockData)
				if !ok || data == nil {
					continue
				}

				if firstTime {
					firstTime = false
					startSlot = endSlot + 1
					endSlot = data.Block.Slot
					if startSlot < endSlot {
						if err := sender(startSlot, endSlot); err != nil {
							return err
						}
					}
				}

				if err := stream.Send(data.Block); err != nil {
					return status.Errorf(codes.Unavailable,
						"Could not send over stream: %v", err)
				}

				log.WithField("slot", data.Block.Slot).Debug(
					"New pending block has been published successfully")
			}
		case <-pBlockSub.Err():
			return status.Error(codes.Aborted, "Subscriber closed, exiting goroutine")
		case <-bs.Ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Context canceled")
		}
	}
}
