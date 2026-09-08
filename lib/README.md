# zvec Native Libraries (C API)

Prebuilt, self-contained shared libraries for the zvec Go client's optional
CGO binding (../cgo). Each library is the official zvec C API build
(`zvec_c_api`) produced by the alibaba/zvec project: it embeds the entire
C++ core plus all third-party dependencies, so binaries built against it have
no runtime dependencies beyond system libraries (libc / libm / libpthread /
libdl).

## Layout

    lib/
    ├── linux-x86_64/libzvec_c_api.so    # Linux x86_64
    ├── linux-arm64/libzvec_c_api.so     # Linux ARM64
    ├── macos-arm64/libzvec_c_api.dylib  # macOS Apple Silicon
    └── data/jieba_dict/                 # FTS tokenizer data (all platforms)
        ├── jieba.dict.utf8
        └── hmm_model.utf8

The file name must stay `libzvec_c_api.{so,dylib}` (it equals the library's
SONAME) because the cgo build rules link the file directly and set
`-Wl,-rpath` to the per-platform subdirectory, so the dynamic loader finds it
next to the binary layout at runtime. Do not rename or add platform suffixes.

## zvec version

The libraries correspond to the zvec C++ submodule pinned in this repository
(currently **v0.7.0** — see `zvec/` and the zvec gitlink). They are the
prebuilt SDK assets of the matching alibaba/zvec GitHub release
(`zvec-sdk-<platform>.tar.gz`), not a from-source build of this checkout.
When the submodule moves to a new version, re-download the matching release
SDKs and replace the files here (see
[`build-zvec-native-libraries`](../.github/workflows/build-zvec-native-libraries.yml)
for the automated flow).

## Usage

The CGO binding is optional; the pure-Go client (root package) and the HTTP
service (`./server`, `./cmd/zvec-httpd`) do not need these libraries.

```bash
# Build/test the CGO binding (requires a C compiler, e.g. gcc/clang):
CGO_ENABLED=1 go build -tags 'cgo zvec_cgo' ./cgo/
CGO_ENABLED=1 go test -tags 'cgo zvec_cgo' -v ./cgo/
```

The cgo flags in `../cgo/collection_cgo.go` link the per-platform library and
embed an rpath pointing at its subdirectory, so no `LD_LIBRARY_PATH` or
`DYLD_LIBRARY_PATH` setup is needed.

### Full-text search (jieba) data

The FTS pipeline uses the jieba tokenizer, which reads
`lib/data/jieba_dict/jieba.dict.utf8` and `hmm_model.utf8` at runtime. Point
the environment variable `ZVEC_JIEBA_DICT_DIR` at `lib/data/jieba_dict`
(absolute path recommended) before running applications that use FTS
indexes.

## Provenance / updating the libraries

1. Find the zvec version this repo pins to: `git describe --tags --exact-match zvec/`
   (or `cd zvec && git describe --tags --exact-match HEAD`).
2. Download the release assets for that tag from
   `https://github.com/alibaba/zvec/releases` — one per platform:
   `zvec-sdk-linux-amd64.tar.gz`, `zvec-sdk-linux-arm64.tar.gz`,
   `zvec-sdk-osx-arm64.tar.gz`.
3. From each archive keep only `libzvec_c_api.{so,dylib}` (and
   `data/jieba_dict/*` for the data directory) and place them under the
   matching `lib/<platform>/` subdirectory above.
4. Run `CGO_ENABLED=1 go test -tags 'cgo zvec_cgo' -v ./cgo/` on the target
   platform to verify, then commit.

The GitHub Actions workflow
[`build-zvec-native-libraries.yml`](../.github/workflows/build-zvec-native-libraries.yml)
automates steps 1–4 and commits the result when the `zvec/` submodule or the
workflow itself changes.
