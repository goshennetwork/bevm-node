package bevm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"testing"
)

func createMiner(t *testing.T) *Miner {
	// Create chainConfig
	memdb := memorydb.New()
	chainDB := rawdb.NewDatabase(memdb)
	genesis := core.DeveloperGenesisBlock(15, 11_500_000, common.HexToAddress("12345"))
	chainConfig, _, err := core.SetupGenesisBlock(chainDB, genesis)
	if err != nil {
		t.Fatalf("can't create new chain config: %v", err)
	}
	// Create consensus engine
	engine := clique.New(chainConfig.Clique, chainDB)
	// Create Ethereum backend
	bc, err := core.NewBlockChain(chainDB, nil, chainConfig, engine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("can't create new chain %v", err)
	}

	backend := NewMockBackend(bc)
	return NewMiner(backend, &BtcRpcConfig{
		Host: "127.0.0.1:12345",
		User: "",
		Pass: "",
	})
}

type mockBackend struct {
	bc *core.BlockChain
}

func NewMockBackend(bc *core.BlockChain) *mockBackend {
	return &mockBackend{
		bc: bc,
	}
}

func (m *mockBackend) BlockChain() *core.BlockChain {
	return m.bc
}

func TestBevmTx(t *testing.T) {
	//	miner := createMiner(t)

}
