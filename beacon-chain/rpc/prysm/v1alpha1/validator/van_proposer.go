package validator

import (
	"context"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/interop"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpdateStateRoot
func (vs *Server) UpdateStateRoot(ctx context.Context, blk *ethpb.BeaconBlock) (*ethpb.BeaconBlock, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.UpdateStateRoot")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("slot", int64(blk.Slot)))

	// Compute state root with the newly constructed block.
	stateRoot, err := vs.computeStateRoot(ctx, wrapper.WrappedPhase0SignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: blk, Signature: make([]byte, 96)}))
	if err != nil {
		interop.WriteBlockToDisk(wrapper.WrappedPhase0SignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: blk}), true /*failed*/)
		return nil, status.Errorf(codes.Internal, "Could not compute state root: %v", err)
	}
	blk.StateRoot = stateRoot

	return blk, nil
}
