# gqlclient

[![godocs.io](https://godocs.io/git.sr.ht/~emersion/gqlclient?status.svg)](https://godocs.io/git.sr.ht/~emersion/gqlclient)

A GraphQL client and code generator for Go.

## Usage

gqlclient can be used as a thin GraphQL client, and can be augmented with code
generation. See the GoDoc examples for direct usage.

### GraphQL schema code generation

The code generator can parse a GraphQL schema and generate Go types. For
instance, the following schema:

```graphqls
type Train {
	name: String!
	maxSpeed: Int!
	weight: Int!
	linesServed: [String!]!
}
```

and the following `gqlclientgen` invocation:

```sh
gqlclientgen -s rail.graphqls -o rail.go -n rail
```

will generate the following Go type:

```go
type Train struct {
	Name string
	MaxSpeed int32
	Weight int32
	LinesServed []string
}
```

which can then be used in a GraphQL query:

```go
op := gqlclient.NewOperation(`query {
	train(name: "Shinkansen E5") {
		name
		maxSpeed
		linesServed
	}
}`)

var data struct {
	Train rail.Train
}
if err := c.Execute(ctx, op, &data); err != nil {
	log.Fatal(err)
}
```

## License

MIT
