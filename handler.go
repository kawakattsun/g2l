package g2l

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
	"google.golang.org/api/gmail/v1"
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
	Run(time.Time) error
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
	gmailClient *gmail.Service,
	lineClient *linebot.Client,
	forwardLineID string,
	intervalMinutes time.Duration,
) Handler {
	return &handler{
		gmail:           gmailClient,
		line:            lineClient,
		forwardLineID:   forwardLineID,
		intervalMinutes: intervalMinutes,
	}
}

// Run running g2l program.
func (h *handler) Run(now time.Time) error {
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
		Q(fmt.Sprintf("%s after:%d", gmailMessageQuery, border.Unix())).
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
