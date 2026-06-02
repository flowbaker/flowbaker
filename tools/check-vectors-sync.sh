#!/usr/bin/env bash
set -euo pipefail

A="${1:-pkg/expressions/vectors.json}"
B="${2:-../workflow-editor-react/src/components/Node/ExpressionInput/cel/vectors.json}"

if [ ! -f "$A" ]; then echo "missing: $A" >&2; exit 2; fi
if [ ! -f "$B" ]; then echo "missing: $B" >&2; exit 2; fi

sa=$(shasum -a 256 "$A" | awk '{print $1}')
sb=$(shasum -a 256 "$B" | awk '{print $1}')

if [ "$sa" != "$sb" ]; then
  echo "VECTORS DRIFT" >&2
  echo "  $A  $sa" >&2
  echo "  $B  $sb" >&2
  diff -u "$A" "$B" || true
  exit 1
fi

echo "vectors in sync: $sa"
