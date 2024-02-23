package cmd

import (
	"context"
	"os"
	"plexobject.com/formicary/internal/buildversion"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/ants"
	"plexobject.com/formicary/ants/config"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var tags string

// antCmd represents the ant command
var antCmd = &cobra.Command{
	Use:   "ant",
	Short: "starts a formicary ant",
	Long:  "starts a formicary ant for executing tasks using docker/kubernetes/APIs",
	Run: func(cmd *cobra.Command, args []string) {
		log.WithFields(log.Fields{
			"Args": args,
			"ID":   id,
			"Port": port}).
			Infof("starting formicary ant ...")
		antCfg, err := config.NewAntConfig(id)
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err}).
				Errorf("failed to parse config...")
			os.Exit(5)
		}
		antCfg.Common.Version = buildversion.New(Version, Commit, Date, id)
		if port > 0 {
			antCfg.Common.HTTPPort = port
		}
		for _, next := range utils.SplitTags(tags) {
			antCfg.Tags = append(antCfg.Tags, next)
		}
		if err = antCfg.Validate(); err != nil {
			log.WithFields(log.Fields{
				"Error": err}).
				Errorf("Failed to validate config...")
			os.Exit(88)
		}
		err = ants.Start(context.Background(), antCfg)
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err}).
				Errorf("failed to start ant ...")
			os.Exit(99)
		}
	},
}

func init() {
	rootCmd.AddCommand(antCmd)

	antCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.formicary.yaml)")
	antCmd.Flags().StringVar(&id, "id", "", "id of formicary")
	antCmd.Flags().IntVar(&port, "port", 0, "HTTP port to listen")
	antCmd.Flags().StringVar(&tags, "tags", "", "tags of ant")
}
