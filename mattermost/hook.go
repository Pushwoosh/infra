package inframm

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
)

// Hook is a helper for making bots for Mattermost

type Hook struct {
	address  string
	secret   string
	channel  string
	username string
	icon     string
}

// NewHook creates a new Hook
// address - mattermost server address
// secret - mattermost hook secret
// channel - channel name
// username - bot username
// icon - bot icon
func NewHook(address, secret, channel, username, icon string) *Hook {
	return &Hook{
		address:  address,
		secret:   secret,
		channel:  channel,
		username: username,
		icon:     icon,
	}
}

func (h *Hook) Send(message string) error {
	type Request struct {
		Channel  string `json:"channel"`
		Username string `json:"username"`
		IconUrl  string `json:"icon_url"`
		Text     string `json:"text"`
	}

	request := Request{
		Channel:  h.channel,
		Username: h.username,
		IconUrl:  h.icon,
		Text:     message,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "json.Marshal")
	}

	postURL := "https://" + h.address + "/hooks/" + h.secret
	r, err := http.NewRequest("POST", postURL, bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrap(err, "unable to create POST request to Mattermost channel")
	}

	r.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return errors.Wrap(err, "unable to send POST request to Mattermost channel")
	}
	defer func() { _ = res.Body.Close() }()

	code := res.StatusCode
	if code != http.StatusOK {
		return errors.Errorf("mattermost server returned: %s", res.Status)
	}

	return nil
}
