#!/usr/bin/env python3
"""Compatibility entry point for Windropolis fixed NIST controls."""
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from ai_validation.fixed import *  # noqa: F401,F403 - old offline tests import these helpers
from ai_validation.run_tests import main as validation_main

if __name__ == "__main__":
    sys.exit(validation_main(["--group", "fixed", *sys.argv[1:]]))
