// Command snagarr runs the server and provides its command-line client.
package main

import (
	"os"

	"github.com/sirrobot01/snagarr/internal/cli"
)

func main() {
	command := cli.New()
	if err := command.Execute(); err != nil {
		cli.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
