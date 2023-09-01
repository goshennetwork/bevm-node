package bevm

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"sync/atomic"
	"time"
)

type Backend interface {
	BlockChain() *core.BlockChain
}

type Miner struct {
	eth    Backend
	bt     *BlockTranslator
	closed int32
}

func NewMiner(eth Backend, config *BtcRpcConfig) *Miner {
	return &Miner{
		eth:    eth,
		bt:     NewBlockTranslator(config),
		closed: 0,
	}
}

func (self *Miner) ExecuteBlock(block *types.Block) (*types.Block, types.Receipts, []*types.Log, uint64, error) {
	prevHeader := self.eth.BlockChain().GetHeaderByNumber(block.NumberU64() - 1)
	statedb, err := self.eth.BlockChain().StateAt(prevHeader.Root)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	receipts, logs, usedGas, err := self.eth.BlockChain().Processor().Process(block, statedb, *self.eth.BlockChain().GetVMConfig())
	header := block.Header()
	header.GasUsed = usedGas
	header.Bloom = types.CreateBloom(receipts)
	header.ReceiptHash = types.DeriveSha(receipts, trie.NewStackTrie(nil))
	header.Root = statedb.IntermediateRoot(true)
	block = block.WithSeal(header)

	return block, receipts, logs, usedGas, nil
}

func (self *Miner) SubmitBlock(block *types.Block) (*types.Block, error) {
	sealed, _, _, _, err := self.ExecuteBlock(block)
	if err != nil {
		return nil, err
	}
	_, err = self.eth.BlockChain().InsertChain([]*types.Block{sealed})
	if err != nil {
		return nil, err
	}
	return sealed, nil
}

func (self *Miner) Start() error {
	for atomic.LoadInt32(&self.closed) == 0 {
		log.Info("try mint new block")
		err := self.loop()
		if err != nil {
			log.Info("Mint block error", "err", err)
		}

		time.Sleep(time.Second * 10)
	}

	return nil
}

func (self *Miner) Stop() error {
	atomic.StoreInt32(&self.closed, 1)
	return nil
}

func (self *Miner) loop() error {
	client := self.bt.Client
	bheight, err := client.GetBlockCount()
	if err != nil {
		return fmt.Errorf("get btc block height error: %v", err)
	}
	header := self.eth.BlockChain().CurrentHeader()
	currHeight := header.Number.Int64()
	log.Info("sync info:", "curr ledger height", currHeight, "btc height", bheight)
	if currHeight+1 > bheight {
		return nil
	}

	for currHeight+1 <= bheight {
		log.Info("sync info:", "curr ledger height", currHeight, "btc height", bheight)
		hash, err := client.GetBlockHash(currHeight + 1)
		if err != nil {
			return fmt.Errorf("get btc block hash error: %v", err)
		}
		log.Info("btc block hash", "hash", hash)
		block, err := client.GetBlock(hash)
		if err != nil {
			return fmt.Errorf("get btc block error: %v", err)
		}
		prevHash := BtcHashToEvmHash(block.Header.PrevBlock)
		if header.UncleHash != prevHash {
			return fmt.Errorf("block reorged, btc hash: %s, uncle hash: %s", prevHash, header.UncleHash)
		}
		log.Info("get btc block success", "hash", hash)
		_, err = self.bt.PreparePrevOutPoint(block)
		if err != nil {
			return fmt.Errorf("get btc block prev output point error: %v", err)
		}
		log.Info("prepare btc block prev outpoint success")

		eblock := self.bt.ParseBTCBlock(block, currHeight+1, header.Hash())
		log.Info("parse btc block success")
		eblock, err = self.SubmitBlock(eblock)
		if err != nil {
			return fmt.Errorf("submit block error: %v", err)
		}
		log.Info("submit evm block success", "height", eblock.Header().Number.Int64())
		currHeight += 1
		header = eblock.Header()
	}

	return nil
}
