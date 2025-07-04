package main

import (
	"context"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/sample/golang"
	"strings"
	"sync"
)

var cfgFile string
var requestTopic string
var responseTopic string
var id string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "messaging-ant",
	Short: "Starts messaging ant worker",
	Long:  "Starts messaging ant worker",
	Run: func(cmd *cobra.Command, args []string) {
		if err := startAntWorker(cmd, args); err != nil {
			logrus.WithFields(logrus.Fields{"Error": err}).
				Errorf("failed to execute messaging ant worker!")
			os.Exit(1)
		}
		logrus.WithFields(logrus.Fields{
			"ID":           id,
			"RequestTopic": requestTopic,
		}).
			Infof("started messaging ant worker!")
		var wg sync.WaitGroup
		wg.Add(1)
		wg.Wait()
	},
}

func startAntWorker(_ *cobra.Command, _ []string) error {
	serverConfig, err := config.NewServerConfig(id)
	if err != nil {
		return err
	}
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverConfig.Common)
	if err != nil {
		return err
	}
	ant := golang.NewMessagingHandler(
		&serverConfig.Common,
		id,
		requestTopic,
		responseTopic,
		queueClient)
	return ant.Start(context.Background())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logrus.WithFields(logrus.Fields{"Error": err}).
			Errorf("failed to execute messaging ant worker!")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.formicary.yaml)")

	rootCmd.Flags().StringVar(&requestTopic, "requestTopic", "", "request topic")
	rootCmd.Flags().StringVar(&responseTopic, "responseTopic", "", "response topic")
	rootCmd.Flags().StringVar(&id, "id", "", "id of messaging ant-worker")

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"ConfigFile": cfgFile,
			}).Debugf("specifying default config file...")
		}
	} else {
		viper.AddConfigPath(".")
		// Find home directory.
		if home, err := homedir.Dir(); err == nil {
			// Search config in home directory with name ".formicary" (without extension).
			viper.AddConfigPath(home)
		}

		viper.SetConfigName(".formicary")
		viper.SetConfigType("yaml")
		viper.SetEnvPrefix("")
		viper.AutomaticEnv()
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	}
}
