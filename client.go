package gqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	endpoint string
	http     *http.Client
}

func New(endpoint string, hc *http.Client) *Client {
	return &Client{
		endpoint: endpoint,
		http:     hc,
	}
}

type Operation struct {
	query string
	vars  map[string]interface{}
}

func NewOperation(query string) *Operation {
	return &Operation{query: query}
}

func (op *Operation) Var(k string, v interface{}) {
	if op.vars == nil {
		op.vars = make(map[string]interface{})
	}
	op.vars[k] = v
}

func (c *Client) Execute(ctx context.Context, op *Operation, data interface{}) error {
	reqData := struct {
		Query string                 `json:"query"`
		Vars  map[string]interface{} `json:"variables"`
	}{
		Query: op.query,
		Vars:  op.vars,
	}

	var reqBuf bytes.Buffer
	if err := json.NewEncoder(&reqBuf).Encode(&reqData); err != nil {
		return fmt.Errorf("failed to encode request payload: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &reqBuf)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respData := struct {
		Data   interface{}
		Errors []Error
	}{Data: data}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to decode response payload: %v", err)
	}

	if len(respData.Errors) > 0 {
		return &respData.Errors[0]
	}
	return nil
}
