// otb — Obsidian Tasks Board
//
// A self-contained TUI kanban board for Obsidian vaults using the Tasks plugin.
// Compiles to a single static binary with no runtime dependencies.
//
// Usage:
//
//	otb [board] [--vault <path>] [--project <name>]
//	otb init [--name <name>] [--dir <path>] [--author <handle>] [--force]
package main

import (
	"fmt"
	"os"

	"github.com/pot-labs/otb/cmd"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		// Default: run board
		runBoard(args)
		return
	}

	switch args[0] {
	case "board":
		runBoard(args[1:])
	case "init":
		runInit(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("otb %s\n", version)
	case "help", "--help", "-h":
		printHelp()
	default:
		// Treat unknown first arg as --vault path (backward-compat) or show help
		runBoard(args)
	}
}

func runBoard(args []string) {
	flags := cmd.BoardFlags{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vault", "-v":
			if i+1 < len(args) {
				i++
				flags.VaultPath = args[i]
			}
		case "--project", "-p":
			if i+1 < len(args) {
				i++
				flags.Project = args[i]
			}
		}
	}
	if err := cmd.RunBoard(flags); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func runInit(args []string) {
	flags := cmd.InitFlags{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				i++
				flags.Name = args[i]
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				i++
				flags.Dir = args[i]
			}
		case "--author", "-a":
			if i+1 < len(args) {
				i++
				flags.Author = args[i]
			}
		case "--force", "-f":
			flags.Force = true
		}
	}
	if err := cmd.RunInit(flags); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`otb — Obsidian Tasks Board

USAGE:
  otb [board] [OPTIONS]       Interactive kanban TUI (default)
  otb init [OPTIONS]          Scaffold a new vault

BOARD OPTIONS:
  --vault, -v <path>          Explicit vault path (auto-detected if omitted)
  --project, -p <name>        Pre-filter by project name

INIT OPTIONS:
  --name, -n <name>           Vault display name (default: my-vault)
  --dir, -d <path>            Target directory (default: ./<slug(name)>)
  --author, -a <handle>       Author handle in templates (default: user)
  --force, -f                 Overwrite existing files

OTHER:
  version                     Print version
  help                        Show this help

KEYBINDINGS (board):
  Tab / →           Next column
  Shift+Tab / ←     Previous column
  ↑ / ↓             Navigate tasks
  i / t / d / x     Move task to In Progress / Todo / Done / Cancelled
  c                 Add comment
  /                 Text filter
  p                 Project filter
  F                 Clear filters
  r                 Reload
  q / Esc           Quit
`)
}
