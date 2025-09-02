package logger

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

// AccessesTracer tracks accessed state during transaction execution using bitmasks.
// It records operations (read, write, read-write) on nonce, code, balances, and storage slots.
type AccessesTracer struct {
	env *tracing.VMContext

	// metas maps an address to a bitmask of account state operations (nonce, code, balances).
	metas map[common.Address]uint8

	// storage maps an address to a map of storage slots,
	// where each slot maps to a bitmask of storage operations.
	storage map[common.Address]map[common.Hash]uint8
}

// NewAccessesTracer creates a new tracer to populate an access list.
// It can be initialized with an existing `types.Accesses` list, which
// will be used to pre-populate the tracer's internal state.
func NewAccessesTracer(accesses types.Accesses) *AccessesTracer {
	accessTracer := &AccessesTracer{
		metas:   make(map[common.Address]uint8, len(accesses)),
		storage: make(map[common.Address]map[common.Hash]uint8, len(accesses)),
	}

	// Pre-populate from the provided metas list, if any.
	for _, access := range accesses {
		accessTracer.addAccess(access.Address, access.Meta)
		for _, slotOp := range access.Storage {
			accessTracer.addStorageAccess(access.Address, slotOp.Slot, slotOp.Ops)
		}
	}

	return accessTracer
}

// addAccess records an account meta (nonce, code, balances) access operation for the given address.
func (a *AccessesTracer) addAccess(addr common.Address, op uint8) {
	a.metas[addr] |= op
}

// addNonceAccess records an account nonce access operation for the given address.
func (a *AccessesTracer) addNonceAccess(addr common.Address, op uint8) {
	a.metas[addr] |= op << types.AccessShiftNonce
}

// addCodeAccess records an account code access operation for the given address.
func (a *AccessesTracer) addCodeAccess(addr common.Address, op uint8) {
	a.metas[addr] |= op << types.AccessShiftCode
}

// addBalanceAccess records an account balance access operation for the given address.
func (a *AccessesTracer) addBalanceAccess(addr common.Address, op uint8) {
	a.metas[addr] |= op << types.AccessShiftBalance
}

// addStorageAccess records a storage access operation for the given address and slot.
func (a *AccessesTracer) addStorageAccess(addr common.Address, slot common.Hash, op uint8) {
	if _, ok := a.storage[addr]; !ok {
		a.storage[addr] = make(map[common.Hash]uint8)
	}
	a.storage[addr][slot] |= op
}

// setDestructedAccess clear a write nonce and storage access operation for the given address.
func (a *AccessesTracer) setDestructedAccess(addr common.Address) {
	a.addBalanceAccess(addr, types.AccessOpRW)

	if !a.env.StateDB.IsNewContract(addr) {
		return
	}

	delete(a.storage, addr)
	a.metas[addr] &= ^(types.AccessOpWrite << types.AccessShiftNonce)
}

// Accesses converts the internally tracked metas into the `types.Accesses` serializable format.
func (a *AccessesTracer) Accesses() types.Accesses {
	// Create the result with one entry per unique address.
	result := make(types.Accesses, 0, len(a.metas))

	for addr, meta := range a.metas {
		access := types.Access{
			Address: addr,
			Meta:    meta,
			Storage: nil,
		}

		// Add storage slots if any were accessed for this address.
		if slots, ok := a.storage[addr]; ok {
			access.Storage = make([]types.SlotOp, 0, len(slots))
			for slot, ops := range slots {
				access.Storage = append(access.Storage, types.SlotOp{
					Slot: slot,
					Ops:  ops,
				})
			}
		}

		result = append(result, access)
	}

	return result
}

// Hooks returns the set of tracing hooks implemented by AccessesTracer.
// These hooks are called by the EVM during transaction execution.
func (a *AccessesTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		// VM execution lifecycle hooks
		OnTxStart: a.OnTxStart,
		OnEnter:   a.OnEnter,
		OnOpcode:  a.OnOpcode,

		// State change specific hooks
		OnBalanceChange: a.OnBalanceChange,
		OnNonceChange:   a.OnNonceChange,
		OnNonceChangeV2: a.OnNonceChangeV2, // Preferred over OnNonceChange if available
		OnCodeChange:    a.OnCodeChange,
		OnStorageChange: a.OnStorageChange,
	}
}

