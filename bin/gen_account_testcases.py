#!/usr/bin/env python3

from datetime import timedelta
from string import Template

import dateutil.parser
import requests

ACCOUNTS = [
    "ndacc2gihhrj6rhe3v2jx5k6gqpedy878eaxn35j4tvcdirq",
    "ndadyrd7u7kyjkq9nwcz3rgyi3m6fyeexwwi6hy6giwby7wy",
]

ACCOUNT = "http://localhost:3032/account/account/{address}"
HISTORY = "http://localhost:3032/account/history/{address}"
TRANSACTION = "http://localhost:3032/transaction/{txhash}"

EPOCH = dateutil.parser.parse("2000-01-01T00:00:00Z")


def timestamp_ms(time):
    "Number of microseconds since the epoch; ndau-style"
    return (dateutil.parser.parse(time) - EPOCH) // timedelta(microseconds=1)


def unts_ms(ms):
    "Convert ndau-style microseconds past the epoch into a timestamp"
    return EPOCH + timedelta(microseconds=ms)


def getjs(endpoint):
    resp = requests.get(endpoint)
    resp.raise_for_status()
    return resp.json()


class Transaction:
    def __init__(self, txhash):
        self.hash = txhash
        self.data = getjs(TRANSACTION.format(txhash=txhash))


class Acct:
    def __init__(self, address):
        self.address = address
        self.history = getjs(HISTORY.format(address=self.address))["Items"]

        # it's better to just iterate directly on the list, but we're editing
        # the items in-place, which requires index-based access
        for idx in range(len(self.history)):
            txhash = self.history[idx]["TxHash"]
            self.history[idx]["tx"] = Transaction(txhash)

        self.validation_keys = ", ".join(
            f'"{k}"'
            for k in getjs(ACCOUNT.format(address=self.address))[self.address][
                "validationKeys"
            ]
        )


HEADER_TEMPLATE = """
package ndau

import (
    "encoding/base64"
    "testing"

    "github.com/oneiro-ndev/chaincode/pkg/vm"
    "github.com/oneiro-ndev/metanode/pkg/meta/app/code"
    metast "github.com/oneiro-ndev/metanode/pkg/meta/state"
    metatx "github.com/oneiro-ndev/metanode/pkg/meta/transaction"
    "github.com/oneiro-ndev/ndau/pkg/ndau/backing"
    "github.com/oneiro-ndev/ndaumath/pkg/address"
    "github.com/oneiro-ndev/ndaumath/pkg/constants"
    "github.com/oneiro-ndev/ndaumath/pkg/eai"
    "github.com/oneiro-ndev/ndaumath/pkg/signature"
    math "github.com/oneiro-ndev/ndaumath/pkg/types"
    sv "github.com/oneiro-ndev/system_vars/pkg/system_vars"
    "github.com/stretchr/testify/require"
)

func makeVKs(t *testing.T, keys ...string) []signature.PublicKey {
    vks := make([]signature.PublicKey, 0, len(keys))
    for _, ks := range keys {
        vk, err := signature.ParsePublicKey(ks)
        require.NoError(t, err)
        vks = append(vks, *vk)
    }
    return vks
}
"""

TEST_TX_TEMPLATE = """
    {
        data, err := base64.StdEncoding.DecodeString("$data")
        require.NoError(t, err)
        tx, err := metatx.Unmarshal(data, TxIDs)
        require.NoError(t, err)
        resp, _ := deliverTxContext(t, app, tx, context.at($timestamp))
        require.Equal(t, code.OK, code.ReturnCode(resp.Code))
        acct, _ := app.getAccount(addr)
        require.Equal(t, math.Ndau($balance), acct.Balance)
    }
""".strip(
    "\n"
)

