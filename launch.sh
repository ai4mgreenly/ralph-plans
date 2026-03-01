#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
reset-repo
make
clear
exec ./ralph-plans
