package main

// ----- ---- --- -- -
// Copyright 2019 Oneiro NA, Inc. All Rights Reserved.
//
// Licensed under the Apache License 2.0 (the "License").  You may not use
// this file except in compliance with the License.  You can obtain a copy
// in the file LICENSE in the source distribution or at
// https://www.apache.org/licenses/LICENSE-2.0.txt
// - -- --- ---- -----

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ndau/chaincode/pkg/vm"
	"github.com/ndau/ndaumath/pkg/address"
)

// parseInt parses an integer from a string. It is just like
// strconv.ParseInt(s, 0, bitSize) except that it can handle binary.
func parseInt(s string, bitSize int) (int64, error) {
	if strings.HasPrefix(s, "0b") {
		return strconv.ParseInt(s[2:], 2, bitSize)
	}
	return strconv.ParseInt(s, 0, bitSize)
}

// parseUint parses an unsigned integer from a string. It is just like
// strconv.ParseUint(s, 0, bitSize) except that it can handle binary.
func parseUint(s string, bitSize int) (uint64, error) {
	if strings.HasPrefix(s, "0b") {
		return strconv.ParseUint(s[2:], 2, bitSize)
	}
	return strconv.ParseUint(s, 0, bitSize)
}

func toIfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

// Node is the fundamental unit that the parser manipulates (it builds a structure of nodes).
//Each node can emit itself as an array of bytes, or nil.
type Node interface {
	bytes() []byte
}

// Fixupper is an interface that is implemented by all nodes that need fixups and all nodes
// that contain other nodes as children. It is called before the bytes() function to allow
// nodes to do any fixing up necessary.
type Fixupper interface {
	fixup(map[string]int) error
}

// Script is the highest level node in the system
type Script struct {
	nodes []Node
	funcs map[string]int
}

var _ Node = (*Script)(nil)

