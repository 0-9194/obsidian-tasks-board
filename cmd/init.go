package cmd

import (
	"fmt"
	"os"

	initvault "github.com/pot-labs/otb/internal/init"
)

// InitFlags holds parsed CLI flags for the init subcommand.
type InitFlags struct {
	Name   string
	Dir    string
	Author string
	Force  bool
}

// RunInit is the entry point for `otb init`.
func RunInit(flags InitFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	opts := initvault.Options{
		Name:   flags.Name,
		Dir:    flags.Dir,
		Author: flags.Author,
		Force:  flags.Force,
	}

	_, err = initvault.Run(cwd, opts, os.Stdout)
	return err
}
