#!/usr/bin/env python3

#  ----- ---- --- -- -
#  Copyright 2019 Oneiro NA, Inc. All Rights Reserved.
# 
#  Licensed under the Apache License 2.0 (the "License").  You may not use
#  this file except in compliance with the License.  You can obtain a copy
#  in the file LICENSE in the source distribution or at
#  https://www.apache.org/licenses/LICENSE-2.0.txt
#  - -- --- ---- -----

import os
import requests


def post_to_slack(message):
    """
    Post the given message to the Oneiro #deploys Slack channel.
    Must have SLACK_KEY environment variable set.
    """

    slack_key_name = "SLACK_DEPLOYS_KEY"
    slack_key_value = os.environ.get(slack_key_name, "")
    if len(slack_key_value) == 0:
        print(f"Unable to post to slack without {slack_key_name} env var: '{message}'")
        return

    url = f"https://hooks.slack.com/services/{slack_key_value}"
    body = {"text": message}
    r = requests.post(url, json=body)
    if r.status_code != 200:
        print(
            f"Got {r.status_code} when posting to slack because {r.reason}: '{message}'"
        )
    elif r.text != "ok":
        print(f"Posted to slack but got back non-ok response: '{message}'")
    else:
        print(f"Posted to slack: '{message}'")
