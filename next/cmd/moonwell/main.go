// Command moonwell is the Moonwell command line: a toolchain for Warcraft III maps with YueScript gameplay and Pkl
// data.
package main

import (
	"os"

	"github.com/mdlsvensson/moonwell/next/internal/cli"
)

func main() { os.Exit(cli.Main()) }
