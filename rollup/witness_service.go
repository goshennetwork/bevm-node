package rollup

import (
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/layer2"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/laizy/log"
	"github.com/laizy/web3/utils"
	"github.com/ontology-layer-2/rollup-contracts/binding"
	"github.com/ontology-layer-2/rollup-contracts/store/schema"
)

type WitnessService struct {
	*RollupBackend
	quit chan struct{}
}

func NewWitnessService(backend *RollupBackend) *WitnessService {
	return &WitnessService{backend, make(chan struct{})}
}

func (self *WitnessService) Start() error {
	go self.run()
	return nil
}

func (self *WitnessService) run() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			if err := self.Work(); err != nil {
				log.Error("check batch failed", "err", err)
			}
		case <-self.quit:
			return
		}

	}
}

func (self *WitnessService) Save(blocks []*BlockWithReceipts) {
	for _, blockWithReceipts := range blocks {
		block := blockWithReceipts.b
		var logs []*types.Log
		for i, receipt := range blockWithReceipts.r {
			// add block location fields
			receipt.BlockHash = block.Hash()
			receipt.BlockNumber = block.Number()
			receipt.TransactionIndex = uint(i)

			// Update the block hash in all logs since it is now available and not when the
			// receipt/log of individual transactions were created.
			for _, l := range receipt.Logs {
				l.BlockHash = block.Hash()
			}
			logs = append(logs, receipt.Logs...)
		}
		// Commit block and state to database.
		_, err := self.ethBackend.BlockChain().WriteBlockWithState(block, blockWithReceipts.r, logs, blockWithReceipts.s, true)
		if err != nil { // wired
			panic(err)
		}

	}
}

func (self *WitnessService) Work() error {
	inputChainStore := self.Store.InputChain()
	l2Batches := inputChainStore.GetInfo().TotalBatches

	checkedBatchNum := self.Store.L2Client().GetTotalCheckedBatchNum()
	if checkedBatchNum >= l2Batches {
		log.Debug("have checked all l2 batches", "checkedBatchNum", checkedBatchNum, "l2 total", l2Batches)
		return nil
	}
	checkedBlockNum := self.Store.L2Client().GetTotalCheckedBlockNum(checkedBatchNum)
	//now only check one batch
	input, err := inputChainStore.GetAppendedTransaction(checkedBatchNum)
	utils.Ensure(err)

	data, err := inputChainStore.GetSequencerBatchData(input.Index)
	if err != nil {
		//not found, should never happen
		panic(1)
	}

	parentBlock := self.ethBackend.BlockChain().GetBlockByNumber(checkedBlockNum - 1)
	blocks, inputHash := ProcessBatch(data, parentBlock.Hash(), self.RollupBackend)
	utils.EnsureTrue(inputHash == input.InputHash)
	self.Save(blocks)
	checkedBlockNum += uint64(len(blocks))
	checkedBatchNum += 1

	//now store witnessed state
	writer := self.Store.Writer()
	writer.L2Client().StoreTotalCheckedBatchNum(checkedBatchNum)
	writer.L2Client().StoreCheckedBlockNum(checkedBatchNum, checkedBlockNum)
	writer.Commit()
	return nil
}

func ProcessBatch(input []byte, parentBlockHash common.Hash, rollupBackend *RollupBackend) ([]*BlockWithReceipts, [32]byte) {
	eth := rollupBackend.ethBackend
	parent := eth.BlockChain().GetBlockByHash(parentBlockHash)
	inputChainStore := rollupBackend.Store.InputChain()
	batch := &binding.RollupInputBatches{}
	if err := batch.Decode(input); err != nil { //may decode failed, just ignore
		log.Warn("malicious batch code found", "err", err)
	}
	queues, err := inputChainStore.GetEnqueuedTransactions(batch.QueueStart, batch.QueueNum)
	if err != nil {
		panic(err)
	}

	var orderTxs []*binding.SubBatch
	txs := &binding.SubBatch{}
	last := len(queues) - 1
	for i, enqueue := range queues {
		tx, timestamp := enqueue.MustToTransaction(), enqueue.Timestamp
		if i == 0 {
			txs.Timestamp = timestamp
			txs.Txs = append(txs.Txs, tx)
		} else {
			if timestamp != txs.Timestamp {
				//append first
				orderTxs = append(orderTxs, txs)
				txs = &binding.SubBatch{timestamp, nil}
			}
			txs.Txs = append(txs.Txs, tx)
		}
		if i == last {
			orderTxs = append(orderTxs, txs)
		}

	}
	orderTxs = append(orderTxs, batch.SubBatches...)

	//sort in timestamp
	sort.SliceStable(orderTxs, func(i, j int) bool {
		return orderTxs[i].Timestamp < orderTxs[j].Timestamp
	})
	return RunOrderesTxs(eth.BlockChain(), orderTxs, parent.Header()), batch.InputHash(schema.CalcQueueHash(queues))

}

