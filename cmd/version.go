package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"plexobject.com/formicary/internal/buildversion"
)

var (
	shortened  = false
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Version will output the current build information",
		Long:  ``,
		Run: func(_ *cobra.Command, _ []string) {
			versionOutput := buildversion.New(Version, Commit, Date, id)

			if shortened {
				fmt.Printf("%+v", versionOutput.ToShortened())
			} else {
				fmt.Printf("%+v", versionOutput.ToJSON())
			}
		},
	}
)

func init() {
	versionCmd.Flags().BoolVarP(&shortened, "short", "s", false, "Print just the version number.")
	rootCmd.AddCommand(versionCmd)
}
