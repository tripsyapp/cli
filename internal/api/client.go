package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.tripsy.app"

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Response struct {
	StatusCode int
	Header     http.Header
	Data       any
	Raw        []byte
}

type Error struct {
	StatusCode int
	Data       any
	Raw        []byte
}

func (e *Error) Error() string {
	body := strings.TrimSpace(string(e.Raw))
	if body == "" {
		return fmt.Sprintf("Tripsy API returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("Tripsy API returned HTTP %d: %s", e.StatusCode, body)
}

func NewClient(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	target, err := c.url(path, query)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			reader = bytes.NewReader(v)
		case string:
			reader = strings.NewReader(v)
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			reader = bytes.NewReader(encoded)
		}
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), target, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Token "+strings.TrimSpace(c.Token))
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed := parseBody(resp.Header.Get("Content-Type"), raw)
	result := &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Data:       parsed,
		Raw:        raw,
	}
	if resp.StatusCode >= 400 {
		return result, &Error{StatusCode: resp.StatusCode, Data: parsed, Raw: raw}
	}
	return result, nil
}

func (c *Client) UploadFile(ctx context.Context, uploadURL string, headers map[string]string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, file)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &Error{StatusCode: resp.StatusCode, Raw: raw}
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) url(path string, query url.Values) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil {
			return "", err
		}
		addQuery(u, query)
		return u.String(), nil
	}

	u, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return "", err
	}
	addQuery(u, query)
	return u.String(), nil
}

func addQuery(u *url.URL, query url.Values) {
	if len(query) == 0 {
		return
	}
	values := u.Query()
	for key, items := range query {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	u.RawQuery = values.Encode()
}

func parseBody(contentType string, raw []byte) any {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}

	trimmed := bytes.TrimSpace(raw)
	if strings.Contains(contentType, "application/json") || bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		var data any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&data); err == nil {
			return data
		}
	}
	return string(raw)
}
