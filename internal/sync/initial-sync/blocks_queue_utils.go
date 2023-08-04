package initialsync

import (
	"context"
	"errors"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
)

// resetWithBlocks removes all state machines, then re-adds enough machines to contain all provided
// blocks (machines are set into stateDataParsed state, so that their content is immediately
// consumable). It is assumed that blocks come in an ascending order.
func (q *blocksQueue) resetFromFork(fork *forkData) error {
	if fork == nil {
		return errors.New("nil fork data")
	}
	if len(fork.blocks) == 0 {
		return errors.New("no blocks to reset from")
	}
	firstBlock := fork.blocks[0]
	if firstBlock == nil {
		return errors.New("invalid first block in fork data")
	}

	blocksPerRequest := q.blocksFetcher.blocksPerPeriod
	if err := q.smm.removeAllStateMachines(); err != nil {
		return err
	}
	firstBlockNr := utils.ConvertH256ToUint256Int(firstBlock.Header.Number)
	fsm := q.smm.addStateMachine(firstBlockNr)
	fsm.pid = fork.peer
	fsm.blocks = fork.blocks
	fsm.state = stateDataParsed

	// The rest of machines are in skipped state.
	startBlockNr := new(uint256.Int).AddUint64(firstBlockNr, uint64(len(fork.blocks)))
	for i := startBlockNr.Clone(); i.Cmp(new(uint256.Int).AddUint64(startBlockNr, blocksPerRequest*(lookaheadSteps-1))) == -1; i.AddUint64(i, blocksPerRequest) {

		fsm := q.smm.addStateMachine(i)
		fsm.state = stateSkipped
	}
	return nil
}

// resetFromBlockNr removes all state machines, and re-adds them starting with a given BlockNr.
func (q *blocksQueue) resetFromBlockNr(ctx context.Context, startBlockNr *uint256.Int) error {
	// Shift start position of all the machines except for the last one.
	blocksPerRequest := q.blocksFetcher.blocksPerPeriod
	if err := q.smm.removeAllStateMachines(); err != nil {
		return err
	}
	for i := startBlockNr.Clone(); i.Cmp(new(uint256.Int).AddUint64(startBlockNr, blocksPerRequest*(lookaheadSteps-1))) == -1; i.AddUint64(i, blocksPerRequest) {
		q.smm.addStateMachine(i)
	}

	if q.highestExpectedBlockNr.Cmp(q.blocksFetcher.bestFinalizedBlockNr()) == -1 {
		q.highestExpectedBlockNr = q.blocksFetcher.bestFinalizedBlockNr()
	}
	return nil
}
