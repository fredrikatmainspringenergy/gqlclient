package gqlclient_test

import (
	"context"
	"log"
	"strings"

	"git.sr.ht/~emersion/gqlclient"
)

func ExampleClient_Execute() {
	var ctx context.Context
	var c *gqlclient.Client

	op := gqlclient.NewOperation(`query {
		me {
			name
		}
	}`)

	var data struct {
		Me struct {
			Name string
		}
	}
	if err := c.Execute(ctx, op, &data); err != nil {
		log.Fatal(err)
	}

	log.Print(data)
}

func ExampleClient_Execute_vars() {
	var ctx context.Context
	var c *gqlclient.Client

	op := gqlclient.NewOperation(`query ($name: String!) {
		user(username: $name) {
			age
		}
	}`)

	op.Var("name", "emersion")

	var data struct {
		User struct {
			Age int
		}
	}
	if err := c.Execute(ctx, op, &data); err != nil {
		log.Fatal(err)
	}

	log.Print(data)
}

func ExampleClient_Execute_upload() {
	var ctx context.Context
	var c *gqlclient.Client

	op := gqlclient.NewOperation(`mutation ($file: Upload!) {
		send(file: $file)
	}`)

	op.Var("file", gqlclient.Upload{
		Filename: "gopher.txt",
		MIMEType: "text/plain",
		Body:     strings.NewReader("Hello, 世界"),
	})

	if err := c.Execute(ctx, op, nil); err != nil {
		log.Fatal(err)
	}
}
