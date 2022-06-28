package rollup

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/consts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ontology-layer-2/rollup-contracts/binding"
	"github.com/ontology-layer-2/rollup-contracts/store/schema"
)

type L2Api struct {
	*RollupBackend
}

func Apis(backend *RollupBackend) []rpc.API {
	return []rpc.API{
		{
			Namespace: "l2",
			Version:   "1.0",
			Service:   newUploadApi(backend),
			Public:    true,
		},
	}
}

func newUploadApi(rollupBackend *RollupBackend) *L2Api {
	return &L2Api{rollupBackend}
}

type GlobalInfo struct {
	//total batch num in l1 RollupInputChain contract
	L1InputInfo schema.InputChainInfo
	//l2 client have checked tx batch num
	L2CheckedBatchNum uint64
	//the total block num l2 already checked,start from 1, because genesis block do not need to check
	L2CheckedBlockNum uint64
	//l2 client head block num
	L2HeadBlockNumber   uint64
	L1SyncedBlockNumber uint64
	L1SyncedTimestamp   *uint64
}

func (self *L2Api) GlobalInfo() *GlobalInfo {
	//maybe syncing should also return some info
	var ret GlobalInfo
	l2Store := self.Store.L2Client()
	info := self.Store.InputChain().GetInfo()
	ret.L1InputInfo = *info
	ret.L2CheckedBatchNum = l2Store.GetTotalCheckedBatchNum()
	ret.L2CheckedBlockNum = l2Store.GetTotalCheckedBlockNum(info.TotalBatches)
	ret.L2HeadBlockNumber = self.ethBackend.BlockChain().CurrentHeader().Number.Uint64()
	ret.L1SyncedBlockNumber = self.Store.GetLastSyncedL1Height()
	ret.L1SyncedTimestamp = self.Store.GetLastSyncedL1Timestamp()
	return &ret
}

//fixme: now only support one sequencer.
// GetPendingTxBatches return the batchCode, which is params in AppendBatch, set func selector in front of it to invoke
//append input batch.
func (self *L2Api) GetPendingTxBatches() []byte {
	if !self.IsSynced() {
		log.Warn("syncing")
		return nil
	}
	info := self.GlobalInfo()
	if info.L2CheckedBatchNum < info.L1InputInfo.TotalBatches { //should check all the batch first
		log.Warn("nothing to append")
		return nil
	}
	l2CheckedBlockNum, l2HeadBlockNumber := info.L2CheckedBlockNum, info.L2HeadBlockNumber
	if l2CheckedBlockNum > l2HeadBlockNumber {
		//local have nothing to upload
		log.Warn("nothing need to upload ", "total checked block", l2CheckedBlockNum, "local block number", l2HeadBlockNumber)
		return nil
	}
	maxBlockes := l2HeadBlockNumber - l2CheckedBlockNum + 1
	//todo: now simple limit upload size.should limit calldata size instead
	if maxBlockes > 512 {
		maxBlockes = 512
	}
	batches := &binding.RollupInputBatches{QueueStart: info.L1InputInfo.PendingQueueIndex, BatchIndex: info.L2CheckedBatchNum}
	var batchesData []byte
	for i := uint64(0); i < maxBlockes; i++ {
		blockNumber := i + l2CheckedBlockNum
		block := self.ethBackend.BlockChain().GetBlockByNumber(blockNumber)
		if block == nil { // should not happen except chain reorg
			log.Warn("nil block", "blockNumber", blockNumber)
			return nil
		}
		txs := block.Transactions()
		l2txs, queueNum := FilterOutQueues(txs)
		batches.QueueNum += queueNum
		if len(l2txs) > 0 {
			batches.SubBatches = append(batches.SubBatches, &binding.SubBatch{Timestamp: block.Time(), Txs: l2txs})
		}
		newBatch := batches.Encode()
		if len(newBatch)+4 < consts.MaxRollupInputBatchSize {
			batchesData = newBatch
		}
	}
	log.Info("generate batch", "index", batches.BatchIndex, "size", len(batchesData))
	return batchesData
}

func (self *L2Api) GetState(batchIndex uint64) common.Hash {
	if batchIndex >= self.Store.L2Client().GetTotalCheckedBatchNum() {
		return common.Hash{}
	}
	blockNum := self.Store.L2Client().GetTotalCheckedBlockNum(batchIndex)
	index := blockNum - 1
	block := self.ethBackend.BlockChain().GetBlockByNumber(index)
	if block == nil {
		log.Warn("nil block", "blockNumber", index)
		return common.Hash{}
	}
	return block.Hash()
}

func FilterOutQueues(txs []*types.Transaction) ([]*types.Transaction, uint64) {
	ret := make([]*types.Transaction, 0, len(txs))
	queueNum := uint64(0)
	for _, tx := range txs {
		if tx.IsQueue() {
			queueNum++
		} else {
			ret = append(ret, tx)
		}
	}
	return ret, queueNum
}
