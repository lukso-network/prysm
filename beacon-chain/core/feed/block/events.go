// Package block contains types for block-specific events fired
// during the runtime of a beacon node.
package block

import (
	"github.com/prysmaticlabs/prysm/proto/interfaces"
)

const (
	// ReceivedBlock is sent after a block has been received by the beacon node via p2p or RPC.
	ReceivedBlock = iota + 1
	// UnconfirmedBlock is sent after a block has been processed by the beacon node but not confirmed by orchestrator
	UnConfirmedBlock
	// ConfirmedBlock is sent after a block has been confirmed by orchestrator client
	ConfirmedBlock
)

// ReceivedBlockData is the data sent with ReceivedBlock events.
type ReceivedBlockData struct {
	SignedBlock interfaces.SignedBeaconBlock
}

// UnConfirmedBlockData is the data sent to orchestrator
type UnConfirmedBlockData struct {
	Block interfaces.BeaconBlock
}
