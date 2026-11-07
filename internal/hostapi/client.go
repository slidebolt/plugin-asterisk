package hostapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrCursorExpired = errors.New("asterisk host event cursor expired")

type Client struct {
	base   string
	token  string
	http   *http.Client
	events *http.Client
}

func New(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("ASTERISK_HOST_URL is required")
	}
	if token == "" {
		return nil, errors.New("ASTERISK_HOST_TOKEN is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid Asterisk host URL: %w", err)
	}
	return &Client{
		base: baseURL, token: token,
		http:   &http.Client{Timeout: 15 * time.Second},
		events: &http.Client{},
	}, nil
}

func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := c.do(ctx, http.MethodGet, "/v1/snapshot", nil, &result)
	return result, err
}

func callPath(id string) string {
	return "/v1/calls/" + url.PathEscape(id)
}

func endpointPath(id string) string {
	return "/v1/endpoints/" + url.PathEscape(id)
}

// AddParticipant dials a target into a call, creating the call if needed.
func (c *Client) AddParticipant(ctx context.Context, call string, req AddParticipantRequest) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/participants", req, nil)
}

func (c *Client) RemoveParticipant(ctx context.Context, call, id string) error {
	return c.do(ctx, http.MethodDelete, callPath(call)+"/participants/"+url.PathEscape(id), nil, nil)
}

func (c *Client) ParticipantCommand(ctx context.Context, call, id string, req CommandRequest) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/participants/"+url.PathEscape(id)+"/commands", req, nil)
}

func (c *Client) EndCall(ctx context.Context, call string) error {
	return c.do(ctx, http.MethodDelete, callPath(call), nil, nil)
}

func (c *Client) Play(ctx context.Context, call string, req PlayRequest) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/playbacks", req, nil)
}

func (c *Client) StartCallStream(ctx context.Context, call string) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/streams", nil, nil)
}

func (c *Client) StopCallStream(ctx context.Context, call string) error {
	return c.do(ctx, http.MethodDelete, callPath(call)+"/streams", nil, nil)
}

func (c *Client) StartTranscription(ctx context.Context, call string) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/transcription", nil, nil)
}

func (c *Client) StopTranscription(ctx context.Context, call string) error {
	return c.do(ctx, http.MethodDelete, callPath(call)+"/transcription", nil, nil)
}

func (c *Client) Say(ctx context.Context, call string, req SayRequest) error {
	return c.do(ctx, http.MethodPost, callPath(call)+"/speech", req, nil)
}

func (c *Client) StartListen(ctx context.Context, endpoint string) error {
	return c.do(ctx, http.MethodPost, endpointPath(endpoint)+"/listen", nil, nil)
}

func (c *Client) StopListen(ctx context.Context, endpoint string) error {
	return c.do(ctx, http.MethodDelete, endpointPath(endpoint)+"/listen", nil, nil)
}

func (c *Client) HangupEndpoint(ctx context.Context, endpoint string) error {
	return c.do(ctx, http.MethodPost, endpointPath(endpoint)+"/hangup", nil, nil)
}

func (c *Client) Reload(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/server/reload", nil, nil)
}

func (c *Client) Events(ctx context.Context, streamID string, after int64, receive func(Event) error) error {
	path := fmt.Sprintf("/v1/events?streamId=%s&after=%d", url.QueryEscape(streamID), after)
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Last-Event-ID", fmt.Sprint(after))
	resp, err := c.events.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ErrCursorExpired
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() > 0 {
				var event Event
				if err := json.Unmarshal([]byte(data.String()), &event); err != nil {
					return fmt.Errorf("decode event: %w", err)
				}
				if err := receive(event); err != nil {
					return err
				}
				data.Reset()
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func (c *Client) do(ctx context.Context, method, path string, payload any, target any) error {
	req, err := c.request(ctx, method, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var problem struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(data, &problem)
	if problem.Detail == "" {
		problem.Detail = strings.TrimSpace(string(data))
	}
	return fmt.Errorf("asterisk host HTTP %d: %s", resp.StatusCode, problem.Detail)
}
