package main

import (
	"flag"
	"fmt"
)

// parseFlagSet parses args and allows flags after positional operands.
// encoding/flag stops at the first non-flag; `one change create slug --title X`
// must still bind --title.
func parseFlagSet(fs *flag.FlagSet, args []string) error {
	var positionals []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		got := fs.Args()
		if len(got) == 0 {
			break
		}
		positionals = append(positionals, got[0])
		rest = got[1:]
	}
	if err := fs.Parse(positionals); err != nil {
		return err
	}
	if extra := fs.Args(); len(extra) != len(positionals) {
		return fmt.Errorf("unexpected arguments: %v", extra)
	}
	return nil
}
