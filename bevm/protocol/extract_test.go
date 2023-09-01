package protocol

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/ethereum/go-ethereum/bevm/protocol/utils"
)

func TestExtractEVMWitness(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)

	prevHash, _ := chainhash.NewHashFromStr("cf5eaad6c16fd78166fd3bea28727de0025b45a78953e2d9780e13225a910859")
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(prevHash, 0),
	})
	payScript, err := PayToAddrScript(evmAddress)
	utils.Ensure(err)
	tx.AddTxOut(&wire.TxOut{
		Value:    8000,
		PkScript: payScript,
	})

	fetcher := blockchain.NewUtxoViewpoint()
	fetcher.Entries()[tx.TxIn[0].PreviousOutPoint] = blockchain.NewUtxoEntry(&wire.TxOut{Value: 9208, PkScript: payScript}, 0, false)
	sigHashes := txscript.NewTxSigHashes(tx, fetcher)
	witness, err := EVMWitnessSign(tx, sigHashes, 0, 9208, privateKey, nil)
	utils.Ensure(err)
	tx.TxIn[0].Witness = witness

	txHex := getTxHex(tx)
	serializedTx, err := hex.DecodeString(txHex)
	var mtx wire.MsgTx
	err = mtx.Deserialize(bytes.NewReader(serializedTx))

	fmt.Println("tx raw: ", txHex)
	fmt.Println("tx json: ", utils.JsonString(mtx))
	err = blockchain.ValidateTransactionScripts(btcutil.NewTx(tx), fetcher,
		txscript.StandardVerifyFlags, txscript.NewSigCache(10),
		txscript.NewHashCache(100))
	utils.Ensure(err)
}

func TestHello(t *testing.T) {

}
