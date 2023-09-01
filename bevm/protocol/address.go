package protocol

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/bevm/protocol/utils"
)

// TODO: some code is dulplicated from btcutil, add a NewAddressSigWit method in btcutil to clean this.

// AddressSegWit is the base address type for all SegWit addresses.
type AddressSegWit struct {
	hrp            string
	witnessVersion byte
	witnessProgram []byte
}

// EncodeAddress returns the bech32 (or bech32m for SegWit v1) string encoding
// of an AddressSegWit.
//
// NOTE: This method is part of the Address interface.
func (a *AddressSegWit) EncodeAddress() string {
	str, err := encodeSegWitAddress(
		a.hrp, a.witnessVersion, a.witnessProgram[:],
	)
	if err != nil {
		return ""
	}
	return str
}

// encodeSegWitAddress creates a bech32 (or bech32m for SegWit v1) encoded
// address string representation from witness version and witness program.
func encodeSegWitAddress(hrp string, witnessVersion byte, witnessProgram []byte) (string, error) {
	// Group the address bytes into 5 bit groups, as this is what is used to
	// encode each character in the address string.
	converted, err := bech32.ConvertBits(witnessProgram, 8, 5, true)
	if err != nil {
		return "", err
	}

	// Concatenate the witness version and program, and encode the resulting
	// bytes using bech32 encoding.
	combined := make([]byte, len(converted)+1)
	combined[0] = witnessVersion
	copy(combined[1:], converted)

	var bech string
	switch witnessVersion {
	case 0:
		bech, err = bech32.Encode(hrp, combined)

	case 1:
		bech, err = bech32.EncodeM(hrp, combined)

	default:
		return "", fmt.Errorf("unsupported witness version %d",
			witnessVersion)
	}
	if err != nil {
		return "", err
	}

	// Check validity by decoding the created address.
	version, program, err := decodeSegWitAddress(bech)
	if err != nil {
		return "", fmt.Errorf("invalid segwit address: %v", err)
	}

	if version != witnessVersion || !bytes.Equal(program, witnessProgram) {
		return "", fmt.Errorf("invalid segwit address")
	}

	return bech, nil
}

// decodeSegWitAddress parses a bech32 encoded segwit address string and
// returns the witness version and witness program byte representation.
func decodeSegWitAddress(address string) (byte, []byte, error) {
	// Decode the bech32 encoded address.
	_, data, bech32version, err := bech32.DecodeGeneric(address)
	if err != nil {
		return 0, nil, err
	}

	// The first byte of the decoded address is the witness version, it must
	// exist.
	if len(data) < 1 {
		return 0, nil, fmt.Errorf("no witness version")
	}

	// ...and be <= 16.
	version := data[0]
	if version > 16 {
		return 0, nil, fmt.Errorf("invalid witness version: %v", version)
	}

	// The remaining characters of the address returned are grouped into
	// words of 5 bits. In order to restore the original witness program
	// bytes, we'll need to regroup into 8 bit words.
	regrouped, err := bech32.ConvertBits(data[1:], 5, 8, false)
	if err != nil {
		return 0, nil, err
	}

	// The regrouped data must be between 2 and 40 bytes.
	if len(regrouped) < 2 || len(regrouped) > 40 {
		return 0, nil, fmt.Errorf("invalid data length")
	}

	// For witness version 0, address MUST be exactly 20 or 32 bytes.
	if version == 0 && len(regrouped) != 20 && len(regrouped) != 32 {
		return 0, nil, fmt.Errorf("invalid data length for witness "+
			"version 0: %v", len(regrouped))
	}

	// For witness version 0, the bech32 encoding must be used.
	if version == 0 && bech32version != bech32.Version0 {
		return 0, nil, fmt.Errorf("invalid checksum expected bech32 " +
			"encoding for address with witness version 0")
	}

	// For witness version 1, the bech32m encoding must be used.
	if version == 1 && bech32version != bech32.VersionM {
		return 0, nil, fmt.Errorf("invalid checksum expected bech32m " +
			"encoding for address with witness version 1")
	}

	return version, regrouped, nil
}

// ScriptAddress returns the witness program for this address.
//
// NOTE: This method is part of the Address interface.
func (a *AddressSegWit) ScriptAddress() []byte {
	return a.witnessProgram[:]
}

// IsForNet returns whether the AddressSegWit is associated with the passed
// bitcoin network.
//
// NOTE: This method is part of the Address interface.
func (a *AddressSegWit) IsForNet(net *chaincfg.Params) bool {
	return a.hrp == net.Bech32HRPSegwit+"e"
}

