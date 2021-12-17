package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~emersion/gqlclient"
)

type stringSliceFlag []string

func (v *stringSliceFlag) String() string {
	return fmt.Sprint([]string(*v))
}

func (v *stringSliceFlag) Set(s string) error {
	*v = append(*v, s)
	return nil
}

func splitKeyValue(kv string) (string, string) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		log.Fatalf("in variable definition %q: missing equal sign", kv)
	}
	return parts[0], parts[1]
}

type transport struct {
	http.RoundTripper

	header http.Header
}

func (tr *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, values := range tr.header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	return tr.RoundTripper.RoundTrip(req)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var rawVars, jsonVars, fileVars, header []string
	flag.Var((*stringSliceFlag)(&rawVars), "v", "set raw variable")
	flag.Var((*stringSliceFlag)(&jsonVars), "j", "set JSON variable")
	flag.Var((*stringSliceFlag)(&fileVars), "f", "set file variable")
	flag.Var((*stringSliceFlag)(&header), "H", "set HTTP header")
	flag.Parse()

	endpoint := flag.Arg(0)
	if endpoint == "" {
		log.Fatalf("missing endpoint")
	}

	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("failed to read GraphQL query from stdin: %v", err)
	}
	query := string(b)

	op := gqlclient.NewOperation(query)
	for _, kv := range rawVars {
		k, v := splitKeyValue(kv)
		op.Var(k, v)
	}
	for _, kv := range jsonVars {
		k, raw := splitKeyValue(kv)
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			log.Fatalf("in variable definition %q: invalid JSON: %v", kv, err)
		}
		op.Var(k, json.RawMessage(raw))
	}
	for _, kv := range fileVars {
		k, filename := splitKeyValue(kv)

		f, err := os.Open(filename)
		if err != nil {
			log.Fatalf("in variable definition %q: failed to open input file: %v", kv, err)
		}
		defer f.Close()

		t := mime.TypeByExtension(filename)
		if t == "" {
			t = "application/octet-stream"
		}

		op.Var(k, gqlclient.Upload{
			Filename: filepath.Base(filename),
			MIMEType: t,
			Body:     f,
		})
	}

	tr := transport{
		RoundTripper: http.DefaultTransport,
		header:       make(http.Header),
	}
	httpClient := http.Client{Transport: &tr}
	gqlClient := gqlclient.New(endpoint, &httpClient)

	for _, kv := range header {
		parts := strings.SplitN(kv, ":", 2)
		if len(parts) != 2 {
			log.Fatalf("in header definition %q: missing colon", kv)
		}
		tr.header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	var data json.RawMessage
	if err := gqlClient.Execute(ctx, op, &data); err != nil {
		log.Fatal(err)
	}

	r := bytes.NewReader([]byte(data))
	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatalf("failed to write response: %v", err)
	}
}
