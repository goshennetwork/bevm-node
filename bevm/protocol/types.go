package protocol

import (
	"bytes"
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/ripemd160"
)

type EVMInvokeData interface {
	ToWitness() [][]byte
	SetFrom(from common.Address)
}

type EVMCall struct {
	From common.Address
	To   common.Address
	Data []byte
}

type EVMDeploy struct {
	From common.Address
	Data []byte
}

func (self *EVMCall) SetFrom(from common.Address) {
	self.From = from
}

func (self *EVMCall) ToWitness() [][]byte {
	encoded := BytesConcat([]byte("evmc"), self.To[:], self.Data[:])
	return EncodeToWitness(encoded)
}

func (self *EVMDeploy) SetFrom(from common.Address) {
	self.From = from
}

func (self *EVMDeploy) ToWitness() [][]byte {
	encoded := BytesConcat([]byte("evmd"), self.Data[:])
	return EncodeToWitness(encoded)
}

func EncodeToWitness(encoded []byte) [][]byte {
	var witness [][]byte
	itemSize := MAX_STANDARD_P2WSH_STACK_ITEM_SIZE
	for len(encoded) > itemSize {
		witness = append(witness, encoded[:itemSize])
		encoded = encoded[itemSize:]
	}
	if len(encoded) > 0 {
		witness = append(witness, encoded)
	}

	return witness
}

func BytesConcat(s ...[]byte) []byte {
	return bytes.Join(s, nil)
}

func DecodeEVMWitness(witness []byte) (EVMInvokeData, error) {
	if bytes.HasPrefix(witness, []byte("evmc")) {
		if len(witness) < 4+20 {
			return nil, io.ErrUnexpectedEOF
		}
		call := &EVMCall{Data: witness[24:]}
		copy(call.To[:], witness[4:])
		return call, nil
	} else if bytes.HasPrefix(witness, []byte("evmd")) {
		return &EVMDeploy{Data: witness[4:]}, nil
	}

	return nil, errors.New("wrong evm prefix")
}

func Ripemd160(data []byte) (result [20]byte) {
	hasher := ripemd160.New()
	hasher.Write(data)
	hasher.Sum(result[:0])
	return
}
