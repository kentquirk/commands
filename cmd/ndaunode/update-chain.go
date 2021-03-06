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
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	metast "github.com/ndau/metanode/pkg/meta/state"
	"github.com/ndau/ndau/pkg/ndau"
	"github.com/ndau/ndau/pkg/ndau/backing"
	"github.com/ndau/ndau/pkg/ndau/config"
	"github.com/ndau/ndaumath/pkg/address"
	"github.com/ndau/ndaumath/pkg/signature"
	math "github.com/ndau/ndaumath/pkg/types"
	generator "github.com/ndau/system_vars/pkg/genesis.generator"
	"github.com/ndau/system_vars/pkg/genesisfile"
	sv "github.com/ndau/system_vars/pkg/system_vars"
	"github.com/pkg/errors"
)

func updateFromGenesis(gfilePath, asscpath string, conf *config.Config) {
	app, err := ndau.NewAppSilent(getDbSpec(), "", -1, *conf)
	check(err)

	check(app.UpdateStateImmediately(func(stI metast.State) (metast.State, error) {
		st := stI.(*backing.State)

		if gfilePath != "" {
			gfile, err := genesisfile.Load(gfilePath)
			if err != nil {
				return st, errors.Wrap(err, "loading genesis file")
			}
			st.Sysvars, err = gfile.IntoSysvars()
			if err != nil {
				return st, errors.Wrap(err, "converting genesis file into sysvars")
			}
		}

		if asscpath != "" {
			assc := make(generator.Associated)
			_, err := toml.DecodeFile(asscpath, &assc)
			if err != nil {
				return st, errors.Wrap(err, "decoding associated file")
			}

			for _, sa := range []sv.SysAcct{
				sv.CommandValidatorChange,
				sv.NodeRulesAccount,
				sv.NominateNodeReward,
				sv.ReleaseFromEndowment,
				sv.RecordPrice,
				sv.SetSysvar,
			} {
				addrV, addrok := assc[sa.Address]
				valkeyV, valkeyok := assc[sa.Validation.Public]
				if !(addrok && valkeyok) {
					continue
				}

				// parse address
				addrS, ok := addrV.(string)
				if !ok {
					return st, fmt.Errorf("%s address not stored as string", sa.Name)
				}

				addr, err := address.Validate(addrS)
				if err != nil {
					return st, errors.Wrap(err, "validating "+sa.Address)
				}

				// parse pubkey
				valkeyS, ok := valkeyV.(string)
				if !ok {
					return st, fmt.Errorf("%s validator public key not stored as string", sa.Name)
				}

				var valkey signature.PublicKey
				err = valkey.UnmarshalText([]byte(valkeyS))
				if err != nil {
					return st, errors.Wrap(err, sa.Validation.Public+" invalid")
				}

				// update state
				now, err := math.TimestampFrom(time.Now())
				if err != nil {
					return st, errors.Wrap(err, "computing current timestamp")
				}

				// it would be a pain to ensure that we had system variables here,
				// and this applies only to special accounts anyway, so the best
				// solution is to have them simply start with a 0 recourse period.
				ad, _ := st.GetAccount(addr, now, 0)

				ad.ValidationKeys = append(ad.ValidationKeys, valkey)
				st.Accounts[addr.String()] = ad
			}
		}

		return st, nil
	}))
}
