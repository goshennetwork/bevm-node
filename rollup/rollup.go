package rollup

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

type TxsWithContext struct {
	Txs       []*types.Transaction
	Timestamp uint64
}

type EthBackend interface {
	BlockChain() *core.BlockChain
}

type RollupBackend struct {
	EthBackend EthBackend
	IsVerifier bool
}

func NewBackend(ethBackend EthBackend, isVerifier bool) *RollupBackend {
	return &RollupBackend{ethBackend, isVerifier}
}
