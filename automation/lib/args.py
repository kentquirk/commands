#!/usr/bin/env python3

#  ----- ---- --- -- -
#  Copyright 2019 Oneiro NA, Inc. All Rights Reserved.
# 
#  Licensed under the Apache License 2.0 (the "License").  You may not use
#  this file except in compliance with the License.  You can obtain a copy
#  in the file LICENSE in the source distribution or at
#  https://www.apache.org/licenses/LICENSE-2.0.txt
#  - -- --- ---- -----

from lib.networks import Network
from argparse import ArgumentParser
from enum import Enum


class UrlKind(Enum):
    """
    Supported url kinds.
    """

    API = "API"
    RPC = "RPC"

    def __str__(self):
        return self.value


def get_url(kind):
    """
    Return the url argument.
    """

    parser = ArgumentParser()
    parser.add_argument(f"url", help=f"{kind} URL of the form protocol://domain:port")
    args = parser.parse_args()
    return args.url


def get_net():
    """
    Return an optional net argument.
    """

    parser = ArgumentParser()
    parser.add_argument(
        "net", nargs="?", type=Network, choices=list(Network), help="network name"
    )
    args = parser.parse_args()
    return args.net


def get_node():
    """
    Return a lone node argument.
    """

    parser = ArgumentParser()
    parser.add_argument("node", help="node name")
    args = parser.parse_args()
    return args.node


def get_net_node_sha_snapshot_repave():
    """
    Return the net argument and optional node name and sha arguments.
    """

    parser = ArgumentParser()
    parser.add_argument("net", type=Network, choices=list(Network), help="network name")
    parser.add_argument(
        "--node", required=False, help="node name (defaults to all nodes)"
    )
    parser.add_argument(
        "--repave", required=False, action='store_true', help="repave network to specified snapshot"
    )
    parser.add_argument("--sha", required=True, help="ECR ndauimage SHA to use")
    parser.add_argument(
        "--snapshot",
        required=False,
        help="snapshot from which to catch up (e.g. snapshot-mainnet-1; defaults to latest)",
    )
    args = parser.parse_args()
    return args.net, args.node, args.sha, args.snapshot, args.repave
