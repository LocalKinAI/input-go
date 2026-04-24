// Package dylib embeds the ObjC companion library so downstream users can
// simply `go get` input-go without building C code on their machine.
//
// The embedded dylib is a Mach-O universal binary (arm64 + x86_64) built
// from objc/input_events.m. It is extracted to the user's cache directory
// on first use; see input.Load for details.
//
// This package is internal: callers of input-go should never need to import it
// directly. Expose the bytes via [Bytes].
package dylib

import _ "embed"

// Bytes holds the embedded universal-Mach-O bytes of libinput_sync.dylib.
// It is frozen at build time — updating requires rerunning `make dylib`.
//
//go:embed libinput_sync.dylib
var Bytes []byte
