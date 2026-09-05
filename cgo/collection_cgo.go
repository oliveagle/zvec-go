// Copyright 2025-present zvec project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cgoz provides an OPTIONAL, experimental CGO binding to the zvec C++
// core (the alibaba/zvec submodule under ./zvec, linked against the prebuilt
// static libraries in ./lib).
//
// It is NOT required by the pure-Go client (the root "zvec" package) or by the
// HTTP service (see ./server and ./cmd/zvec-httpd). To build against the real
// C++ core, enable the build tags:
//
//	CGO_ENABLED=1 go build -tags cgo,zvec_cgo ./...
//
// The checked-out zvec submodule must match the version the prebuilt static
// libraries in ../lib were built from (see ../lib/README.md for the hash).
//
// This binding currently exposes a small surface that verifies the
// CGO <-> zvec_core linkage (status code + default-message probes). The full
// Collection/Doc/Query surface is a follow-up.
//go:build cgo && zvec_cgo

package cgoz

/*
#cgo CFLAGS: -I${SRCDIR}/../zvec/src/include
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../lib/libzvec_core-linux-x86_64.a ${SRCDIR}/../lib/libzvec_ailego-linux-x86_64.a -lstdc++ -lpthread -lm
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../lib/libzvec_core-macos-arm64.a ${SRCDIR}/../lib/libzvec_ailego-macos-arm64.a -lstdc++ -lpthread -lm
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../lib/libzvec_core-macos-x86_64.a ${SRCDIR}/../lib/libzvec_ailego-macos-x86_64.a -lstdc++ -lpthread -lm

#include <zvec/db/status.h>

extern int zvec_go_status_ok_code(void);
extern const char* zvec_go_get_default_message(int code);
*/
import "C"

// Version identifies this experimental binding.
const Version = "experimental-cgo"

// StatusOKCode returns the numeric value of zvec::StatusCode::OK from the C++ core.
func StatusOKCode() int {
	return int(C.zvec_go_status_ok_code())
}

// DefaultMessage returns the default human-readable message for a zvec status
// code, as provided by the C++ core. The returned C string points to a static
// buffer owned by the core, so it must not be freed by the caller.
func DefaultMessage(code int) string {
	cs := C.zvec_go_get_default_message(C.int(code))
	if cs == nil {
		return ""
	}
	return C.GoString(cs)
}
