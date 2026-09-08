#!/usr/bin/env bash
# Ensure the zvec C++ submodule carries a nested go.mod boundary file.
#
# The upstream project (alibaba/zvec) is C++ and never committed a go.mod,
# but this parent repo needs one so that `go build ./...` from the root does
# not descend into zvec/thirdparty/** (vendored antlr/arrow/protobuf/thrift
# Go sources that do not belong to this module).
#
# The file is local to the submodule (untracked upstream), so it is lost on
# a fresh clone or `git submodule update --init`. Run this script after
# cloning:
#
#   git submodule update --init zvec
#   scripts/ensure-zvec-gomod.sh
#
# Idempotent: does nothing if zvec/go.mod already exists.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ZVEC_DIR="$ROOT/zvec"

if [ ! -d "$ZVEC_DIR/.git" ] && [ ! -f "$ZVEC_DIR/.git" ]; then
  echo "zvec/ submodule is not initialized; run first:" >&2
  echo "  git submodule update --init zvec" >&2
  exit 1
fi

if [ -f "$ZVEC_DIR/go.mod" ]; then
  echo "zvec/go.mod already present; nothing to do."
  exit 0
fi

cat > "$ZVEC_DIR/go.mod" <<'GOMOD'
module github.com/alibaba/zvec

go 1.22

// This go.mod exists only to make the zvec C++ submodule a nested Go module
// boundary, so that the parent module (github.com/oliveagle/zvec-go) does not
// descend into zvec/thirdparty/** (antlr, arrow, protobuf, thrift) Go sources.
// The zvec C++ project itself is built via CMake, not Go.
GOMOD

echo "created $ZVEC_DIR/go.mod (nested Go module boundary)"
