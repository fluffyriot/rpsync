package common

import (
	"net/http"
	"time"
)

type Client struct {
	HTTPClient http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		HTTPClient: http.Client{
			Timeout: timeout,
		},
	}
}
