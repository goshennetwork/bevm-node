package protocol

import (
	"fmt"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func IsWitnessStandard(tx *wire.MsgTx, fetcher txscript.PrevOutputFetcher) bool {
	if blockchain.IsCoinBaseTx(tx) {
		return true
	}
	for _, txin := range tx.TxIn {
		if len(txin.Witness) == 0 {
			continue
		}
		prev := fetcher.FetchPrevOutput(txin.PreviousOutPoint)
		prevScript := prev.PkScript
		p2sh := false
		if txscript.IsPayToScriptHash(prevScript) {
			panic("todo")
		}
		version, program, err := txscript.ExtractWitnessProgramInfo(prevScript)
		if err != nil {
			fmt.Println("err 0")
			return false
		}
		if version == 0 && len(program) == WITNESS_V0_SCRIPTHASH_SIZE {
			sizeWitnessStack := len(txin.Witness) - 1
			if len(txin.Witness[sizeWitnessStack]) > MAX_STANDARD_P2WSH_SCRIPT_SIZE {
				fmt.Println("err 1")
				return false
			}
			if sizeWitnessStack > MAX_STANDARD_P2WSH_STACK_ITEMS {
				fmt.Println("err 2")
				return false
			}
			for _, item := range txin.Witness[:sizeWitnessStack] {
				if len(item) > MAX_STANDARD_P2WSH_STACK_ITEM_SIZE {
					fmt.Println("err 3")
					return false
				}
			}
		}

		// Check policy limits for Taproot spends:
		// - MAX_STANDARD_TAPSCRIPT_STACK_ITEM_SIZE limit for stack item size
		// - No annexes
		if version == 1 && len(program) == WITNESS_V1_TAPROOT_SIZE && !p2sh {
			panic("todo")
		}
	}

	return true
}

/** The maximum number of witness stack items in a standard P2WSH script */
const MAX_STANDARD_P2WSH_STACK_ITEMS = 100

/** The maximum size in bytes of each witness stack item in a standard P2WSH script */
const MAX_STANDARD_P2WSH_STACK_ITEM_SIZE = 80

/** The maximum size in bytes of each witness stack item in a standard BIP 342 script (Taproot, leaf version 0xc0) */
const MAX_STANDARD_TAPSCRIPT_STACK_ITEM_SIZE = 80

/** The maximum size in bytes of a standard witnessScript */
const MAX_STANDARD_P2WSH_SCRIPT_SIZE = 3600

/** The maximum size of a standard ScriptSig */
const MAX_STANDARD_SCRIPTSIG_SIZE = 1650

const WITNESS_V0_SCRIPTHASH_SIZE = 32
const WITNESS_V0_KEYHASH_SIZE = 20
const WITNESS_V1_TAPROOT_SIZE = 32
