package main

import (
	"plexobject.com/formicary/cmd"
	_ "plexobject.com/formicary/docs" // This line is necessary for go-swagger to find your docs!
)

var (
	version = "xdev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
