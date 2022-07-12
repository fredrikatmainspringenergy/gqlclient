package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"git.sr.ht/~emersion/gqlclient"
)

// The query used to determine type information
const query = `
query IntrospectionQuery {
  __schema {
    queryType {
      name
    }
    mutationType {
      name
    }
    subscriptionType {
      name
    }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type {
    ...TypeRef
  }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}
`

type stringSliceFlag []string

func (v *stringSliceFlag) String() string {
	return fmt.Sprint([]string(*v))
}

func (v *stringSliceFlag) Set(s string) error {
	*v = append(*v, s)
	return nil
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

func typeKeyword(t Type) string {
	switch t.Kind {
	case TypeKindObject:
		return "type"
	case TypeKindInterface:
		return "interface"
	default:
		panic("unreachable")
	}
}

func typeReference(t Type) string {
	var modifiers []TypeKind

	ofType := &t
	for ofType.OfType != nil {
		switch ofType.Kind {
		case TypeKindList, TypeKindNonNull:
			modifiers = append(modifiers, ofType.Kind)
		default:
			panic("invalid type")
		}
		ofType = ofType.OfType
	}

	if ofType.Name == nil {
		return "<invalid>"
	}
	typeName := *ofType.Name
	if len(modifiers) > 0 {
		for i := len(modifiers) - 1; i >= 0; i-- {
			switch modifiers[i] {
			case TypeKindList:
				typeName = "[" + typeName + "]"
			case TypeKindNonNull:
				typeName += "!"
			}
		}
	}
	return typeName
}

func printDescription(desc string, prefix string) {
	if !strings.Contains(desc, "\n") {
		fmt.Printf("%s\"%s\"\n", prefix, desc)
	} else {
		fmt.Printf("%s\"\"\"\n", prefix)
		for _, line := range strings.Split(desc, "\n") {
			fmt.Printf("%s%s\n", prefix, line)
		}
		fmt.Printf("%s\"\"\"\n", prefix)
	}
}

func printType(t Type) {
	if t.Description != nil && *t.Description != "" {
		printDescription(*t.Description, "")
	}
	switch t.Kind {
	case TypeKindScalar:
		fmt.Printf("scalar %s\n\n", *t.Name)
	case TypeKindUnion:
		fmt.Printf("union %s = ", *t.Name)
		for idx, i := range t.Interfaces {
			if idx > 0 {
				fmt.Print(" | ")
			}
			fmt.Printf("%s", typeReference(i))
		}
		fmt.Print("\n\n")
	case TypeKindEnum:
		fmt.Printf("enum %s {\n", *t.Name)
		for _, e := range t.EnumValues {
			if e.Description != nil && *e.Description != "" {
				printDescription(*e.Description, "\t")
			}
			fmt.Printf("	%s\n", e.Name)
		}
		fmt.Print("}\n\n")
	case TypeKindInputObject:
		fmt.Printf("input %s {\n", *t.Name)
		for _, f := range t.InputFields {
			if f.Description != nil && *f.Description != "" {
				printDescription(*f.Description, "\t")
			}
			fmt.Printf("	%s: %s", f.Name, typeReference(*f.Type))
			if f.DefaultValue != nil {
				fmt.Printf(" = %s", *f.DefaultValue)
			}
			fmt.Println()
		}
		fmt.Print("}\n\n")
	case TypeKindObject, TypeKindInterface:
		fmt.Printf("%s %s {\n", typeKeyword(t), *t.Name)
		for _, f := range t.Fields {
			if f.Description != nil && *f.Description != "" {
				printDescription(*f.Description, "\t")
			}
			fmt.Printf("	%s", f.Name)
			if len(f.Args) > 0 {
				fmt.Print("(")
				for idx, a := range f.Args {
					if idx > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s: %s", a.Name, typeReference(*a.Type))
					if a.DefaultValue != nil {
						fmt.Printf(" = %s", *a.DefaultValue)
					}
				}
				fmt.Print(")")
			}
			fmt.Printf(": %s\n", typeReference(*f.Type))
		}
		fmt.Print("}\n\n")
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var header []string
	flag.Var((*stringSliceFlag)(&header), "H", "set HTTP header")
	flag.Parse()

	endpoint := flag.Arg(0)
	if endpoint == "" {
		log.Fatalf("missing endpoint")
	}

	op := gqlclient.NewOperation(query)

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

	var data struct {
		Schema Schema `json:"__schema"`
	}
	if err := gqlClient.Execute(ctx, op, &data); err != nil {
		log.Fatal(err)
	}

	for _, t := range data.Schema.Types {
		if t.Name != nil && strings.HasPrefix(*t.Name, "__") {
			continue
		}
		printType(t)
	}
}
