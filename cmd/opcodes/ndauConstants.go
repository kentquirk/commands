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
	"sort"
	"strings"

	"github.com/ndau/chaincode/pkg/chain"
	metatx "github.com/ndau/metanode/pkg/meta/transaction"
	"github.com/ndau/ndau/pkg/ndau"
	"github.com/ndau/ndau/pkg/ndau/backing"
)

type index struct {
	Name  string
	Value byte
}

func getNdauIndexMap() map[string]byte {
	indices := make(map[string]byte)
	indices["EVENT_DEFAULT"] = 0

	// these are a couple other objects that are not accounted for in the transaction
	// iteration below
	objects := []interface{}{
		backing.AccountData{},
		backing.Lock{},
	}

	// Find the list of transactions and use their names for events, and add their
	// objects to the list we walk for extracting constants
	for txid, tx := range ndau.TxIDs {
		name := metatx.NameOf(tx)
		eventname := "EVENT_" + strings.ToUpper(name)
		indices[eventname] = byte(txid)
		objects = append(objects, tx)
	}

	for _, o := range objects {
		ks, _ := chain.ExtractConstants(o)
		for k, v := range ks {
			indices[k] = v
		}
	}

	return indices
}

// returns the portion of the string before the first _, or the whole string
func getPrefix(s string) string {
	ix := strings.Index(s, "_")
	if ix < 1 {
		return s
	}
	return s[:ix-1]
}

func getNdauIndices() []index {
	indices := getNdauIndexMap()
	out := make([]index, 0)
	for k, v := range indices {
		out = append(out, index{Name: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool {
		prefixi := getPrefix(out[i].Name)
		prefixj := getPrefix(out[j].Name)
		if prefixi == prefixj {
			return out[i].Value < out[j].Value
		}
		return prefixi < prefixj
	})
	return out
}
