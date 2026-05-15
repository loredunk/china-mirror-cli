// Package sysexits exposes a small subset of BSD sysexits.h codes used by
// the OpenCLI convention.
package sysexits

const (
	OK           = 0
	Usage        = 64 // command line usage error
	DataErr      = 65 // data format error
	NoInput      = 66 // cannot open input (also: empty result)
	Unavailable  = 69 // service unavailable
	Software     = 70 // internal software error
	IOErr        = 74
	TempFail     = 75
	NoPerm       = 77 // permission / auth required
	ConfigErr    = 78 // configuration error
)
