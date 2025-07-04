package cmd

import (
	"context"
	"os"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
	"strings"

	"plexobject.com/formicary/internal/buildversion"
	"plexobject.com/formicary/queen"
	"plexobject.com/formicary/queen/config"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	antTags    string
	antMethods string
)

// RunServer starts queen server for formicary
func RunServer(_ *cobra.Command, _ []string) {
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
	if serverConfig.HasEmbeddedAnt() {
		err = configureEmbeddedAnts(serverConfig)
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err}).
				Errorf("Failed to configure embedded ants...")
			os.Exit(6)
		}
	}

	err = queen.Start(context.Background(), serverConfig)
	if err != nil {
		log.WithFields(log.Fields{"Error": err}).
			Errorf("failed to start server...")
		os.Exit(7)
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

	// Embedded ants flags
	serverCmd.Flags().StringVar(&antTags, "ant-tags", "", "Comma-separated list of tags for embedded ants (default: embedded,default)")
	serverCmd.Flags().StringVar(&antMethods, "ant-methods", "", "Comma-separated list of methods for embedded ants (docker,k8s,shell,http_post,etc)")
}

// configureEmbeddedAnts configures embedded ants based on command line flags
func configureEmbeddedAnts(serverConfig *config.ServerConfig) error {
	if serverConfig.EmbeddedAnt == nil {
		serverConfig.EmbeddedAnt = &ant_config.AntConfig{}
	}
	// Parse tags
	if antTags != "" {
		tags := strings.Split(antTags, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
		if len(tags) > 0 {
			serverConfig.EmbeddedAnt.Tags = tags
		}
	}
	if len(serverConfig.EmbeddedAnt.Tags) == 0 {
		serverConfig.EmbeddedAnt.Tags = []string{"embedded", "default"}
	}

	// Parse methods
	var methods []types.TaskMethod
	if antMethods != "" {
		methodStrs := strings.Split(antMethods, ",")
		for _, methodStr := range methodStrs {
			methodStr = strings.TrimSpace(methodStr)
			switch strings.ToUpper(methodStr) {
			case "DOCKER":
				methods = append(methods, types.Docker)
			case "KUBERNETES", "K8S":
				methods = append(methods, types.Kubernetes)
			case "HTTP_GET":
				methods = append(methods, types.HTTPPostJSON)
			case "HTTP_POST":
				methods = append(methods, types.HTTPPostJSON)
			case "HTTP_PUT":
				methods = append(methods, types.HTTPPutJSON)
			case "HTTP_DELETE":
				methods = append(methods, types.HTTPDelete)
			case "SHELL":
				methods = append(methods, types.Shell)
			case "WEBSOCKET":
				methods = append(methods, types.WebSocket)
			default:
				log.WithFields(log.Fields{
					"Method": methodStr,
				}).Warn("Unknown method, skipping")
			}
		}
	}
	if len(methods) > 0 {
		serverConfig.EmbeddedAnt.Methods = methods
	}
	if len(serverConfig.EmbeddedAnt.Methods) == 0 {
		// Default methods
		serverConfig.EmbeddedAnt.Methods = []types.TaskMethod{types.Kubernetes, types.Shell, types.HTTPPostJSON}
	}

	return nil
}
