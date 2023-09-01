package main

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/bevm/protocol"
	"github.com/ethereum/go-ethereum/bevm/protocol/utils"
)

func main() {
	netParams := &chaincfg.TestNet3Params
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		log.Fatal(err)
	}
	privateKeyHex := hex.EncodeToString(privateKey.Serialize())
	log.Printf("new priviate key %s \n", privateKeyHex)

	taprootAddress, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(txscript.ComputeTaprootKeyNoScript(privateKey.PubKey())), netParams)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("new taproot address %s \n", taprootAddress.EncodeAddress())
	evmAddress := protocol.NewAddressEVMFromPubKey(privateKey.PubKey(), netParams)
	log.Printf("new bevm address %s \n", evmAddress.EncodeAddress())
	script := protocol.NewEVMScriptFromPubKey(privateKey.PubKey())
	hash := sha256.Sum256(script)
	wsh, err := btcutil.NewAddressWitnessScriptHash(hash[:], netParams)
	utils.Ensure(err)
	log.Printf("new wsh address %s \n", wsh.EncodeAddress())

	/*
			2023/06/01 16:35:24 new priviate key 2129ba4799e5d010cf8ae6c79168f1e3746bbd578fb8d2e4725ec2ba969d47a1
			2023/06/01 16:35:24 new taproot address tb1p0fg3qyzr5umlywmaqz5wzgmfkltdwdaj94x0y03alm3qf4xhf5qqcg3y5f
			2023/06/01 16:35:24 new bevm address tbe1q73kpl9nj2kxa9ts7zar0hdn7h2nw7syn69auhkwpvh9dq8hhz5vs07ldj4
			2023/06/01 16:35:24 new wsh address tb1q73kpl9nj2kxa9ts7zar0hdn7h2nw7syn69auhkwpvh9dq8hhz5vsl4dp6x

		https://testnet-faucet.com/btc-testnet/
	*/
}
