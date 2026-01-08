package main

import (
	"github.com/baggiiiie/configlock/cmd"
)

// Version can be set at build time using -ldflags="-X 'main.Version=$VERSION'"
var Version = ""

func main() {
	cmd.SetVersion(Version)
	cmd.Execute()
}
