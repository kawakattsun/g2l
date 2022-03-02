package g2l

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	gmailUserID        = "me"
	gmailMessageFields = "messages/id"
	gmailMessageQuery  = "label:forward-to-line"
)

const lineMessageTemplate = "[From]\n%s\n[Subject]\n%s\n[Body]\n%s\n"

var errorNoMessageFound = errors.New("no message found")

// Handler g2l handler interface.
type Handler interface {
	Run(now time.Time, googleCredentials, googleToken []byte) error
}

type handler struct {
	gmail           *gmail.Service
	line            *linebot.Client
	forwardLineID   string
	intervalMinutes time.Duration
}

type message struct {
	from    string
	subject string
	body    string
}

// New new g2l instance.
func New(
	lineClient *linebot.Client,
	forwardLineID string,
	intervalMinutes time.Duration,
) Handler {
	return &handler{
		line:            lineClient,
		forwardLineID:   forwardLineID,
		intervalMinutes: intervalMinutes,
	}
}

// Run running g2l program.
func (h *handler) Run(now time.Time, googleCredentials, googleToken []byte) error {
	gmailClient, err := newGmailClient(googleCredentials, googleToken)
	if err != nil {
		return fmt.Errorf("unable to retrieve Gmail client: %w", err)
	}
	h.gmail = gmailClient
	border := now.Add(-h.intervalMinutes)
	msg, err := h.gmailMessagesByPeriod(border)
	if err != nil {
		if err == errorNoMessageFound {
			fmt.Println(errorNoMessageFound)
			return nil
		}
		return fmt.Errorf("error gmailMessagesByPeriod: %w", err)
	}
	if err := h.forwardToLine(msg); err != nil {
		return fmt.Errorf("error forwardToLine: %w", err)
	}
	return nil
}

func (h *handler) gmailMessagesByPeriod(border time.Time) ([]*message, error) {
	res, err := h.gmail.Users.Messages.
		List(gmailUserID).
		Fields(gmailMessageFields).
		Q(fmt.Sprintf("is:unread %s after:%d", gmailMessageQuery, border.Unix())).
		Do()

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve labels: %w", err)
	}
	if len(res.Messages) == 0 {
		return nil, errorNoMessageFound
	}

	var forwardMessages []*message
	for _, m := range res.Messages {
		fw, err := h.GmailMessage(m.Id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable get message: %v\n", err)
			continue
		}
		forwardMessages = append(forwardMessages, fw)
		if err := h.GmailRemoveUnread(m.Id); err != nil {
			fmt.Fprintf(os.Stderr, "failed modify message, remove unread: %v\n", err)
			continue
		}
	}

	return forwardMessages, nil
}

func (h *handler) GmailMessage(messageID string) (*message, error) {
	m, _ := h.gmail.Users.Messages.Get(gmailUserID, messageID).Do()
	msg := new(message)
	msg = setHeaders(msg, m.Payload.Headers)
	body, err := decodeBody(m.Payload)
	if err != nil {
		return nil, err
	}
	msg.body = body
	return msg, nil
}

func (h *handler) GmailRemoveUnread(messageID string) error {
	_, err := h.gmail.Users.Messages.
		Modify(
			gmailUserID,
			messageID,
			&gmail.ModifyMessageRequest{
				RemoveLabelIds: []string{"UNREAD"},
			},
		).
		Do()
	return err
}

func setHeaders(msg *message, headers []*gmail.MessagePartHeader) *message {
	for _, h := range headers {
		switch h.Name {
		case "From":
			msg.from = h.Value
		case "Subject":
			msg.subject = h.Value
		}
	}
	return msg
}

func decodeBody(payload *gmail.MessagePart) (string, error) {
	if payload.MimeType != "text/plain" {
		var body string
		for _, p := range payload.Parts {
			b, err := decodeBody(p)
			if err != nil {
				return "", err
			}
			body += b
		}
		return body, nil
	}

	b, err := base64.URLEncoding.DecodeString(payload.Body.Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}
	return string(b), nil
}

func (h *handler) forwardToLine(forwardMessages []*message) error {
	var lineMessages []linebot.SendingMessage
	for _, m := range forwardMessages {
		newMessage := fmt.Sprintf(lineMessageTemplate, m.from, m.subject, m.body)
		lineMessages = append(lineMessages, linebot.NewTextMessage(newMessage))
	}
	if _, err := h.line.PushMessage(h.forwardLineID, lineMessages...).Do(); err != nil {
		return err
	}
	return nil
}

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
	config, err := google.ConfigFromJSON(jsonBytes, gmail.GmailModifyScope)
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
