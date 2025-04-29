package main

import (
	"github.com/MetroStar/quartzctl/internal/cmd"
	"github.com/MetroStar/quartzctl/internal/log"
)

var (
	version   = "0.1.0"
	buildDate = ""
)

func main() {
	defer log.Sync() //nolint:errcheck

	cmd.RunAppService(cmd.AppServiceParams{
		Version:   version,
		BuildDate: buildDate,
	})
}
