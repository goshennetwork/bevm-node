// Copyright 2023 The Goshen network Authors

package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const MaxGas = 10000000

// BevmTx is the transaction data of regular Ethereum transactions.
type BevmTx struct {
	From    common.Address
	To      *common.Address `rlp:"nil"` // nil means contract creation
	Data    []byte          // contract invocation input data
	RefHash [32]byte        // btc transaction hash
	Index   uint64          // index in the btc transaction
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *BevmTx) copy() TxData {
	cpy := &BevmTx{
		From:    tx.From,
		To:      copyAddressPtr(tx.To),
		Data:    common.CopyBytes(tx.Data),
		RefHash: tx.RefHash,
		Index:   tx.Index,
	}
	return cpy
}

// accessors for innerTx.
func (tx *BevmTx) txType() byte           { return LegacyTxType }
func (tx *BevmTx) chainID() *big.Int      { return big.NewInt(0) }
func (tx *BevmTx) accessList() AccessList { return nil }
func (tx *BevmTx) data() []byte           { return tx.Data }
func (tx *BevmTx) gas() uint64            { return MaxGas }
func (tx *BevmTx) gasPrice() *big.Int     { return big.NewInt(0) }
func (tx *BevmTx) gasTipCap() *big.Int    { return big.NewInt(0) }
func (tx *BevmTx) gasFeeCap() *big.Int    { return big.NewInt(0) }
func (tx *BevmTx) value() *big.Int        { return big.NewInt(0) }
func (tx *BevmTx) nonce() uint64          { return 0 }
func (tx *BevmTx) to() *common.Address    { return tx.To }

func (tx *BevmTx) rawSignatureValues() (v, r, s *big.Int) {
	return big.NewInt(0), big.NewInt(0), big.NewInt(0)
}

func (tx *BevmTx) setSignatureValues(chainID, v, r, s *big.Int) {
}
