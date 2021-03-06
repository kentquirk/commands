#! /usr/bin/env python3

#  ----- ---- --- -- -
#  Copyright 2019 Oneiro NA, Inc. All Rights Reserved.
# 
#  Licensed under the Apache License 2.0 (the "License").  You may not use
#  this file except in compliance with the License.  You can obtain a copy
#  in the file LICENSE in the source distribution or at
#  https://www.apache.org/licenses/LICENSE-2.0.txt
#  - -- --- ---- -----

"""
Reads .crankgen files and creates iterations of them based on iterating
through values and plugging those values into a template; relieve tedium in
testing combinations of chasm code.
It supports two kinds of substitutions:
If you do something like:

```
VAR a = A, B, C
VAR n = 1, 2, 3
BEGIN_TEMPLATE
Here we go: $a$n = {-{n*100}-}
```

You'll get 9 files for all combinations of a and n, and $a will get the current
value of a, and $n will get the current value of n. After that, text between {-{ and }-}
will be evaluated as a python expression, converted to a string, and substituted
for the original. So file 8 will look like this:
`Here we go: C2 = 200`

It is also legal to evaluate expressions within the header. This allows you to
do things like:

```
VAR a = 2, 4, 6
VAR n = 1, 2, 3
LAMBDA sum = a+n
BEGIN_TEMPLATE
assert $a + $n == $sum
assert $sum - $a == $n
assert $sum - $n == $a
```

This will compute $sum for each combination of $a and $n, and inject the value
appropriately in the output file. The advantage is that you don't need to copy
and paste the same expression multiple times.

LAMBDAS are evaluated sequentially, so it is safe to refer to a previous lambda
within a subsequent one. However, recursion is not supported.

Because LAMBDA and VAR aggressively attempt to analyze the type of their arguments,
it is sometimes necessary to tell VAR to simply treat them as strings. In this instance,
you can use SVAR instead of VAR.

In order to support timestamp calculations, a helper function exists in the LAMBDA
namespace called `ts`. This accepts a string in RFC3339 format, exactly as
accepted by chaincode, and returns a `datetime.datetime` object.
"""

import itertools
import os
import re
import sys
from collections import OrderedDict
from datetime import datetime
from string import Template

EVAL_PAT = re.compile(r"{-{(.*?)}-}", re.DOTALL)


def evalText(m):
    expr = m.group(1)
    try:
        result = eval(expr)
        return str(result)
    except Exception:
        # print(expr)
        return expr


def parse_as(val, *types):
    """
    For each type in types, attempt to parse the value as that type.

    Return the first parse which does not raise ValueError.
    If no type succeeds, return the string value.
    """
    for typ in types:
        try:
            return typ(val)
        except ValueError:
            pass
    return quote(val)


def parse_ts(timestamp):
    # microseconds are optinal, which means we have to check multiple formats;
    # timestamp formats can't have optional values
    for fmt in ("%Y-%m-%dT%H:%M:%S.%fZ", "%Y-%m-%dT%H:%M:%SZ"):
        try:
            return datetime.strptime(timestamp, fmt)
        except ValueError:
            pass  # just try the next format
    raise ValueError(f"timestamp {timestamp} did not match any parseable time format")


def quote(s):
    if len(s) < 2 or s[0] != '"' or s[-1] != '"':
        return f'"{s}"'
    return s


def generate(fname):
    var_lines = {}
    vars = {}
    lams = OrderedDict()
    templ = []
    recording = False
    with open(fname) as f:
        for lineno, l in enumerate(f):
            stripped = l.strip()
            if l.startswith("SVAR") or l.startswith("VAR") or l.startswith("LAMBDA"):
                lhs, rhs = [s.strip() for s in l.split("=", maxsplit=1)]
                # split off the keyword
                _, name = lhs.split(maxsplit=1)
                var_lines[name] = lineno + 1  # use 1-based indexing
                if lhs.startswith("VAR"):
                    vars[name] = [
                        (name, parse_as(s.strip(), int, float)) for s in rhs.split(",")
                    ]
                elif lhs.startswith("SVAR"):
                    vars[name] = [(name, s.strip()) for s in rhs.split(",")]
                else:
                    lams[name] = rhs
            elif stripped == "BEGIN_TEMPLATE":
                recording = True
            elif recording is True:
                templ.append(l)

    values = vars.values()
    combos = [dict(x) for x in itertools.product(*values)]
    for idx, combo in enumerate(combos):
        # add utility functions to each combo
        combo["ts"] = parse_ts

        for name, lam in lams.items():
            try:
                combo[name] = eval(lam, combo)
                if isinstance(combo[name], str):
                    # strings have to be re-quoted
                    combo[name] = quote(combo[name])
            except Exception as e:
                print(f"{e.__class__.__name__}: {e} at line {var_lines[name]}")
                sys.exit(1)
        combos[idx] = combo

    template = Template("".join(templ))

    name, ext = os.path.splitext(fname)
    ext = ".crank"
    for index in range(len(combos)):
        outf = open(f"{name}_{index+1}_gen{ext}", "w")
        print(
            f"; GENERATED BY generate.py FROM {os.path.basename(fname)} - DO NOT EDIT",
            file=outf,
        )
        try:
            txt = template.substitute(combos[index])
        except Exception as e:
            print(repr(e), file=sys.stderr)
            print("locals:", combos[index], file=sys.stderr)
            print(f"Error processing template {fname}, index {index}", file=sys.stderr)
            sys.exit(1)
        else:
            txt = EVAL_PAT.sub(evalText, txt)
            outf.write(txt)
            outf.close()


if __name__ == "__main__":
    for x in sys.argv[1:]:
        generate(x)
