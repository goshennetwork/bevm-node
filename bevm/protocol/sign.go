package protocol

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func EVMWitnessSign(tx *wire.MsgTx, sigHashes *txscript.TxSigHashes, idx int, amt int64,
	privKey *btcec.PrivateKey, evmData EVMInvokeData) (wire.TxWitness, error) {
	deploy := false
	if _, ok := evmData.(*EVMDeploy); ok {
		deploy = true
	}
	subscript := NewEVMScriptFromPubKey(privKey.PubKey(), deploy)

	return EVMWitnessSignature(tx, sigHashes, idx, amt, subscript, txscript.SigHashAll, privKey, evmData)
}

func EVMWitnessSignature(tx *wire.MsgTx, sigHashes *txscript.TxSigHashes, idx int, amt int64,
	subscript []byte, hashType txscript.SigHashType, privKey *btcec.PrivateKey, evmData EVMInvokeData) (wire.TxWitness, error) {
	var witness [][]byte
	sig, err := txscript.RawTxInWitnessSignature(tx, sigHashes, idx, amt, subscript, hashType, privKey)
	if err != nil {
		return nil, err
	}
	witness = append(witness, sig)
	drops := numDrop(subscript)
	var evmWit [][]byte
	if evmData != nil {
		evmWit = evmData.ToWitness()
	}
	if len(evmWit) > drops {
		return nil, fmt.Errorf("evm witness data too large, drops: %d, actrual: %d", drops, len(evmWit))
	}
	for i := len(evmWit); i < drops; i++ {
		witness = append(witness, nil)
	}
	for i := 0; i < len(evmWit); i++ {
		witness = append(witness, evmWit[len(evmWit)-i-1])
	}
	witness = append(witness, subscript)

	return witness, nil
}

func numDrop(script []byte) int {
	drops := 0
	for _, b := range script {
		switch b {
		case txscript.OP_DROP:
			drops += 1
		case txscript.OP_2DROP:
			drops += 2
		default:
			return drops
		}
	}

	return drops
}