TEST_TEMPLATE = """
func Test_${address}_History(t *testing.T) {
    app, _ := initApp(t)

    node1, err := address.Validate("ndarw5i7rmqtqstw4mtnchmfvxnrq4k3e2ytsyvsc7nxt2y7")
    require.NoError(t, err)
    modify(t, node1.String(), app, func(ad *backing.AccountData) {
        ad.Balance = 1500 * constants.NapuPerNdau
        ad.ValidationKeys = makeVKs(t,
            "npuba8jadtbbeamn89h5zgr5cmjggcwkchbsgqhf5m7zb58xe7rwqwvzif23ebfqz4wh224ve2qw",
            "npuba8jadtbbeabmk869zakhpzmiv2xvzc7yyxrzcmfu6eqbw9ttyi9bwrcpiz7jqki9pwsw7vsp",
            "npuba8jadtbbebivxyxnve83n7rwdmdzg3k3mpv7ed9y5jptgsnd5qf3uu9fx7sbddf63b636s3i",
            "npuba8jadtbbed6uj93t6c8hn72bt4ypw2rxx6zmfpcfkqmmxxt5m2e7ydit3gtfpt4quxzfcmkr",
        )
    })
    err = app.UpdateStateImmediately(func(stI metast.State) (metast.State, error) {
        st := stI.(*backing.State)
        st.Nodes[node1.String()] = backing.Node{
            Active: true,
        }
        return st, nil
    })
    require.NoError(t, err)

    node2, err := address.Validate("ndam75fnjn7cdues7ivi7ccfq8f534quieaccqibrvuzhqxa")
    require.NoError(t, err)
    modify(t, node2.String(), app, func(ad *backing.AccountData) {
        ad.Balance = 1500 * constants.NapuPerNdau
        ad.ValidationKeys = makeVKs(t,
            "npuba8jadtbbea97bcz4v2c4gtcntx53cgjpv92hscm95gg2m6tntwysawxkahkse4bcdpp5dm24",
            "npuba8jadtbbeabmk869zakhpzmiv2xvzc7yyxrzcmfu6eqbw9ttyi9bwrcpiz7jqki9pwsw7vsp",
            "npuba8jadtbbebivxyxnve83n7rwdmdzg3k3mpv7ed9y5jptgsnd5qf3uu9fx7sbddf63b636s3i",
            "npuba8jadtbbed6uj93t6c8hn72bt4ypw2rxx6zmfpcfkqmmxxt5m2e7ydit3gtfpt4quxzfcmkr",
        )
    })
    err = app.UpdateStateImmediately(func(stI metast.State) (metast.State, error) {
        st := stI.(*backing.State)
        st.Nodes[node2.String()] = backing.Node{
            Active: true,
        }
        return st, nil
    })
    require.NoError(t, err)

    ts := math.Timestamp($creation)
    // create the account
    // from https://github.com/oneiro-ndev/genesis/blob/master/pkg/etl/transform.go
    modify(t, "$address", app, func(ad *backing.AccountData) {
        ad.Balance = 1000 * constants.NapuPerNdau
        ad.LastEAIUpdate = ts
        ad.LastWAAUpdate = ts
        ad.CurrencySeatDate = &ts
        ad.Lock = backing.NewLock(
            math.Year + (2*math.Month) + (22*math.Day),
            eai.DefaultLockBonusEAI,
        )
        ad.Lock.Notify(ts, 0)
        ad.RecourseSettings.Period = math.Hour
        ad.ValidationKeys = makeVKs(t, $validation_keys)
    })

    addr, err := address.Validate("$address")
    require.NoError(t, err)
    err = app.UpdateStateImmediately(app.Delegate(addr, $node))
    require.NoError(t, err)

    // set the EAI overtime system var above what we need
    overtime, err := math.Duration(10 * math.Year).MarshalMsg(nil)
    require.NoError(t, err)
    // set the real transaction fee script
    txFeeScript, err := base64.StdEncoding.DecodeString("oAAlAIhSanSIoA0UEwkXFhANDgYIBwUEIyChB4igAxIMCyQA4fUFiKADFQIKgQCIoAEDIugDgQGIoAEBIugDgQGIoAEPIugDgQEjIKEHQIiAAAIJIhAnQiMgoQdAiIABAwlgCwlDgQKIgAIBBSMgoQfAiiMgoQcQjwUkgPD6AsSKJIDw+gIQj4g=")
    require.NoError(t, err)
    txFeeCC := vm.ToChaincode(txFeeScript)
    txFeeSV, err := txFeeCC.MarshalMsg(nil)
    require.NoError(t, err)
    context := ddc(t).with(func(svs map[string][]byte) {
        svs[sv.EAIOvertime] = overtime
        svs[sv.TxFeeScriptName] = txFeeSV
    })

    $txs
}
"""


def generate_tests():
    print(HEADER_TEMPLATE)
    tx_template = Template(TEST_TX_TEMPLATE)
    test_template = Template(TEST_TEMPLATE)
    for account, node in zip(ACCOUNTS, ["node1", "node2"]):
        acct = Acct(account)
        tx_tests = []
        for event in acct.history:
            tx_tests.append(
                tx_template.substitute(
                    id=event["tx"].data["Tx"]["TransactableID"],
                    data=event["tx"].data["TxBytes"],
                    timestamp=timestamp_ms(event["Timestamp"]),
                    balance=event["Balance"],
                )
            )
        print(
            test_template.substitute(
                address=account,
                creation=timestamp_ms("2018-04-05T00:00:00Z"),
                txs="\n".join(tx_tests),
                node=node,
                validation_keys=acct.validation_keys,
            )
        )


if __name__ == "__main__":
    generate_tests()
