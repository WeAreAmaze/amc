package vm

import (
	"errors"
	"fmt"
)

// List evm execution errors
var (
	ErrOutOfGas                 = errors.New("out of gas")
	ErrCodeStoreOutOfGas        = errors.New("contract creation code storage out of gas")
	ErrDepth                    = errors.New("max call depth exceeded")
	ErrInsufficientBalance      = errors.New("insufficient balance for transfer")
	ErrContractAddressCollision = errors.New("contract address collision")
	ErrExecutionReverted        = errors.New("execution reverted")
	ErrMaxCodeSizeExceeded      = errors.New("max code size exceeded")
	ErrInvalidJump              = errors.New("invalid jump destination")
	ErrWriteProtection          = errors.New("write protection")
	ErrReturnDataOutOfBounds    = errors.New("return data out of bounds")
	ErrGasUintOverflow          = errors.New("gas uint64 overflow")
	ErrInvalidCode              = errors.New("invalid code: must not begin with 0xef")
	ErrNonceUintOverflow        = errors.New("nonce uint64 overflow")

	// errStopToken is an internal token indicating interpreter loop termination,
	// never returned to outside callers.
	errStopToken = errors.New("stop token")
)

// ErrStackUnderflow wraps an evm error when the items on the stack less
// than the minimal requirement.
type ErrStackUnderflow struct {
	stackLen int
	required int
}

func (e *ErrStackUnderflow) Error() string {
	return fmt.Sprintf("stack underflow (%d <=> %d)", e.stackLen, e.required)
}

// ErrStackOverflow wraps an evm error when the items on the stack exceeds
// the maximum allowance.
type ErrStackOverflow struct {
	stackLen int
	limit    int
}

func (e *ErrStackOverflow) Error() string {
	return fmt.Sprintf("stack limit reached %d (%d)", e.stackLen, e.limit)
}

// ErrInvalidOpCode wraps an evm error when an invalid opcode is encountered.
type ErrInvalidOpCode struct {
	opcode OpCode
}

func (e *ErrInvalidOpCode) Error() string { return fmt.Sprintf("invalid opcode: %s", e.opcode) }
