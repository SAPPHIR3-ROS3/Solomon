#!/usr/bin/env python3
"""Run a legacy upgrade with a controlling terminal, as an interactive CLI does."""

import os
import pty
import sys

if len(sys.argv) < 2:
    sys.exit("usage: upgrade_smoke_pty.py command [args...]")

sys.exit(os.waitstatus_to_exitcode(pty.spawn(sys.argv[1:])))
