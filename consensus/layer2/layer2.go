package layer2

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/layer2"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ontology-layer-2/rollup-contracts/store"
	"github.com/ontology-layer-2/rollup-contracts/store/schema"
)

type Layer2Instant struct {
	Timestamp           uint64
	L2CrossLayerWitness common.Address
	FeeCollector        common.Address
}

func New(config *params.Layer2InstantConfig, db ethdb.Database) *Layer2Instant {
	return &Layer2Instant{
		Timestamp:           uint64(time.Now().Unix()),
		L2CrossLayerWitness: config.L2CrossLayerWitness,
		FeeCollector:        config.FeeCollector,
	}
}

func (self *Layer2Instant) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

func (self *Layer2Instant) verifyHeader(header, parent *types.Header) error {
	if header.Coinbase != self.FeeCollector {
		return fmt.Errorf("invalid coinbase, expected: %s, found: %s", self.FeeCollector, header.Coinbase)
	}
	if header.Difficulty.Sign() != 0 {
		return fmt.Errorf("invalid difficulty, expected: 0, found: %d", header.Difficulty)
	}
	if len(header.Extra) != 0 {
		return fmt.Errorf("extra data not nil, found: %x", header.Extra)
	}

	currNumber := header.Number.Uint64()
	if parent == nil || parent.Number.Uint64() != currNumber-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}

	return nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (self *Layer2Instant) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	currNumber := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, currNumber-1)
	return self.verifyHeader(header, parent)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (self *Layer2Instant) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			var err error
			if i == 0 {
				err = self.VerifyHeader(chain, header, seals[i])
			} else {
				err = self.verifyHeader(header, headers[i-1])
			}

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (self *Layer2Instant) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if block.UncleHash() != types.CalcUncleHash(nil) || len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}

	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (self *Layer2Instant) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	if header.Time == 0 {
		return errors.New("header's timestamp must be set")
	}
	header.Coinbase = self.FeeCollector
	header.Difficulty = big.NewInt(1)
	header.Extra = nil
	currNumber := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, currNumber-1)
	if parent == nil || parent.Number.Uint64() != currNumber-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	header.Nonce = parent.Nonce         // we use nonce to save mmr size
	header.MixDigest = parent.MixDigest // we use mixdigest to save mmr root

	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// but does not assemble the block.
//
// Note: The block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (self *Layer2Instant) Finalize(chain consensus.ChainHeaderReader, header *types.Header,
	state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)
}

// FinalizeAndAssemble runs any post-transaction state modifications (e.g. block
// rewards) and assembles the final block.
//
// Note: The block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (self *Layer2Instant) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	self.Finalize(chain, header, state, txs, uncles)
	mmrSize, mmrRoot := self.ExtractMMR(receipts)
	if mmrSize != 0 {
		if mmrSize <= header.Nonce.Uint64() {
			return nil, fmt.Errorf("mmr size not increase, prev: %d, curr:%d", header.Nonce.Uint64(), mmrSize)
		}
		header.Nonce = types.EncodeNonce(mmrSize)
		header.MixDigest = mmrRoot
	}

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts, trie.NewStackTrie(nil)), nil
}

func (self *Layer2Instant) ExtractMMR(receipts []*types.Receipt) (uint64, common.Hash) {
	// figure out if there is any MessageSent event
	mmrSize := uint64(0)
	var mmrRoot common.Hash
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			if log.Address == self.L2CrossLayerWitness && log.Topics[0] == layer2.MessageSentEventID {
				event := layer2.MustParseMessageSentEvent(log)
				mmrSize = event.MessageIndex + 1
				mmrRoot = event.MmrRoot
			}
		}
	}

	return mmrSize, mmrRoot
}

// Seal generates a new sealing request for the given input block and pushes
// the result into the given channel.
//
// Note, the method returns immediately and will send the result async. More
// than one result may also be returned depending on the consensus algorithm.
func (self *Layer2Instant) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	// sealed in FinalizeAndAssemble
	results <- block

	return nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (self *Layer2Instant) SealHash(header *types.Header) common.Hash {
	return header.Hash()
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (self *Layer2Instant) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

// APIs returns the RPC APIs this consensus engine provides.
func (self *Layer2Instant) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return nil
}

// Close terminates any background threads maintained by the consensus engine.
func (self *Layer2Instant) Close() error {
	return nil
}
