package bevm

import (
	"fmt"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/ethereum/go-ethereum/bevm/protocol"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"math/big"
	"os"
)

type BtcRpcConfig struct {
	Host string
	User string
	Pass string
}

type BlockTranslator struct {
	fetcher txscript.PrevOutputFetcher
	Client  *rpcclient.Client
}

func NewBlockTranslator(conf *BtcRpcConfig) *BlockTranslator {
	log.Info("btc rpc config:", "conf", conf)
	connCfg := &rpcclient.ConnConfig{
		Host:         conf.Host,
		User:         conf.User,
		Pass:         conf.Pass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Error("Failed to create BTC RPC client", "err", err)
		os.Exit(-1)
	}

	return &BlockTranslator{
		fetcher: nil,
		Client:  client,
	}
}

func (self *BlockTranslator) PreparePrevOutPoint(bblock *wire.MsgBlock) (txscript.PrevOutputFetcher, error) {
	fetcher := txscript.NewMultiPrevOutFetcher(nil)
	for _, tx := range bblock.Transactions {
		points := protocol.PreparePrevOutPoints(tx)
		for _, p := range points {
			preTx, err := self.Client.GetRawTransaction(&p.Hash)
			if err != nil {
				return nil, err
			}
			txout := preTx.MsgTx().TxOut[p.Index]
			fetcher.AddPrevOut(p, txout)
		}
	}

	self.fetcher = fetcher

	return fetcher, nil
}
func BtcHashToEvmHash(hash chainhash.Hash) common.Hash {
	const hashSize = 32
	for i := 0; i < hashSize/2; i++ {
		hash[i], hash[hashSize-1-i] = hash[hashSize-1-i], hash[i]
	}

	return common.Hash(hash)
}

func (self *BlockTranslator) ParseBTCBlock(bblock *wire.MsgBlock, height int64, prevHash common.Hash) *types.Block {
	header := &types.Header{
		Difficulty: blockchain.CalcWork(bblock.Header.Bits),
		ParentHash: prevHash,
		UncleHash:  BtcHashToEvmHash(bblock.BlockHash()),
		Number:     big.NewInt(height),
		Time:       uint64(bblock.Header.Timestamp.Unix()),
		GasLimit:   math.MaxUint64,
		TxHash:     common.Hash{},

		MixDigest: common.Hash{},
		Nonce:     types.BlockNonce{},
		BaseFee:   nil,
	}

	var txs []*types.Transaction
	for _, tx := range bblock.Transactions {
		witness := protocol.ExtractEVMWitness(tx, self.fetcher)
		fmt.Println("witness ", witness)
		for i, w := range witness {
			btx := witnessToBevmTx(w, common.Hash(tx.TxHash()), uint64(i))
			txs = append(txs, types.NewTx(btx))
		}
	}

	return types.NewBlock(header, txs, nil, nil, trie.NewStackTrie(nil))
}

func witnessToBevmTx(witness protocol.EVMInvokeData, txHash common.Hash, index uint64) *types.BevmTx {
	tx := types.BevmTx{
		RefHash: txHash,
		Index:   index,
	}
	switch data := witness.(type) {
	case *protocol.EVMCall:
		tx.From = data.From
		to := data.To
		tx.To = &to
		tx.Data = data.Data
	case *protocol.EVMDeploy:
		tx.From = data.From
		tx.To = nil
		tx.Data = data.Data
	}

	return &tx
}
