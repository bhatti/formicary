package cmd

import (
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var id string
var port int

// Version of the queen server
var Version string

// Commit of the last change
var Commit string

// Date of the build
var Date string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "formicary",
	Short: "Starts default server (queen) for processing the formicary jobs",
	Long:  "Starts default server (queen) for processing the formicary jobs",
	Run: func(cmd *cobra.Command, args []string) {
		RunServer(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string, commit string, date string) {
	Version = version
	Commit = commit
	Date = date
	if err := rootCmd.Execute(); err != nil {
		log.WithFields(log.Fields{"Error": err}).
			Errorf("failed to execute command...")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.formicary.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringVar(&id, "id", "", "id of formicary")
	rootCmd.Flags().IntVar(&port, "port", 0, "HTTP port to listen")

	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(log.Fields{
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
