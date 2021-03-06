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
	"bytes"
	"io"
	"log"
	"os"

	arg "github.com/alexflint/go-arg"
	"github.com/ndau/chaincode/pkg/vm"
)

func main() {
	var args struct {
		Input   string `arg:"positional"`
		Output  string `arg:"-o" help:"Output filename"`
		Comment string `arg:"-c" help:"Comment to embed in the output file."`
		Debug   bool   `arg:"-d" help:"Dump the code after a successful assembly."`
	}
	arg.MustParse(&args)

	name := "stdin"
	in := os.Stdin
	if args.Input != "" {
		name = args.Input
		f, err := os.Open(name)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		in = f
	}

	var buf bytes.Buffer
	tee := io.TeeReader(in, &buf)

	sn, err := ParseReader(name,
		tee,
		GlobalStore("functions", make(map[string]int)),
		GlobalStore("functionCounter", int(0)),
		GlobalStore("constants", predefinedConstants()),
	)
	if err != nil {
		log.Fatal(describeErrors(err, buf.String()))
	}

	out := os.Stdout
	if args.Output != "" {
		f, err := os.Create(args.Output)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		out = f
	}

	if err := sn.(*Script).fixup(); err != nil {
		log.Fatal(err)
	}
	b := sn.(*Script).bytes()
	err = vm.Serialize(name, args.Comment, b, out)
	if err != nil {
		log.Fatal(err)
	}

	if args.Debug {
		var buf bytes.Buffer
		vm.Serialize(name, args.Comment, b, &buf)
		bin, _ := vm.Deserialize(&buf)
		if err != nil {
			log.Fatal(err)
		}
		thevm, err := vm.New(bin)
		if err != nil {
			log.Fatal(err)
		}
		thevm.DisassembleAll(os.Stdout)
	}
}
