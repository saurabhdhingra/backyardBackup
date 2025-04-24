package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yourusername/backyardBackup/internal/backup"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	// Success notification
	Success NotificationType = "success"
	// Failure notification
	Failure NotificationType = "failure"
	// Warning notification
	Warning NotificationType = "warning"
	// Info notification
	Info NotificationType = "info"
)

// NotificationEvent represents an event that should trigger a notification
type NotificationEvent struct {
	Type        NotificationType
	Title       string
	Message     string
	BackupInfo  *backup.BackupResult
	OccurredAt  time.Time
}

// SlackNotifier implements Slack webhook notifications
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// slackPayload is the JSON payload for Slack webhook
type slackPayload struct {
	Text        string `json:"text"`
	Username    string `json:"username,omitempty"`
	IconEmoji   string `json:"icon_emoji,omitempty"`
	IconURL     string `json:"icon_url,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

// slackAttachment is a Slack message attachment
type slackAttachment struct {
	Fallback   string            `json:"fallback"`
	Color      string            `json:"color"`
	Pretext    string            `json:"pretext,omitempty"`
	Title      string            `json:"title"`
	TitleLink  string            `json:"title_link,omitempty"`
	Text       string            `json:"text"`
	Fields     []slackField      `json:"fields,omitempty"`
	Footer     string            `json:"footer,omitempty"`
	FooterIcon string            `json:"footer_icon,omitempty"`
	Timestamp  int64             `json:"ts,omitempty"`
}

// slackField is a field in a Slack attachment
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Notify sends a notification to Slack
func (n *SlackNotifier) Notify(ctx context.Context, event NotificationEvent) error {
	if n.webhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}

	return fmt.Errorf("slack notification not implemented yet")
}

// buildSlackPayload builds the Slack webhook payload
func (n *SlackNotifier) buildSlackPayload(event NotificationEvent) (*slackPayload, error) {
	// Choose color based on notification type
	var color string
	switch event.Type {
	case Success:
		color = "good" // green
	case Failure:
		color = "danger" // red
	case Warning:
		color = "warning" // yellow
	case Info:
		color = "#439FE0" // blue
	default:
		color = "#cccccc" // grey
	}

	// Create attachment fields
	fields := []slackField{}

	// Add backup info if available
	if event.BackupInfo != nil {
		fields = append(fields, slackField{
			Title: "Backup ID",
			Value: event.BackupInfo.ID,
			Short: true,
		})
		fields = append(fields, slackField{
			Title: "Type",
			Value: string(event.BackupInfo.Type),
			Short: true,
		})
		fields = append(fields, slackField{
			Title: "Size",
			Value: fmt.Sprintf("%d bytes", event.BackupInfo.Size),
			Short: true,
		})
		fields = append(fields, slackField{
			Title: "Duration",
			Value: event.BackupInfo.EndTime.Sub(event.BackupInfo.StartTime).String(),
			Short: true,
		})
	}

	payload := &slackPayload{
		Username:  "BackyardBackup",
		IconEmoji: ":floppy_disk:",
		Attachments: []slackAttachment{
			{
				Fallback:  event.Title,
				Color:     color,
				Title:     event.Title,
				Text:      event.Message,
				Fields:    fields,
				Footer:    "BackyardBackup Notification",
				Timestamp: event.OccurredAt.Unix(),
			},
		},
	}

	return payload, nil
} 