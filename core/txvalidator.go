package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/consts"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type ValidDataTxConfig struct {
	MaxGas   uint64
	GasPrice *big.Int
	BaseFee  *big.Int
	Istanbul bool // Fork indicator whether we are in the istanbul stage.
	Eip2718  bool // Fork indicator whether we are using EIP-2718 type transactions.
	Eip1559  bool // Fork indicator whether we are using EIP-1559 type transactions.
}

func ValidateTx(tx *types.Transaction, statedb *state.StateDB, signer types.Signer, cfg ValidDataTxConfig, l2Mod bool) error {
	if l2Mod { //l2 tx pool do not accept L1CrossLayer signature message,because of hard coded
		//in l1, here use the same is fine, maybe set it in chainConfig?
		if tx.Protected() == false {
			return ErrTxUnprotected
		}
		sender, err := signer.Sender(tx)
		if err != nil {
			return fmt.Errorf("validate l2 tx err: %s", err)
		}
		if sender == consts.L1CrossLayerWitnessSender {
			return ErrUnexpectedSystemSender
		}
		if tx.Nonce() >= consts.MaxSenderNonce {
			return ErrNonceMax
		}
	}
	// Accept only legacy transactions until EIP-2718/2930 activates.
	if !cfg.Eip2718 && tx.Type() != types.LegacyTxType {
		return ErrTxTypeNotSupported
	}
	// Reject dynamic fee transactions until EIP-1559 activates.
	if !cfg.Eip1559 && tx.Type() == types.DynamicFeeTxType {
		return ErrTxTypeNotSupported
	}
	// Reject transactions over defined size to prevent DOS attacks
	if uint64(tx.Size()) > txMaxSize {
		return ErrOversizedData
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	// Ensure the transaction doesn't exceed the current block limit gas.
	if cfg.MaxGas < tx.Gas() {
		return ErrGasLimit
	}
	// Sanity check for extremely large numbers
	if tx.GasFeeCap().BitLen() > 256 {
		return ErrFeeCapVeryHigh
	}
	if tx.GasTipCap().BitLen() > 256 {
		return ErrTipVeryHigh
	}
	// Ensure gasFeeCap is greater than or equal to gasTipCap.
	if tx.GasFeeCapIntCmp(tx.GasTipCap()) < 0 {
		return ErrTipAboveFeeCap
	}
	// Make sure the transaction is signed properly.
	from, err := types.Sender(signer, tx)
	if err != nil {
		return ErrInvalidSender
	}
	// Drop non-local transactions under our own minimal accepted gas price or tip.
	if tx.EffectiveGasTipIntCmp(cfg.GasPrice, cfg.BaseFee) < 0 {
		return ErrUnderpriced
	}
	// Ensure the transaction adheres to nonce ordering
	if statedb.GetNonce(from) > tx.Nonce() {
		return ErrNonceTooLow
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	if statedb.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}
	// Ensure the transaction has more gas than the basic tx fee.
	intrGas, err := IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil, true, cfg.Istanbul)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}
	return nil
}
