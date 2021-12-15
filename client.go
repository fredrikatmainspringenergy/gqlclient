package gqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	query   string
	vars    map[string]interface{}
	uploads map[string]Upload
}

func NewOperation(query string) *Operation {
	return &Operation{query: query}
}

func (op *Operation) Var(k string, v interface{}) {
	if op.vars == nil {
		op.vars = make(map[string]interface{})
		op.uploads = make(map[string]Upload)
	}
	if _, ok := op.vars[k]; ok {
		panic(fmt.Sprintf("gqlclient: called Operation.Var twice on %q", k))
	}
	op.vars[k] = v

	// TODO: support more deeply nested uploads
	switch v := v.(type) {
	case Upload:
		op.uploads[k] = v
	case *Upload:
		if v != nil {
			op.uploads[k] = *v
		}
	case []Upload:
		for i, upload := range v {
			upload := upload // copy
			op.uploads[fmt.Sprintf("%v.%v", k, i)] = upload
		}
	case []*Upload:
		for i, upload := range v {
			if upload != nil {
				op.uploads[fmt.Sprintf("%v.%v", k, i)] = *upload
			}
		}
	}
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

	var reqBody io.Reader
	var contentType string
	if len(op.uploads) > 0 {
		pr, pw := io.Pipe()
		defer pr.Close()

		reqBody = pr
		contentType = writeMultipart(pw, op.uploads, &reqBuf)
	} else {
		reqBody = &reqBuf
		contentType = "application/json; charset=utf-8"
	}

	// io.TeeReader(reqBody, os.Stderr)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
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
	// io.TeeReader(resp.Body, os.Stderr)
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to decode response payload: %v", err)
	}

	if len(respData.Errors) > 0 {
		return &respData.Errors[0]
	}
	return nil
}