// String returns a human-readable string for the AddressWitnessPubKeyHash.
// This is equivalent to calling EncodeAddress, but is provided so the type
// can be used as a fmt.Stringer.
//
// NOTE: This method is part of the Address interface.
func (a *AddressSegWit) String() string {
	return a.EncodeAddress()
}

// Hrp returns the human-readable part of the bech32 (or bech32m for SegWit v1)
// encoded AddressSegWit.
func (a *AddressSegWit) Hrp() string {
	return a.hrp
}

// WitnessVersion returns the witness version of the AddressSegWit.
func (a *AddressSegWit) WitnessVersion() byte {
	return a.witnessVersion
}

// WitnessProgram returns the witness program of the AddressSegWit.
func (a *AddressSegWit) WitnessProgram() []byte {
	return a.witnessProgram[:]
}

type AddressWitnessEVMScriptHash struct {
	AddressSegWit
}

func NewAddressEVM(witnessProg []byte, net *chaincfg.Params) (*AddressWitnessEVMScriptHash, error) {
	return newAddressWitnessScriptHash(net.Bech32HRPSegwit+"e", witnessProg)
}

func (self *AddressWitnessEVMScriptHash) EvmAddress() common.Address {
	return Ripemd160(self.WitnessProgram())
}

func (self *AddressWitnessEVMScriptHash) Compat() *AddressSegWit {
	addr := self.AddressSegWit
	addr.hrp = strings.TrimSuffix(addr.hrp, "e")

	return &addr
}

func newAddressWitnessScriptHash(hrp string,
	witnessProg []byte) (*AddressWitnessEVMScriptHash, error) {

	// Check for valid program length for witness version 0, which is 32
	// for P2WSH.
	if len(witnessProg) != 32 {
		return nil, errors.New("witness program must be 32 " +
			"bytes for p2wsh")
	}

	addr := &AddressWitnessEVMScriptHash{
		AddressSegWit{
			hrp:            strings.ToLower(hrp),
			witnessVersion: 0x00,
			witnessProgram: witnessProg,
		},
	}

	return addr, nil
}

func mustRegisterEVM(params chaincfg.Params) {
	params.Net += 1 // just make it pass the following register func
	params.Bech32HRPSegwit += "e"
	if err := chaincfg.Register(&params); err != nil {
		panic("failed to register network: " + err.Error())
	}
}

func init() {
	// Register all default networks to make btcutil.DecodeAddress work
	mustRegisterEVM(chaincfg.MainNetParams)
	mustRegisterEVM(chaincfg.TestNet3Params)
	mustRegisterEVM(chaincfg.RegressionNetParams)
	mustRegisterEVM(chaincfg.SimNetParams)
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin network the address is associated with is extracted if possible.
// When the address does not encode the network, such as in the case of a raw
// public key, the address will be associated with the passed defaultNet.
func DecodeAddress(addr string, defaultNet *chaincfg.Params) (btcutil.Address, error) {
	decoded, err := btcutil.DecodeAddress(addr, defaultNet)
	if err != nil {
		return nil, err
	}
	switch val := decoded.(type) {
	case *btcutil.AddressWitnessScriptHash:
		if strings.HasSuffix(val.Hrp(), "e") {
			return &AddressWitnessEVMScriptHash{
				AddressSegWit{
					hrp:            strings.ToLower(val.Hrp()),
					witnessVersion: 0x00,
					witnessProgram: val.WitnessProgram(),
				},
			}, nil
		}
	}
	return decoded, nil
}

func PayToAddrScript(addr btcutil.Address) ([]byte, error) {
	if val := addr.(*AddressWitnessEVMScriptHash); val != nil {
		return txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(val.ScriptAddress()).Script()
	}
	return txscript.PayToAddrScript(addr)
}

var NewAddressWitnessScriptHash = btcutil.NewAddressWitnessScriptHash
var NewAddressPubKey = btcutil.NewAddressPubKey

func NewAddressEVMFromPubKey(pub *btcec.PublicKey, net *chaincfg.Params, deploy ...bool) *AddressWitnessEVMScriptHash {
	script := NewEVMScriptFromPubKey(pub, deploy...)
	hash := sha256.Sum256(script)

	addr, err := NewAddressEVM(hash[:], net)
	utils.Ensure(err)
	return addr
}

func NewEVMScriptFromPubKey(pub *btcec.PublicKey, deploy ...bool) []byte {
	pBytes := pub.SerializeCompressed()
	builder := txscript.NewScriptBuilder()
	num := 10
	if len(deploy) > 0 && deploy[0] {
		num = 49
	}

	for i := 0; i < num; i++ {
		builder.AddOp(txscript.OP_2DROP)
	}
	script, err := builder.AddData(pBytes).AddOp(txscript.OP_CHECKSIG).Script()
	utils.Ensure(err)

	return script
}
