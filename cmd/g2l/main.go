package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/kawakattsun/g2l"
	"github.com/line/line-bot-sdk-go/linebot"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var (
	version  string
	revision string
)

var (
	jst      = time.FixedZone("Asia/Tokyo", 9*60*60)
	isLambda = strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") ||
		os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
)

func getClient(ctx context.Context, config *oauth2.Config, f io.Reader) (*http.Client, error) {
	tok, err := tokenFromFile(f)
	if err != nil {
		return nil, err
	}
	return config.Client(ctx, tok), nil
}

func tokenFromFile(f io.Reader) (*oauth2.Token, error) {
	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

func newGmailClient(jsonBytes []byte, token []byte) (*gmail.Service, error) {
	config, err := google.ConfigFromJSON(jsonBytes, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}
	ctx := context.Background()
	client, err := getClient(ctx, config, bytes.NewReader(token))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve http client: %w", err)
	}
	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

func newLinebotClient(secret, token string) (*linebot.Client, error) {
	return linebot.New(secret, token)
}

func parseIntervalMinutes(s string) (time.Duration, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("unable to strconv.Atoi: %w", err)
	}
	return time.Duration(i * int(time.Minute)), nil
}

type parameters struct {
	googleToken            []byte
	googleCredentials      []byte
	lineChannelSecret      string
	lineChannelAccessToken string
	forwardLineID          string
	intervalMinutes        string
}

var (
	googleTokenName            = os.Getenv("GOOGLE_TOKEN")
	googleCredentialsName      = os.Getenv("GOOGLE_CREDENTIALS")
	lineChannelSecretName      = os.Getenv("LINE_CHANNEL_SECRET")
	lineChannelAccessTokenName = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	forwardLineIDName          = os.Getenv("FORWARD_LINE_ID")
	intervalMinutes            = os.Getenv("INTERVAL_MINUTES")
)

func initParameter() (*parameters, error) {
	p := new(parameters)
	if isLambda {
		ssmParams, err := loadSSMParameter(
			googleTokenName,
			googleCredentialsName,
			lineChannelSecretName,
			lineChannelAccessTokenName,
			forwardLineIDName,
		)
		if err != nil {
			return nil, err
		}
		for _, sp := range ssmParams {
			switch *sp.Name {
			case googleTokenName:
				p.googleToken = []byte(*sp.Value)
			case googleCredentialsName:
				p.googleCredentials = []byte(*sp.Value)
			case lineChannelSecretName:
				p.lineChannelSecret = *sp.Value
			case lineChannelAccessTokenName:
				p.lineChannelAccessToken = *sp.Value
			case forwardLineIDName:
				p.forwardLineID = *sp.Value
			}
		}
		p.intervalMinutes = intervalMinutes
		return p, nil
	}

	flag.Parse()
	googleCredentialsJSONPath := flag.Arg(0)
	googleTokenJSONPath := flag.Arg(1)
	p.lineChannelSecret = flag.Arg(2)
	p.lineChannelAccessToken = flag.Arg(3)
	p.forwardLineID = flag.Arg(4)
	p.intervalMinutes = flag.Arg(5)

	credentials, err := ioutil.ReadFile(googleCredentialsJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed googleCredentialsJSON open file: %w", err)
	}
	p.googleCredentials = credentials

	token, err := ioutil.ReadFile(googleTokenJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed googleTokenJSON open file: %w", err)
	}
	p.googleToken = token

	return p, nil
}

func loadSSMParameter(
	googleTokenName,
	googleCredentialsName,
	lineChannelSecretName,
	lineChannelAccessTokenName,
	forwardLineIDName string,
) ([]*ssm.Parameter, error) {
	svc := ssm.New(
		session.Must(session.NewSession()),
		aws.NewConfig().WithRegion(os.Getenv("AWS_REGION")),
	)
	input := &ssm.GetParametersInput{
		Names: []*string{
			&googleTokenName,
			&googleCredentialsName,
			&lineChannelSecretName,
			&lineChannelAccessTokenName,
			&forwardLineIDName,
		},
		WithDecryption: aws.Bool(true),
	}
	output, err := svc.GetParameters(input)
	if err != nil {
		err := fmt.Errorf("ssm GetParameters error occurred: %w", err)
		return nil, err
	}
	return output.Parameters, nil
}

func main() {
	p, err := initParameter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init parameter: %v\n", err)
		os.Exit(1)
	}

	gmailClient, err := newGmailClient(p.googleCredentials, p.googleToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve Gmail client: %v\n", err)
		os.Exit(1)
	}

	lineClient, err := newLinebotClient(p.lineChannelSecret, p.lineChannelAccessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve Linebot client: %v\n", err)
		os.Exit(1)
	}

	interval, err := parseIntervalMinutes(p.intervalMinutes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse intervalMinutes: %v\n", err)
		os.Exit(1)
	}

	handler := g2l.New(gmailClient, lineClient, p.forwardLineID, interval)
	if isLambda {
		lambda.Start(func(event events.CloudWatchEvent) error {
			if err := handler.Run(event.Time); err != nil {
				fmt.Fprintf(os.Stderr, "handler run error: %v\n", err)
			}
			return nil
		})
	} else {
		now := time.Now().In(jst).Truncate(60 * time.Second)
		if err := handler.Run(now); err != nil {
			fmt.Fprintf(os.Stderr, "handler run error: %v\n", err)
			os.Exit(1)
		}
	}
}
