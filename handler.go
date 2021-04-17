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
	Run() error
}

type handler struct {
	gmail          *gmail.Service
	line           *linebot.Client
	forwardLineID  string
	intervalSecond time.Duration
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
	intervalSecond time.Duration,
) Handler {
	return &handler{
		gmail:          gmailClient,
		line:           lineClient,
		forwardLineID:  forwardLineID,
		intervalSecond: intervalSecond,
	}
}

// Run running g2l program.
func (h *handler) Run() error {
	now := time.Now()
	border := now.Add(-h.intervalSecond)
	msg, err := h.gmailMessagesByPeriod(border)
	if err != nil {
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
		Q(fmt.Sprintf("%s after:%s", gmailMessageQuery, border.Format(time.RFC3339))).
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
	for _, h := range m.Payload.Headers {
		switch h.Name {
		case "From":
			msg.from = h.Value
		case "Subject":
			msg.subject = h.Value
		}
	}
	data, err := base64.URLEncoding.DecodeString(m.Payload.Body.Data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}
	msg.body = string(data)
	return msg, nil
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