// OnTxStart tracks state access at the beginning of transaction execution.
// This is the critical entry point for capturing initial state dependencies
// including sender nonce/balance, recipient code/balance, and coinbase impacts.
func (a *AccessesTracer) OnTxStart(vmctx *tracing.VMContext, tx *types.Transaction, from common.Address) {
	a.env = vmctx

	// Sender's nonce is incremented by the transaction (read-write operation)
	// This is critical for tracking nonce-based conflicts between transactions
	a.addNonceAccess(from, types.AccessOpRW)

	// Sender's balance is modified to pay for gas and potential value transfer
	a.addBalanceAccess(from, types.AccessOpRW)

	// Handle recipient-specific state access if this isn't a contract creation

	if value := tx.Value(); tx.To() != nil && value != nil && value.Sign() > 0 {
		// If ETH is being transferred, recipient's balance is written
		a.addBalanceAccess(*tx.To(), types.AccessOpWrite)
	}
	if tx.To() != nil {
		// Even without value, contract code execution requires reading the code
		// to determine if it's a contract with callable functions
		a.addCodeAccess(*tx.To(), types.AccessOpRead)
	}

	// Coinbase (miner/block producer) balance is written to collect fees
	a.addBalanceAccess(vmctx.Coinbase, types.AccessOpWrite)

	// Handle SetCode authorizations (EIP-7702)
	for _, authorization := range tx.SetCodeAuthorizations() {
		authority, err := authorization.Authority()
		if err != nil {
			continue
		}
		// The authority's account nonce is read and potentially written
		a.addNonceAccess(authority, types.AccessOpRW)

		// The authority's code is always modified - either set to delegation indicator
		// or cleared if delegating to the zero address
		a.addCodeAccess(authority, types.AccessOpRW)

		// If delegating to non-zero address, the template contract code is read for validation
		if authorization.Address != (common.Address{}) {
			a.addCodeAccess(authorization.Address, types.AccessOpRead)
		}
	}
}

// OnEnter tracks state access at the beginning of an internal call frame.
// This captures dependencies between contracts when they interact,
// whether through direct calls, delegatecalls, or contract creation.
func (a *AccessesTracer) OnEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	// Handle value transfers for internal calls (if applicable)
	if value != nil && value.Sign() > 0 {
		// Sender's balance is read and modified
		a.addBalanceAccess(from, types.AccessOpRW)

		// Recipient's balance is modified
		a.addBalanceAccess(to, types.AccessOpWrite)
	}

	op := vm.OpCode(typ)
	// Handle different call types
	switch op {
	// Contract creation operations
	case vm.CREATE, vm.CREATE2:
		// Creator's nonce is incremented
		a.addNonceAccess(from, types.AccessOpRW)

		// New contract's code is written
		a.addCodeAccess(to, types.AccessOpRW)

	// Selfdestruct operation
	case vm.SELFDESTRUCT:
		// Mark the self-destructing contract as having its balance modified
		a.addBalanceAccess(from, types.AccessOpRW)

		// Note: The beneficiary's state access is handled elsewhere through
		// the OnBalanceChange hook with BalanceIncreaseSelfdestruct reason

	// Regular calls to existing contracts
	case vm.CALL, vm.STATICCALL, vm.DELEGATECALL, vm.CALLCODE:
		// Target contract's code is read for execution
		// This happens regardless of whether value is transferred
		a.addCodeAccess(to, types.AccessOpRead)

	// Any other operations not explicitly handled
	default:
		// For all call types except contract creation,
		// we need to read the code of the target address
		a.addCodeAccess(to, types.AccessOpRead)
	}
}

// OnOpcode tracks state access during EVM opcode execution, capturing read/write
// patterns for storage slots, balances, code, and nonces. This is the most granular
// level of state access tracking, providing precise information about which specific
// parts of state each transaction depends on or modifies.
func (a *AccessesTracer) OnOpcode(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	// Skip processing if there was an error executing this opcode
	if err != nil {
		return
	}

	op := vm.OpCode(opcode)
	stackData := scope.StackData()
	stackLen := len(stackData)
	caller := scope.Address() // The currently executing contract address

	// Process different opcodes based on how they access state
	switch {

	//
	// Storage access operations
	//

	// SLOAD reads a storage slot from the current contract's storage
	case stackLen >= 1 && op == vm.SLOAD:
		slot := common.Hash(stackData[stackLen-1].Bytes32())
		a.addStorageAccess(caller, slot, types.AccessOpRead)

	// SSTORE writes a value to a storage slot in the current contract's storage
	case stackLen >= 2 && op == vm.SSTORE:
		slot := common.Hash(stackData[stackLen-1].Bytes32())
		a.addStorageAccess(caller, slot, types.AccessOpWrite)

	//
	// Balance access operations
	//

	// BALANCE fetches the balance of a specific address
	case stackLen >= 1 && op == vm.BALANCE:
		addr := common.Address(stackData[stackLen-1].Bytes20())
		a.addBalanceAccess(addr, types.AccessOpRead)

	// SELFBALANCE fetches the balance of the current contract
	case op == vm.SELFBALANCE:
		a.addBalanceAccess(caller, types.AccessOpRead)

	//
	// External code access operations
	//

	// EXTCODESIZE, EXTCODECOPY, EXTCODEHASH all read an address's code
	case stackLen >= 1 && (op == vm.EXTCODECOPY || op == vm.EXTCODEHASH || op == vm.EXTCODESIZE):
		addr := common.Address(stackData[stackLen-1].Bytes20())
		a.addCodeAccess(addr, types.AccessOpRead)

	//
	// Contract call operations
	//

	// CALL, CALLCODE, DELEGATECALL, STATICCALL perform contract calls
	case stackLen >= 2 && (op == vm.DELEGATECALL || op == vm.CALL || op == vm.STATICCALL || op == vm.CALLCODE):
		// Target address is the second item on the stack
		addr := common.Address(stackData[stackLen-2].Bytes20())
		// Code of the target address is read
		a.addCodeAccess(addr, types.AccessOpRead)

		// For CALL and CALLCODE, value transfers may occur
		if (op == vm.CALL || op == vm.CALLCODE) && stackLen >= 3 {
			value := stackData[stackLen-3]
			if value.Sign() > 0 {
				// Caller's balance is read/written during value transfer
				a.addBalanceAccess(caller, types.AccessOpRW)
				// Recipient's balance is written
				a.addBalanceAccess(addr, types.AccessOpWrite)
			}
		}

	//
	// Contract creation operations
	//

	// CREATE operations are handled primarily in OnEnter
	// This case is for any additional state accesses not captured there
	case op == vm.CREATE && stackLen >= 3 || op == vm.CREATE2 && stackLen >= 4:
		a.addNonceAccess(caller, types.AccessOpRW)

		// Value from stack (if non-zero) indicates balance transfer
		value := stackData[stackLen-1]
		if value.Sign() > 0 {
			// Ensure caller's balance is marked as RW
			a.addBalanceAccess(caller, types.AccessOpRW)

			// Note: The new contract address isn't available here,
			// but OnEnter will capture this state access
		}

	//
	// Self-destruct operation
	//

	// SELFDESTRUCT transfers all balance to a beneficiary and destroys the contract
	case stackLen >= 1 && op == vm.SELFDESTRUCT:
		beneficiary := common.Address(stackData[stackLen-1].Bytes20())

		// Beneficiary's balance is written (receives the self-destructed contract's balance)
		a.addBalanceAccess(beneficiary, types.AccessOpWrite)

		// Self-destructing contract's state is changed
		a.setDestructedAccess(caller)
	}
}

