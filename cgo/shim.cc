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

// C shim exposing a minimal, C-linkage surface over the zvec C++ core.
//
// It deliberately references real core symbols (zvec::StatusCode,
// zvec::GetDefaultMessage) so that building with -tags cgo,zvec_cgo actually
// exercises linking against the zvec static library.
//go:build cgo && zvec_cgo

#include <zvec/db/status.h>

extern "C" int zvec_go_status_ok_code(void) {
  return static_cast<int>(zvec::StatusCode::OK);
}

extern "C" const char *zvec_go_get_default_message(int code) {
  return zvec::GetDefaultMessage(static_cast<zvec::StatusCode>(code));
}
