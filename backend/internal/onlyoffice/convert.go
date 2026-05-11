package onlyoffice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, h *http.Client) *Client {
	return &Client{baseURL: baseURL, http: h}
}

type convertRequest struct {
	Async      bool   `json:"async"`
	Filetype   string `json:"filetype"`
	Key        string `json:"key"`
	Outputtype string `json:"outputtype"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

type convertResponse struct {
	EndConvert bool   `json:"endConvert"`
	FileURL    string `json:"fileUrl"`
	Percent    int    `json:"percent"`
	Error      int    `json:"error"`
}

func (c *Client) Convert(ctx context.Context, key, filetype, outputtype, title, sourceURL string) (string, error) {
	body, err := json.Marshal(convertRequest{
		Async:      false,
		Filetype:   filetype,
		Key:        key,
		Outputtype: outputtype,
		Title:      title,
		URL:        sourceURL,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ConvertService.ashx", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out convertResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode convert response: %w", err)
	}
	if out.Error != 0 {
		return "", fmt.Errorf("onlyoffice convert error %d", out.Error)
	}
	if !out.EndConvert || out.FileURL == "" {
		return "", fmt.Errorf("onlyoffice convert incomplete")
	}
	return out.FileURL, nil
}