func (n *Script) fixup() error {
	for _, op := range n.nodes {
		if f, ok := op.(Fixupper); ok {
			err := f.fixup(n.funcs)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *Script) bytes() []byte {
	var b []byte
	for _, op := range n.nodes {
		b = append(b, op.bytes()...)
	}
	return b
}

func newScript(nodes interface{}, funcs map[string]int) (*Script, error) {
	sl := toIfaceSlice(nodes)
	nodeArray := []Node{}
	for _, v := range sl {
		if n, ok := v.(Node); ok {
			nodeArray = append(nodeArray, n)
		}
	}
	return &Script{nodes: nodeArray, funcs: funcs}, nil
}

// HandlerDef is a node that expresses the information in a handler definition
type HandlerDef struct {
	ids   []byte
	nodes []Node
}

var _ Node = (*HandlerDef)(nil)

func (n *HandlerDef) bytes() []byte {
	if len(n.ids) == 1 && n.ids[0] == 0 {
		// optimization: if the only ID is 0 then we can just use "handler 0"
		n.ids = []byte{}
	}
	b := []byte{byte(vm.OpHandler), byte(len(n.ids))}
	b = append(b, n.ids...)
	for _, op := range n.nodes {
		b = append(b, op.bytes()...)
	}
	b = append(b, byte(vm.OpEndDef))
	return b
}

func (n *HandlerDef) fixup(funcs map[string]int) error {
	for _, op := range n.nodes {
		if f, ok := op.(Fixupper); ok {
			err := f.fixup(funcs)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func newHandlerDef(sids []string, nodes interface{}, constants map[string]string) (*HandlerDef, error) {
	sl := toIfaceSlice(nodes)
	nl := []Node{}
	for _, v := range sl {
		if n, ok := v.(Node); ok {
			nl = append(nl, n)
		}
	}

	ids := []byte{}
	for _, sid := range sids {
		s, ok := constants[sid]
		if !ok {
			s = sid
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			return nil, errors.New(sid + " is an invalid handler ID")
		}
		ids = append(ids, byte(id))
	}

	f := &HandlerDef{ids: ids, nodes: nl}
	return f, nil
}

// FunctionDef is a node that expresses the information in a function definition
type FunctionDef struct {
	name     string
	index    byte
	nodes    []Node
	argcount byte
}

var _ Node = (*FunctionDef)(nil)

func (n *FunctionDef) fixup(funcs map[string]int) error {
	me, ok := funcs[n.name]
	if ok {
		n.index = byte(me)
	} else {
		return fmt.Errorf("function %s not found in funcs map", n.name)
	}
	for _, op := range n.nodes {
		if f, ok := op.(Fixupper); ok {
			err := f.fixup(funcs)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *FunctionDef) bytes() []byte {
	b := []byte{byte(vm.OpDef), byte(n.index), byte(n.argcount)}
	for _, op := range n.nodes {
		b = append(b, op.bytes()...)
	}
	b = append(b, byte(vm.OpEndDef))
	return b
}

func newFunctionDef(name string, argcount string, nodes interface{}) (*FunctionDef, error) {
	argc, err := parseInt(argcount, 8)
	if err != nil {
		return nil, err
	}
	sl := toIfaceSlice(nodes)
	nl := []Node{}
	for _, v := range sl {
		if n, ok := v.(Node); ok {
			nl = append(nl, n)
		}
	}
	f := &FunctionDef{name: name, index: 0xff, nodes: nl, argcount: byte(argc)}
	return f, nil
}

// UnitaryOpcode is for opcodes that cannot take arguments
type UnitaryOpcode struct {
	opcode vm.Opcode
}

var _ Node = (*UnitaryOpcode)(nil)

func (n *UnitaryOpcode) bytes() []byte {
	return []byte{byte(n.opcode)}
}

func newUnitaryOpcode(op vm.Opcode) (*UnitaryOpcode, error) {
	return &UnitaryOpcode{opcode: op}, nil
}

// BinaryOpcode is for opcodes that take one single-byte argument
type BinaryOpcode struct {
	opcode vm.Opcode
	value  byte
}

var _ Node = (*BinaryOpcode)(nil)

func (n BinaryOpcode) bytes() []byte {
	return []byte{byte(n.opcode), n.value}
}

func newBinaryOpcode(op vm.Opcode, v string) (*BinaryOpcode, error) {
	n, err := parseUint(v, 8)
	if err != nil {
		return &BinaryOpcode{}, err
	}
	return &BinaryOpcode{opcode: op, value: byte(n)}, nil
}

// CallOpcode is for opcodes that call a function and take a function name
type CallOpcode struct {
	opcode vm.Opcode
	name   string
	fix    byte
}

var _ Node = (*CallOpcode)(nil)

func (n *CallOpcode) fixup(funcs map[string]int) error {
	me, ok := funcs[n.name]
	if !ok || byte(me) == 0xff {
		return errors.New("unable to fix up call to " + n.name)
	}
	n.fix = byte(me)
	return nil
}

func (n *CallOpcode) bytes() []byte {
	return []byte{byte(n.opcode), n.fix}
}

func newCallOpcode(op vm.Opcode, name string) (*CallOpcode, error) {
	return &CallOpcode{opcode: op, name: name, fix: 0xff}, nil
}

// DecoOpcode is for Deco, which calls a function and takes a function name
// as well as a field index
type DecoOpcode struct {
	opcode vm.Opcode
	name   string
	field  byte
	fix    byte
}

var _ Node = (*DecoOpcode)(nil)

func (n *DecoOpcode) fixup(funcs map[string]int) error {
	me, ok := funcs[n.name]
	if !ok || byte(me) == 0xff {
		return errors.New("unable to fix up deco to " + n.name)
	}
	n.fix = byte(me)
	return nil
}

func (n *DecoOpcode) bytes() []byte {
	return []byte{byte(n.opcode), n.fix, n.field}
}

func newDecoOpcode(op vm.Opcode, name string, fieldid string) (*DecoOpcode, error) {
	fid, err := parseInt(fieldid, 8)
	if err != nil {
		return nil, err
	}
	return &DecoOpcode{opcode: op, name: name, field: byte(fid)}, nil
}

// PushOpcode constructs push operations with the appropriate number of bytes to express
// the specified value. It has special cases for the special opcodes zero, one, and neg1.
type PushOpcode struct {
	arg int64
}

var _ Node = (*PushOpcode)(nil)

// This function builds a sequence of bytes consisting of either:
//   A ZERO, ONE, or NEG1 opcode
// OR
//   A PushN opcode followed by N bytes, where N is a value from 1-8.
//   The bytes are a representation of the value in little-endian order (low
//   byte first). The highest bit is the sign bit.
func (n *PushOpcode) bytes() []byte {
	switch n.arg {
	case 0:
		return []byte{byte(vm.OpZero)}
	case 1:
		return []byte{byte(vm.OpOne)}
	case -1:
		return []byte{byte(vm.OpNeg1)}
	default:
		// In order to minimize the size of generated code, we want to generate a pushN opcode
		// of the minimal number of bytes. The VM knows how to sign-extend values, so we can
		// suppress FF values if the preceding value has the high bit set, and we can suppress
		// 0 values as long as the preceding value has the high bit off.
		b := vm.ToBytes(n.arg)
		for ix := len(b) - 1; ix > 0; ix-- {
			if ((b[ix-1]&0x80 == 0x80) && b[ix] == 0xFF) ||
				((b[ix-1]&0x80 == 0x00) && b[ix] == 0x00) {
				b = b[:ix]
			} else {
				break
			}
		}

		nbytes := byte(len(b))
		// All the PushN opcodes are related
		op := byte(vm.OpPush1) + (nbytes - 1)
		b = append([]byte{op}, b...)
		return b
	}
}

func newPushOpcode(s string) (*PushOpcode, error) {
	v, err := parseInt(s, 64)
	return &PushOpcode{arg: v}, err
}

// PushB is an array of bytes
type PushB struct {
	b []byte
}

var _ Node = (*PushB)(nil)

func newPushB(iface interface{}) (*PushB, error) {
	ia := toIfaceSlice(iface)
	out := make([]byte, 0)

	for _, item := range ia {
		if b, ok := item.([]byte); ok {
			// if the item is a []byte, it was a quoted string so just embed the bytes of the string
			out = append(out, b[0])
		} else {
			// if the item was a string, it should be parsed as the representation of a value,
			// either decimal or hex, or possibly as an address (which starts with "addr(")
			s := item.(string)
			if strings.HasPrefix(s, "addr(") {
				// we couldn't get here if it wasn't valid hex
				addr, _ := hex.DecodeString(s[5 : len(s)-1])
				out = append(out, addr...)
			} else {
				v, err := strconv.ParseUint(s, 0, 8)
				if err != nil {
					return nil, err
				}
				out = append(out, byte(v))
			}
		}
	}
	return &PushB{out}, nil
}

// this pushes an address onto the stack as an array of bytes corresponding
// to the string version of the address.
// TODO: consider doing this in the decoded form
func newPushAddr(addr string) (*PushB, error) {
	_, err := address.Validate(addr)
	if err != nil {
		return nil, err
	}
	return &PushB{[]byte(addr)}, nil
}

func (n *PushB) bytes() []byte {
	out := append([]byte{byte(vm.OpPushB)}, byte(len(n.b)))
	return append(out, n.b...)
}

// PushTimestamp is a 64-bit representation of the time since the start of the epoch in microseconds
type PushTimestamp struct {
	t int64
}

var _ Node = (*PushTimestamp)(nil)

func newPushTimestamp(s string) (*PushTimestamp, error) {
	ts, err := vm.ParseTimestamp(s)
	if err != nil {
		return &PushTimestamp{}, err
	}
	return &PushTimestamp{ts.T()}, nil
}

func (n *PushTimestamp) bytes() []byte {
	return append([]byte{byte(vm.OpPushT)}, vm.ToBytes(n.t)...)
}
