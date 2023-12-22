package sync

import (
	"context"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/log"
	"google.golang.org/protobuf/proto"
	"strings"
)

func (s *Service) blockSubscriber(ctx context.Context, msg proto.Message) error {

	iBlock := new(block2.Block)
	if err := iBlock.FromProtoMessage(msg); err != nil {
		return err
	}

	blocks := make([]block2.IBlock, 0)
	blocks = append(blocks, iBlock)

	log.Debug("Subscriber new Block", "hash", iBlock.Header().Hash(), "blockNr", iBlock.Header().Number64().Uint64())

	if _, err := s.cfg.chain.InsertChain(blocks); err != nil {
		// todo bad block
		if !strings.Contains(err.Error(), "blockchain is stopped") {
			s.setBadBlock(ctx, iBlock.Hash())
		}
		return err
	}
	return nil
}
