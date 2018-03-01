package broker

import (
	"flag"
)

// Options holds the options specified by on the command line.
type Options struct {
	CatalogPath string
	Async       bool
}

// AddFlags is a hook called to initialize the CLI flags for the broker options, it
// is called after the flags are added for the skeleton and before flag. Parse is
// called.
func AddFlags(o *Options) {
	flag.StringVar(&o.CatalogPath, "catalogPath", "", "The path to the catalog")
	flag.BoolVar(&o.Async, "async", false, "Indicates whether the broker is handling the requests asynchronously.")
}
