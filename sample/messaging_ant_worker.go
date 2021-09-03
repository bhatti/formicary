package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"plexobject.com/formicary/queen/config"
	"strings"
	"sync"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

// MessagingHandler structure
type MessagingHandler struct {
	id           string
	requestTopic string
	queueClient  queue.Client
}

// NewMessagingHandler constructor
func NewMessagingHandler(
	id string,
	requestTopic string,
	queueClient queue.Client,
) *MessagingHandler {
	return &MessagingHandler{
		id:           id,
		requestTopic: requestTopic,
		queueClient:  queueClient,
	}
}

// Start starts subscription
func (rh *MessagingHandler) Start(
	ctx context.Context,
) (err error) {
	if rh.id == "" {
		return fmt.Errorf("id is not specified")
	}
	if rh.requestTopic == "" {
		return fmt.Errorf("requestTopic is not specified")
	}
	return rh.queueClient.Subscribe(
		ctx,
		rh.requestTopic,
		rh.id,
		make(map[string]string),
		true, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			err = rh.execute(ctx, event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "MessagingHandler",
					"Payload":   string(event.Payload),
					"Target":    rh.id,
					"Error":     err}).Error("failed to execute")
				return err
			}
			return nil
		},
	)
}

// Stop stops subscription
func (rh *MessagingHandler) Stop(
	ctx context.Context,
) (err error) {
	return rh.queueClient.UnSubscribe(
		ctx,
		rh.requestTopic,
		rh.id,
	)
}

// execute request
func (rh *MessagingHandler) execute(
	ctx context.Context,
	reqPayload []byte) (err error) {
	var req types.TaskRequest
	err = json.Unmarshal(reqPayload, &req)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"ID":           id,
		"RequestTopic": requestTopic,
		"Request":      req.String(),
	}).
		Infof("received messaging request")
	resp := types.NewTaskResponse(&req)
	epoch := time.Now().Unix()
	if epoch%2 == 0 {
		resp.Status = types.COMPLETED
	} else {
		resp.ErrorCode = "ERR_MESSAGING_WORKER"
		resp.ErrorMessage = "mock error for messaging client"
		resp.Status = types.FAILED
	}
	resp.AddContext("epoch", epoch)
	resPayload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = rh.queueClient.Send(
		ctx,
		req.ResponseTopic,
		make(map[string]string),
		resPayload,
		false)
	logrus.WithFields(logrus.Fields{
		"ID":            id,
		"RequestTopic":  requestTopic,
		"ResponseTopic": req.ResponseTopic,
		"Status":        resp.Status,
	}).
		Infof("sent reply")
	return err
}

var cfgFile string
var requestTopic string
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
	queueClient, err := queue.NewMessagingClient(&serverConfig.CommonConfig)
	if err != nil {
		return err
	}
	ant := NewMessagingHandler(
		id,
		requestTopic,
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