func (a *AccessesTracer) OnStorageChange(addr common.Address, slot common.Hash, prev, new common.Hash) {
	a.addStorageAccess(addr, slot, types.AccessOpWrite)
}

func (a *AccessesTracer) OnNonceChange(addr common.Address, prev, new uint64) {
	a.OnNonceChangeV2(addr, prev, new, 0)
}

// OnNonceChangeV2 tracks nonce modifications with specific reason information.
// All nonce changes are tracked as RW operations since they represent
// potential conflicts in transaction order.
func (a *AccessesTracer) OnNonceChangeV2(addr common.Address, prev, new uint64, reason tracing.NonceChangeReason) {
	// All nonce changes are tracked as read-write operations
	// This is critical for transaction ordering conflicts
	a.addNonceAccess(addr, types.AccessOpRW)
}

// OnCodeChange tracks modifications to account code.
// Code changes always affect nonce state since deployment
// is tracked by nonce changes.
func (a *AccessesTracer) OnCodeChange(addr common.Address, prevCodeHash common.Hash, prevCode []byte, codeHash common.Hash, code []byte) {
	a.addCodeAccess(addr, types.AccessOpRW)
}

// OnBalanceChange tracks balance modifications and their relationship to code execution.
// Only certain types of balance changes can trigger contract code execution,
// which is crucial for accurate conflict detection.
func (a *AccessesTracer) OnBalanceChange(addr common.Address, prev, new *big.Int, reason tracing.BalanceChangeReason) {
	// Always mark the balance as written regardless of reason
	a.addBalanceAccess(addr, types.AccessOpWrite)

	// Determine whether this balance change might trigger code execution
	switch reason {

	// Value transfers between accounts can trigger contract code
	case tracing.BalanceChangeTransfer:
		// Only when balance increases (receiving ETH) and when
		// the recipient could be a contract, the code might execute
		if new.Cmp(prev) > 0 {
			a.addCodeAccess(addr, types.AccessOpRead)
		} else {
			a.addBalanceAccess(addr, types.AccessOpRead)
		}

	// System-level balance changes that don't execute code:
	case tracing.BalanceIncreaseRewardMineBlock,
		tracing.BalanceIncreaseRewardMineUncle,
		tracing.BalanceIncreaseRewardTransactionFee,
		tracing.BalanceIncreaseWithdrawal,
		tracing.BalanceIncreaseGenesisBalance,
		tracing.BalanceIncreaseDaoContract,
		tracing.BalanceDecreaseDaoAccount,
		tracing.BalanceChangeTouchAccount,
		tracing.BalanceChangeRevert,
		tracing.BalanceDecreaseGasBuy,
		tracing.BalanceIncreaseGasReturn,
		tracing.BalanceIncreaseSelfdestruct:
		// These balance changes are system-level state transitions
		// that cannot trigger contract code execution
		// No need to mark code as read

	case tracing.BalanceDecreaseSelfdestruct, // Balance deducted from self-destructing contract (sender side)
		tracing.BalanceDecreaseSelfdestructBurn: // ETH sent to an already self-destructed account (effectively burned)
		a.setDestructedAccess(addr)

	// For any unspecified reasons, be conservative and mark as read
	// This ensures we don't miss potential execution paths in future updates
	default:
		a.addCodeAccess(addr, types.AccessOpRead)
		a.addBalanceAccess(addr, types.AccessOpRW)
	}
}
