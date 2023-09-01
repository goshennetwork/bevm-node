package protocol

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/bevm/protocol/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewAddressWitnessEVMScriptHash(t *testing.T) {
	hash := make([]byte, 32)
	rand.Read(hash)
	addr, err := NewAddressEVM(hash[:], &chaincfg.MainNetParams)
	assert.Nil(t, err)
	addr2, err := DecodeAddress(addr.EncodeAddress(), nil)
	assert.Nil(t, err)

	assert.Equal(t, addr.EncodeAddress(), addr2.EncodeAddress())
}

func TestNewEVMScriptFromPubKey(t *testing.T) {
	key, err := btcec.NewPrivateKey()
	assert.Nil(t, err)
	script := NewEVMScriptFromPubKey(key.PubKey())
	fmt.Printf("%x\n", script)

	addr, err := DecodeAddress("tbe1q73kpl9nj2kxa9ts7zar0hdn7h2nw7syn69auhkwpvh9dq8hhz5vs07ldj4", nil)
	utils.Ensure(err)
	val, err := PayToAddrScript(addr)
	utils.Ensure(err)

	fmt.Printf("pay script: %x\n", val)
	fmt.Println(txscript.DisasmString(val))
}
