package p2p

import (
	"github.com/amazechain/amc/utils"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
)

// MsgID is a content addressable ID function.
// `SHA256(message.data)[:20]`.
func MsgID(genesisValidatorsRoot []byte, pmsg *pubsubpb.Message) string {
	h := utils.Hash(pmsg.Data)
	return string(h[:20])
}
