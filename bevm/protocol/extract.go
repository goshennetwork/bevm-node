package protocol

import (
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/ethereum/go-ethereum/log"
)

func PreparePrevOutPoints(tx *wire.MsgTx) (results []wire.OutPoint) {
	for id, txin := range tx.TxIn {
		// currently, we only support p2wsh address
		if len(txin.SignatureScript) != 0 || len(txin.Witness) < 2 {
			continue
		}

		script := txin.Witness[len(txin.Witness)-1]
		drops := numDrop(script)
		if drops == 0 {
			continue
		}
		if drops > len(txin.Witness)-1 {
			log.Warn("drops too large", "tx", tx.TxHash(), "in", id, "drops", drops)
			continue
		}
		var witness []byte
		for i := 0; i < drops; i++ {
			witness = append(witness, txin.Witness[len(txin.Witness)-2-i]...)
		}

		if len(witness) == 0 {
			continue
		}
		_, err := DecodeEVMWitness(witness)
		if err != nil {
			log.Info("decode evm witness error: ", "tx", tx.TxHash(), "in", id, "err", err)
			continue
		}
		results = append(results, txin.PreviousOutPoint)
	}

	return results
}

func ExtractEVMWitness(tx *wire.MsgTx, fetcher txscript.PrevOutputFetcher) (result []EVMInvokeData) {
	for id, txin := range tx.TxIn {
		// currently, we only support p2wsh address
		if len(txin.SignatureScript) != 0 || len(txin.Witness) < 2 {
			continue
		}

		script := txin.Witness[len(txin.Witness)-1]
		drops := numDrop(script)
		if drops == 0 {
			continue
		}
		if drops > len(txin.Witness)-1 {
			log.Warn("drops too large", "tx", tx.TxHash(), "in", id, "drops", drops)
			continue
		}
		var witness []byte
		for i := 0; i < drops; i++ {
			witness = append(witness, txin.Witness[len(txin.Witness)-2-i]...)
		}

		if len(witness) == 0 {
			continue
		}
		data, err := DecodeEVMWitness(witness)
		if err != nil {
			log.Info("decode evm witness error", "tx", tx.TxHash(), "in", id, "err", err)
			continue
		}

		// delay the prev output query to here reduce the fetcher size
		prevOut := fetcher.FetchPrevOutput(txin.PreviousOutPoint)
		pkscript, err := txscript.ParsePkScript(prevOut.PkScript)
		if err != nil || pkscript.Class() != txscript.WitnessV0ScriptHashTy {
			continue
		}
		data.SetFrom(Ripemd160(pkscript.Script()[2:34]))
		result = append(result, data)
	}

	return result
}
