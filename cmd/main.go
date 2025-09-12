package main

import (
	"os"

	"github.com/flowbaker/flowbaker/cmd/cli"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cli.Execute()
}
