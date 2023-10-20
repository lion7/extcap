package extcap

import "errors"

var (
	// ErrNoInterfaceSpecified is returned when start capture is called without specifying an interface
	// also returned when querying configuration options or supported DLTs without specifying an interface
	ErrNoInterfaceSpecified = errors.New("no interface specified")

	// ErrNoPipeProvided is returned when start capture is called without providing the FIFO pipe to write to
	ErrNoPipeProvided = errors.New("no FIFO pipe provided")
)
