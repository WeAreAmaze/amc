// Copyright 2023 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"github.com/amazechain/amc/internal/vm/evmtypes"
	"github.com/amazechain/amc/params"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/amazechain/amc/common/types"
)

// CallContext provides a basic interface for the EVM calling conventions. The EVM
// depends on this context being implemented for doing subcalls and initialising new EVM contracts.
type CallContext interface {
	// Call another contract
	Call(env *EVM, me ContractRef, addr types.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// Take another's contract code and execute within our own context
	CallCode(env *EVM, me ContractRef, addr types.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// Same as CallCode except sender and value is propagated from parent to child scope
	DelegateCall(env *EVM, me ContractRef, addr types.Address, data []byte, gas *big.Int) ([]byte, error)
	// Create a new contract
	Create(env *EVM, me ContractRef, data []byte, gas, value *big.Int) ([]byte, types.Address, error)
}

// VMInterface exposes the EVM interface for external callers.
type VMInterface interface {
	Reset(txCtx evmtypes.TxContext, ibs evmtypes.IntraBlockState)
	Create(caller ContractRef, code []byte, gas uint64, value *uint256.Int) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error)
	Call(caller ContractRef, addr types.Address, input []byte, gas uint64, value *uint256.Int, bailout bool) (ret []byte, leftOverGas uint64, err error)
	Cancel()
	Config() Config
	ChainConfig() *params.ChainConfig
	ChainRules() *params.Rules
	Context() evmtypes.BlockContext
	IntraBlockState() evmtypes.IntraBlockState
	TxContext() evmtypes.TxContext
}

// VMInterpreter exposes additional EVM methods for use in the interpreter.
type VMInterpreter interface {
	VMInterface
	Cancelled() bool
	SetCallGasTemp(gas uint64)
	CallGasTemp() uint64
	StaticCall(caller ContractRef, addr types.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	DelegateCall(caller ContractRef, addr types.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	CallCode(caller ContractRef, addr types.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error)
	Create2(caller ContractRef, code []byte, gas uint64, endowment *uint256.Int, salt *uint256.Int) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error)
}
