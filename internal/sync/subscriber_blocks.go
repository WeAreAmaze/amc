package sync

import (
	"context"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/golang/protobuf/proto"
)

func (s *Service) blockSubscriber(ctx context.Context, msg proto.Message) error {

	var iBlock block2.IBlock
	if err := iBlock.FromProtoMessage(msg); err != nil {
		return err
	}

	blocks := make([]block2.IBlock, 0)
	blocks = append(blocks, iBlock)

	if _, err := s.cfg.chain.InsertChain(blocks); err != nil {
		// todo bad block
		//if errors.Is(err, Badblock) {
		s.setBadBlock(ctx, iBlock.Hash())
		return err
	}
	return nil
}