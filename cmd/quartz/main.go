package main

import (
	"github.com/MetroStar/quartzctl/internal/cmd"
	"github.com/MetroStar/quartzctl/internal/log"
)

var (
	version   = "dev"
	buildDate = ""
)

func main() {
	defer log.Sync() //nolint:errcheck

	cmd.RunAppService(cmd.AppServiceParams{
		Version:   version,
		BuildDate: buildDate,
	})
}
