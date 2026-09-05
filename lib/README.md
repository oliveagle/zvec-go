# zvec Static Libraries

Pre-built static libraries for zvec Go client (CGO binding).

## Library Naming Convention

Libraries are named as: `libzvec_core-<os>-<arch>.a`

| OS | Architecture | Library File |
|----|--------------|--------------|
| Linux | x86_64 | `libzvec_core-linux-x86_64.a` |
| macOS | arm64 (Apple Silicon) | `libzvec_core-macos-arm64.a` |
| macOS | x86_64 (Intel) | `libzvec_core-macos-x86_64.a` |
| Windows | x86_64 | `libzvec_core-windows-x86_64.lib` |

## Auxiliary Libraries

| OS | Architecture | Library File |
|----|--------------|--------------|
| Linux | x86_64 | `libzvec_ailego-linux-x86_64.a` |
| macOS | arm64 | `libzvec_ailego-macos-arm64.a` |
| macOS | x86_64 | `libzvec_ailego-macos-x86_64.a` |
| Windows | x86_64 | `libzvec_ailego-windows-x86_64.lib` |

## zvec version

Libraries are built from the `zvec` C++ submodule, which is pinned to a specific
version in this repository (currently **v0.7.0** — see `zvec/`). The exact source
commit is recorded in the `zvec` gitlink.

Libraries are rebuilt and committed automatically by the
[`build-zvec-static-libraries`](../.github/workflows/build-zvec-static-libraries.yml)
GitHub Actions workflow whenever the `zvec/` submodule or the workflow changes, so the
checked-in `.a` files track the pinned zvec version. The filename convention below
(`libzvec_core-<os>-<arch>.a`) is what the CGO build tags in `../cgo/` link against;
the `*-<git-hash>.a` variants are historical artifacts from manual builds.

## Usage

### Linux

```bash
# Install dependencies
sudo apt-get install -y cmake build-essential

# Build from source or download pre-built library
# Library path: lib/libzvec_core-linux-x86_64.a
```

### macOS

```bash
# Install dependencies
brew install cmake

# For Apple Silicon (M1/M2)
# Library path: lib/libzvec_core-macos-arm64.a

# For Intel Mac
# Library path: lib/libzvec_core-macos-x86_64.a
```

### Windows

```powershell
# Install Visual Studio 2022 with C++ workload
# Install CMake

# Library path: lib/libzvec_core-windows-x86_64.lib
```

## CGO Linking

The Go client uses CGO to link against these static libraries:

```go
/*
#cgo CFLAGS: -I./zvec/src/include
#cgo LDFLAGS: -L./lib -lzvec_core -lzvec_ailego -lstdc++ -lpthread -lm
*/
import "C"
```

## Build from GitHub Actions

Libraries are automatically built and committed to this repository by the
`build-zvec-static-libraries.yml` GitHub Actions workflow.

Triggered by:
- Push to `main` branch (when zvec/ submodule or workflow file changes)
- Manual trigger via GitHub Actions UI