type blockTask struct {
	header   *types.Header
	statedb  *state.StateDB
	gasPool  *core.GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt
}

func (b *blockTask) sealToBlock(chain *core.BlockChain) *BlockWithReceipts {
	engine := chain.Engine()
	// no uncles
	block, err := engine.FinalizeAndAssemble(chain, b.header, b.statedb, b.txs, nil, b.receipts)
	if err != nil {
		//should never happen?
		panic(err)
	}
	return &BlockWithReceipts{block, b.statedb, b.receipts}
}

// state is parent state,just use it
func makeBlockTask(parent *types.Header, queueNum uint64, timestamp uint64, stateDb *state.StateDB, engine consensus.Engine) *blockTask {
	num := parent.Number.Uint64()
	gaslimit := parent.GasLimit
	h := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).SetUint64(num + 1),
		GasLimit:   gaslimit,
		Time:       timestamp,
		Coinbase:   engine.(*layer2.Layer2Instant).FeeCollector,
	}
	return &blockTask{
		header:  h,
		statedb: stateDb.Copy(),
		gasPool: new(core.GasPool).AddGas(gaslimit),
	}
}

type BlockWithReceipts struct {
	b *types.Block
	s *state.StateDB
	r []*types.Receipt
}

func RunOrderesTxs(chain *core.BlockChain, orderTxs []*binding.SubBatch, parent *types.Header) []*BlockWithReceipts {
	var ret []*BlockWithReceipts
	statedb, err := chain.StateAt(parent.Root)
	if err != nil {
		panic(err)
	}
	cfg := core.ValidDataTxConfig{
		MaxGas:   parent.GasLimit,
		GasPrice: big.NewInt(0),
		BaseFee:  nil,
		Istanbul: true,
		Eip2718:  true,
		Eip1559:  false,
	}

	gasLimit := parent.GasLimit
	parentStateDb := statedb
	for i := 0; i < len(orderTxs); i++ {
		timestamp := orderTxs[i].Timestamp
		isQueue := false
		if len(orderTxs[i].Txs) > 0 {
			isQueue = orderTxs[i].Txs[0].IsQueue()
		}
		usedGas := uint64(0)
		var txs []*types.Transaction
		var queues []*types.Transaction
		for _, tx := range orderTxs[i].Txs {
			switch isQueue {
			case true: //queue tx
				//no matter what tx consumed,just assume it consumes all gas
				usedGas += tx.Gas()
				if usedGas >= gasLimit { // a new block tak
					blockTask := makeBlockTask(parent, uint64(len(queues)), timestamp, parentStateDb, chain.Engine())
					CommitTransactions(chain, txs, blockTask)
					// save blocks which have executed queues no matter what happened
					ret = append(ret, blockTask.sealToBlock(chain))
					parentB := ret[len(ret)-1]
					parent = parentB.b.Header()
					parentStateDb = blockTask.statedb
					if tx.Gas() >= gasLimit { //should never happen
						panic(1)
					}
					usedGas = tx.Gas()
					queues = queues[:0]
				}
				queues = append(queues, tx)
			case false: //l2 origin tx
				//check l2 origin tx first
				if err := core.ValidateTx(tx, parentStateDb, types.LatestSigner(chain.Config()), cfg, true); err != nil {
					log.Error("validate tx failed", "hash", tx.Hash())
					continue
				}
				txs = append(txs, tx)
			}
		}
		blockTask := makeBlockTask(parent, uint64(len(queues)), timestamp, parentStateDb, chain.Engine())
		txs = append(queues, txs...)
		CommitTransactions(chain, txs, blockTask)
		//every order txs try to seal to a block
		ret = append(ret, blockTask.sealToBlock(chain))
		parentB := ret[len(ret)-1]
		parent = parentB.b.Header()
		parentStateDb = blockTask.statedb
	}
	return ret
}

func (b *blockTask) TotalNonQueueTxSize() uint64 {
	s := uint64(0)
	for _, tx := range b.txs {
		if !tx.IsQueue() {
			s += uint64(tx.Size())
		}
	}
	return s
}

func CommitTransactions(chain *core.BlockChain, txs []*types.Transaction, task *blockTask) {
	txIndex := 0
	for _, tx := range txs {
		statedb := task.statedb
		header := task.header
		gasPool := task.gasPool
		statedb.Prepare(tx.Hash(), txIndex)
		snap := statedb.Snapshot()
		chainConfig := chain.Config()
		receipt, err := core.ApplyTransaction(chainConfig, chain, &header.Coinbase, gasPool, statedb, header, tx, &header.GasUsed, *chain.GetVMConfig())
		if err != nil {
			statedb.RevertToSnapshot(snap)
			log.Warn("malicious tx", "err", err, "txHash", tx.Hash())
			continue
		}
		task.txs = append(task.txs, tx)
		task.receipts = append(task.receipts, receipt)
		txIndex += 1
	}
}

func (self *WitnessService) Stop() error {
	self.quit <- struct{}{}
	return nil
}
