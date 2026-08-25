// slack-test: minimal Socket Mode listener for debugging Slack connectivity.
//
// Usage:
//   SLACK_APP_TOKEN=xapp-... SLACK_BOT_TOKEN=xoxb-... go run ./scripts/slack-test/
//
// What it does:
//   - Connects to Slack via Socket Mode (same path as the queen)
//   - Logs EVERY event with full payload as JSON so you can see exactly what
//     Slack sends, including connection_error reason
//   - Prints "MENTION: <text>" when @sb-slack (or any bot mention) arrives
//   - Runs until Ctrl-C
//
// This lets you verify token validity and event delivery without deploying Formicary.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	if appToken == "" {
		fmt.Fprintln(os.Stderr, "ERROR: SLACK_APP_TOKEN not set")
		os.Exit(1)
	}
	if botToken == "" {
		fmt.Fprintln(os.Stderr, "WARN: SLACK_BOT_TOKEN not set — inbound events only, no replies")
	}

	fmt.Printf("[%s] Connecting with AppToken=...%s BotToken=...%s\n",
		ts(), last4(appToken), last4(botToken))

	api := slackapi.New(botToken, slackapi.OptionAppLevelToken(appToken), slackapi.OptionDebug(false))
	client := socketmode.New(api)
	handler := socketmode.NewSocketmodeHandler(client)

	// Log every event — we want to see connection_error reason especially
	handler.Handle(socketmode.EventTypeConnecting, func(evt *socketmode.Event, c *socketmode.Client) {
		fmt.Printf("[%s] CONNECTING...\n", ts())
	})

	handler.Handle(socketmode.EventTypeConnected, func(evt *socketmode.Event, c *socketmode.Client) {
		fmt.Printf("[%s] ✓ CONNECTED — Socket Mode active, waiting for events\n", ts())
		fmt.Println("  → type @sb-slack help in Slack now")
	})

	handler.Handle(socketmode.EventTypeConnectionError, func(evt *socketmode.Event, c *socketmode.Client) {
		if ce, ok := evt.Data.(*slackapi.ConnectionErrorEvent); ok {
			errMsg := ""
			if ce.ErrorObj != nil {
				errMsg = ce.ErrorObj.Error()
			}
			fmt.Printf("[%s] ✗ CONNECTION_ERROR attempt=%d backoff=%s error=%q\n",
				ts(), ce.Attempt, ce.Backoff, errMsg)
		} else {
			raw, _ := json.Marshal(evt.Data)
			fmt.Printf("[%s] ✗ CONNECTION_ERROR data=%s\n", ts(), raw)
		}
	})

	handler.Handle(socketmode.EventTypeInvalidAuth, func(evt *socketmode.Event, c *socketmode.Client) {
		raw, _ := json.Marshal(evt.Data)
		fmt.Printf("[%s] ✗ INVALID_AUTH — token rejected by Slack: %s\n", ts(), raw)
		os.Exit(1)
	})

	handler.Handle(socketmode.EventTypeDisconnect, func(evt *socketmode.Event, c *socketmode.Client) {
		raw, _ := json.Marshal(evt.Data)
		fmt.Printf("[%s] DISCONNECT: %s\n", ts(), raw)
	})

	handler.Handle(socketmode.EventTypeHello, func(evt *socketmode.Event, c *socketmode.Client) {
		raw, _ := json.Marshal(evt.Data)
		fmt.Printf("[%s] HELLO: %s\n", ts(), raw)
	})

	handler.HandleEvents(slackevents.AppMention, func(evt *socketmode.Event, c *socketmode.Client) {
		c.Ack(*evt.Request)
		outer, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			fmt.Printf("[%s] APP_MENTION: unexpected data type %T\n", ts(), evt.Data)
			return
		}
		mention, ok := outer.InnerEvent.Data.(*slackevents.AppMentionEvent)
		if !ok {
			fmt.Printf("[%s] APP_MENTION: unexpected inner type %T\n", ts(), outer.InnerEvent.Data)
			return
		}
		fmt.Printf("[%s] *** APP_MENTION user=%s channel=%s text=%q ***\n",
			ts(), mention.User, mention.Channel, mention.Text)
	})

	handler.HandleEvents(slackevents.Message, func(evt *socketmode.Event, c *socketmode.Client) {
		c.Ack(*evt.Request)
		outer, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		if msg, ok := outer.InnerEvent.Data.(*slackevents.MessageEvent); ok {
			if msg.BotID == "" { // skip bot messages
				fmt.Printf("[%s] MESSAGE channel=%s user=%s text=%q\n",
					ts(), msg.Channel, msg.User, msg.Text)
			}
		}
	})

	// Catch-all: dump everything else as JSON so nothing is silently dropped
	handler.Default = func(evt *socketmode.Event, c *socketmode.Client) {
		raw, _ := json.Marshal(evt.Data)
		fmt.Printf("[%s] EVENT type=%s data=%s\n", ts(), evt.Type, truncate(string(raw), 300))
	}

	// Run event loop in background, exit on signal or fatal error
	errCh := make(chan error, 1)
	go func() { errCh <- handler.RunEventLoop() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		fmt.Printf("\n[%s] Received %s — exiting\n", ts(), sig)
	case err := <-errCh:
		fmt.Printf("[%s] Event loop exited: %v\n", ts(), err)
	}
}

func ts() string {
	return time.Now().Format("15:04:05")
}

func last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
