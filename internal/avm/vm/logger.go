// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/amazechain/amc/internal/avm/common"
)

// EVMLogger is used to collect execution traces from an EVM transaction
// execution. CaptureState is called for each step of the VM with the
// current VM state.
// Note that reference types are actual VM data structures; make copies
// if you need to retain them beyond the current call.
type EVMLogger interface {
	// Transaction level
	CaptureTxStart(gasLimit uint64)
	CaptureTxEnd(restGas uint64)
	// Top call frame
	CaptureStart(env *EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int)
	CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error)
	// Rest of call frames
	CaptureEnter(typ OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int)
	CaptureExit(output []byte, gasUsed uint64, err error)
	// Opcode level
	CaptureState(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, rData []byte, depth int, err error)
	CaptureFault(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, depth int, err error)
}

type mdLogger struct {
	out io.Writer
	env *EVM
}

// NewMarkdownLogger creates a logger which outputs information in a format adapted
// for human readability, and is also a valid markdown table
func NewMarkdownLogger(writer io.Writer) *mdLogger {
	l := &mdLogger{out: writer}
	return l
}

func (t *mdLogger) CaptureStart(env *EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	if !create {
		fmt.Fprintf(t.out, "From: `%v`\nTo: `%v`\nData: `%#x`\nGas: `%d`\nValue `%v` wei\n",
			from.String(), to.String(),
			input, gas, value)
	} else {
		fmt.Fprintf(t.out, "From: `%v`\nCreate at: `%v`\nData: `%#x`\nGas: `%d`\nValue `%v` wei\n",
			from.String(), to.String(),
			input, gas, value)
	}

	fmt.Fprintf(t.out, `
|  Pc   |      Op     | Cost |   Stack   |   RStack  |  Refund |
|-------|-------------|------|-----------|-----------|---------|
`)
}

// CaptureState also tracks SLOAD/SSTORE ops to track storage change.
func (t *mdLogger) CaptureState(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, rData []byte, depth int, err error) {
	stack := scope.Stack
	fmt.Fprintf(t.out, "| %4d  | %10v  |  %3d |", pc, op, cost)

	//if !t.cfg.DisableStack {
	if true {
		// format stack
		var a []string
		for _, elem := range stack.Data() {
			a = append(a, elem.Hex())
		}
		b := fmt.Sprintf("[%v]", strings.Join(a, ","))
		fmt.Fprintf(t.out, "%10v |", b)
	}
	fmt.Fprintf(t.out, "%10v |", t.env.StateDB.GetRefund())
	fmt.Fprintln(t.out, "")
	if err != nil {
		fmt.Fprintf(t.out, "Error: %v\n", err)
	}
}

func (t *mdLogger) CaptureFault(pc uint64, op OpCode, gas, cost uint64, scope *ScopeContext, depth int, err error) {
	fmt.Fprintf(t.out, "\nError: at pc=%d, op=%v: %v\n", pc, op, err)
}

func (t *mdLogger) CaptureEnd(output []byte, gasUsed uint64, tm time.Duration, err error) {
	fmt.Fprintf(t.out, "\nOutput: `%#x`\nConsumed gas: `%d`\nError: `%v`\n",
		output, gasUsed, err)
}

func (t *mdLogger) CaptureEnter(typ OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (t *mdLogger) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (*mdLogger) CaptureTxStart(gasLimit uint64) {}

func (*mdLogger) CaptureTxEnd(restGas uint64) {}
