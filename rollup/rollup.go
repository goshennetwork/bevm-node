package rollup

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/laizy/web3/jsonrpc"
	"github.com/ontology-layer-2/rollup-contracts/store"
	"github.com/ontology-layer-2/rollup-contracts/store/schema"
)

type TxsWithContext struct {
	Txs       []*types.Transaction
	Timestamp uint64
}

type EthBackend interface {
	BlockChain() *core.BlockChain
}

type RollupBackend struct {
	ethBackend EthBackend
	Store      *store.Storage
	//l1 client
	L1Client   *jsonrpc.Client
	IsVerifier bool
}

func NewBackend(ethBackend EthBackend, db schema.PersistStore, dbPath string, l1client *jsonrpc.Client, isVerifier bool) *RollupBackend {
	return &RollupBackend{ethBackend, store.NewStorage(db, dbPath), l1client, isVerifier}
}
