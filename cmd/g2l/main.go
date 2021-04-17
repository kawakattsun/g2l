package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kawakattsun/g2l"
	"github.com/line/line-bot-sdk-go/linebot"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

var (
	version  string
	revision string
)

var (
	googleCredentialsJSONPath = os.Args[1]
	googleTokenJSONPath       = os.Args[2]
	lineChannelSecret         = os.Args[3]
	lineChannelToken          = os.Args[4]
	forwardLineID             = os.Args[5]
	intervalSecond            = os.Args[6]
)

func getClient(config *oauth2.Config) (*http.Client, error) {
	tok, err := tokenFromFile(googleTokenJSONPath)
	if err != nil {
		return nil, err
	}
	return config.Client(context.Background(), tok), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func newGmailClient() (*gmail.Service, error) {
	b, err := ioutil.ReadFile(googleCredentialsJSONPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to read client secret file: %w", err)
	}
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %w", err)
	}
	client, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve http client: %w", err)
	}
	return gmail.New(client)
}

func newLinebotClient() (*linebot.Client, error) {
	return linebot.New(lineChannelSecret, lineChannelToken)
}

func parseIntervalSecond() (time.Duration, error) {
	i, err := strconv.Atoi(intervalSecond)
	if err != nil {
		return 0, fmt.Errorf("Unable to strconv.Atoi: %w", err)
	}

	return time.Duration(i * int(time.Second)), nil
}

func main() {
	gmailClient, err := newGmailClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve Gmail client: %v", err)
		os.Exit(1)
	}
	lineClient, err := newLinebotClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve Linebot client: %v", err)
		os.Exit(1)
	}

	i, err := parseIntervalSecond()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse intervalSecond: %v", err)
		os.Exit(1)
	}

	handler := g2l.New(gmailClient, lineClient, forwardLineID, i)
	if err := handler.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve Gmail client: %v\n", err)
		os.Exit(1)
	}
}
