package cmd

import (
	"context"
	"os"

	"plexobject.com/formicary/internal/buildversion"
	"plexobject.com/formicary/queen"
	"plexobject.com/formicary/queen/config"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// RunServer starts queen server for formicary
func RunServer(_ *cobra.Command, args []string) {
	log.WithFields(log.Fields{
		"Args": args,
		"ID":   id,
		"Port": port}).
		Infof("starting server...")
	serverConfig, err := config.NewServerConfig(id)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err}).
			Errorf("Failed to parse config...")
		os.Exit(5)
	}
	serverConfig.Common.Version = buildversion.New(Version, Commit, Date, id)
	if port > 0 {
		serverConfig.Common.HTTPPort = port
	}
	err = queen.Start(context.Background(), serverConfig)
	if err != nil {
		log.WithFields(log.Fields{"Error": err}).
			Errorf("failed to start server...")
		os.Exit(6)
	}
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "queen",
	Short: "Starts default server (queen) for processing the formicary jobs",
	Long:  "Starts default server (queen) for processing the formicary jobs",
	Run: func(cmd *cobra.Command, args []string) {
		RunServer(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.formicary.yaml)")
	serverCmd.Flags().StringVar(&id, "id", "", "id of formicary")
	serverCmd.Flags().IntVar(&port, "port", 0, "HTTP port to listen")
}
