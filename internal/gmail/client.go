package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"cocomail/internal/auth"
)

type Client struct {
	service *gmail.Service
	user    string
}

type Email struct {
	ID      string
	From    string
	To      string
	Subject string
	Date    time.Time
	Snippet string
	Body    string
	Labels  []string
	Unread  bool
}

func NewClient(ctx context.Context) (*Client, error) {
	a, err := auth.New()
	if err != nil {
		return nil, err
	}

	httpClient, err := a.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %v", err)
	}

	return &Client{
		service: srv,
		user:    "me",
	}, nil
}

func (c *Client) ListMessages(ctx context.Context, query string, maxResults int64) ([]Email, error) {
	req := c.service.Users.Messages.List(c.user).MaxResults(maxResults)
	if query != "" {
		req = req.Q(query)
	}

	resp, err := req.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list messages: %v", err)
	}

	emails := make([]Email, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		email, err := c.GetMessage(ctx, msg.Id)
		if err != nil {
			continue
		}
		emails = append(emails, *email)
	}

	return emails, nil
}

func (c *Client) GetMessage(ctx context.Context, id string) (*Email, error) {
	msg, err := c.service.Users.Messages.Get(c.user, id).Format("full").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get message: %v", err)
	}

	email := &Email{
		ID:      msg.Id,
		Snippet: msg.Snippet,
		Labels:  msg.LabelIds,
	}

	for _, label := range msg.LabelIds {
		if label == "UNREAD" {
			email.Unread = true
			break
		}
	}

	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "From":
			email.From = header.Value
		case "To":
			email.To = header.Value
		case "Subject":
			email.Subject = header.Value
		case "Date":
			if t, err := time.Parse(time.RFC1123Z, header.Value); err == nil {
				email.Date = t
			} else if t, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", header.Value); err == nil {
				email.Date = t
			}
		}
	}

	email.Body = extractBody(msg.Payload)

	return email, nil
}

func extractBody(payload *gmail.MessagePart) string {
	if payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}

	for _, part := range payload.Parts {
		if part.MimeType == "text/plain" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}

	for _, part := range payload.Parts {
		if part.MimeType == "text/html" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}

	for _, part := range payload.Parts {
		if strings.HasPrefix(part.MimeType, "multipart/") {
			if body := extractBody(part); body != "" {
				return body
			}
		}
	}

	return ""
}

func (c *Client) SendMessage(ctx context.Context, to, subject, body string) error {
	profile, err := c.service.Users.GetProfile(c.user).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to get user profile: %v", err)
	}

	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		profile.EmailAddress, to, subject, body)

	msg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(message)),
	}

	_, err = c.service.Users.Messages.Send(c.user, msg).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to send message: %v", err)
	}

	return nil
}

func (c *Client) MarkAsRead(ctx context.Context, id string) error {
	_, err := c.service.Users.Messages.Modify(c.user, id, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Context(ctx).Do()
	return err
}

func (c *Client) MarkAsUnread(ctx context.Context, id string) error {
	_, err := c.service.Users.Messages.Modify(c.user, id, &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"UNREAD"},
	}).Context(ctx).Do()
	return err
}

func (c *Client) TrashMessage(ctx context.Context, id string) error {
	_, err := c.service.Users.Messages.Trash(c.user, id).Context(ctx).Do()
	return err
}

func (c *Client) ListLabels(ctx context.Context) ([]string, error) {
	resp, err := c.service.Users.Labels.List(c.user).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	labels := make([]string, len(resp.Labels))
	for i, label := range resp.Labels {
		labels[i] = label.Name
	}
	return labels, nil
}
