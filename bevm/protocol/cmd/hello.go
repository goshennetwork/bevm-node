package main

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ethereum/go-ethereum/bevm/protocol"
	"github.com/ethereum/go-ethereum/bevm/protocol/utils"
	"log"
)

func main() {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "172.22.0.1:8332",
		User:         "onchain",
		Pass:         "onchain",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	hash, err := client.GetBlockHash(10)
	fmt.Println(hash, err)
	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block count: %d", blockCount)
	txHash, err := chainhash.NewHashFromStr("100088b91eb50e3ad93179d648c36554b8cd0d95f28d123240dc4bbc83fec635")
	utils.Ensure(err)
	tx, err := client.GetRawTransaction(txHash)
	utils.Ensure(err)

	msgTx := tx.MsgTx()

	txin := msgTx.TxIn[0]
	fmt.Printf("witness: %x", txin.Witness)
	protocol.ExtractEVMWitness(msgTx, nil)
}
