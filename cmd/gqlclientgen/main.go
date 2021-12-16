package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
)

const usage = `usage: gqlclientgen -s <schema> -o <output> [options...]

Generate Go types and helpers for the specified GraphQL schema.

Options:

  -s <schema>   GraphQL schema, can be specified multiple times. Required.
  -q <query>    GraphQL query document, can be specified multiple times.
  -o <output>   Output filename for generated Go code. Required.
  -n <package>  Go package name, defaults to "main".
`

type stringSliceFlag []string

func (v *stringSliceFlag) String() string {
	return fmt.Sprint([]string(*v))
}

func (v *stringSliceFlag) Set(s string) error {
	*v = append(*v, s)
	return nil
}

const gqlclient = "git.sr.ht/~emersion/gqlclient"

func genType(schema *ast.Schema, t *ast.Type) *jen.Statement {
	if t.Elem != nil {
		return jen.Index().Add(genType(schema, t.Elem))
	}

	def := schema.Types[t.NamedType]

	if !t.NonNull {
		switch def.Name {
		case "ID", "Time", "Map", "Any":
			// These don't need a pointer
		default:
			cpy := *t
			cpy.NonNull = true
			return jen.Op("*").Add(genType(schema, &cpy))
		}
	}

	switch def.Name {
	// Standard types
	case "Int":
		return jen.Int32()
	case "Float":
		return jen.Float64()
	case "String":
		return jen.String()
	case "Boolean":
		return jen.Bool()
	case "ID":
		return jen.String()
	// Convenience types
	case "Time":
		return jen.Qual("time", "Time")
	case "Map":
		return jen.Map(jen.String()).Interface()
	case "Upload":
		return jen.Qual(gqlclient, "Upload")
	case "Any":
		return jen.Interface()
	default:
		if def.BuiltIn {
			panic(fmt.Sprintf("unsupported built-in type: %s", def.Name))
		}
		return jen.Id(def.Name)
	}
}

func genDef(schema *ast.Schema, def *ast.Definition) *jen.Statement {
	switch def.Kind {
	case ast.Scalar:
		switch def.Name {
		case "Time", "Map", "Upload", "Any":
			// Convenience types
			return nil
		default:
			return jen.Type().Id(def.Name).String()
		}
	case ast.Enum:
		var defs []jen.Code
		for _, val := range def.EnumValues {
			nameWords := strings.Split(strings.ToLower(val.Name), "_")
			for i := range nameWords {
				nameWords[i] = strings.Title(nameWords[i])
			}
			name := strings.Join(nameWords, "")
			defs = append(defs, jen.Id(def.Name+name).Id(def.Name).Op("=").Lit(val.Name))
		}
		return jen.Add(
			jen.Type().Id(def.Name).String(),
			jen.Line(),
			jen.Const().Defs(defs...),
		)
	case ast.Object, ast.Interface, ast.InputObject:
		var fields []jen.Code
		for _, field := range def.Fields {
			if field.Name == "__schema" || field.Name == "__type" {
				continue // TODO
			}
			name := strings.Title(field.Name)
			jsonTag := field.Name
			if !field.Type.NonNull {
				jsonTag += ",omitempty"
			}
			tag := jen.Tag(map[string]string{"json": jsonTag})
			fields = append(fields, jen.Id(name).Add(genType(schema, field.Type)).Add(tag))
		}
		return jen.Type().Id(def.Name).Struct(fields...)
	default:
		panic(fmt.Sprintf("unsupported definition kind: %s", def.Kind))
	}
}

func genOp(schema *ast.Schema, op *ast.OperationDefinition) *jen.Statement {
	var query ast.QueryDocument
	query.Operations = ast.OperationList{op}
	var sb strings.Builder
	formatter.NewFormatter(&sb).FormatQueryDocument(&query)
	queryStr := sb.String()

	var stmts, in, out, ret, dataFields []jen.Code

	in = append(in, jen.Id("client").Op("*").Qual(gqlclient, "Client"))
	in = append(in, jen.Id("ctx").Qual("context", "Context"))

	stmts = append(stmts, jen.Id("op").Op(":=").Qual(gqlclient, "NewOperation").Call(jen.Lit(queryStr)))

	for _, v := range op.VariableDefinitions {
		in = append(in, jen.Id(v.Variable).Add(genType(schema, v.Type)))
		stmts = append(stmts, jen.Id("op").Dot("Var").Call(
			jen.Lit(v.Variable),
			jen.Id(v.Variable),
		))
	}

	for _, sel := range op.SelectionSet {
		field, ok := sel.(*ast.Field)
		if !ok {
			panic(fmt.Sprintf("unsupported selection %T", sel))
		}
		if field.Name != field.Alias {
			panic(fmt.Sprintf("field aliases aren't supported"))
		}
		typ := genType(schema, field.Definition.Type)
		out = append(out, jen.Id(field.Name).Add(typ))
		ret = append(ret, jen.Id("respData").Dot(strings.Title(field.Name)))
		dataFields = append(dataFields, jen.Id(strings.Title(field.Name)).Add(typ))
	}

	out = append(out, jen.Id("err").Id("error"))
	ret = append(ret, jen.Id("err"))

	stmts = append(
		stmts,
		jen.Var().Id("respData").Struct(dataFields...),
		jen.Id("err").Op("=").Id("client").Dot("Execute").Call(
			jen.Id("ctx"),
			jen.Id("op"),
			jen.Op("&").Id("respData"),
		),
	)

	stmts = append(stmts, jen.Return(ret...))

	name := strings.Title(op.Name)
	return jen.Func().Id(name).Params(in...).Params(out...).Block(stmts...)
}

func main() {
	var schemaFilenames, queryFilenames []string
	var pkgName, outputFilename string
	flag.Var((*stringSliceFlag)(&schemaFilenames), "s", "schema filename")
	flag.Var((*stringSliceFlag)(&queryFilenames), "q", "query filename")
	flag.StringVar(&pkgName, "n", "main", "package name")
	flag.StringVar(&outputFilename, "o", "", "output filename")
	flag.Usage = func() {
		fmt.Println(usage)
	}
	flag.Parse()

	if len(schemaFilenames) == 0 || outputFilename == "" || len(flag.Args()) > 0 {
		flag.Usage()
		os.Exit(1)
	}

	var sources []*ast.Source
	for _, filename := range schemaFilenames {
		b, err := os.ReadFile(filename)
		if err != nil {
			log.Fatalf("failed to load schema %q: %v", filename, err)
		}
		sources = append(sources, &ast.Source{Name: filename, Input: string(b)})
	}

	schema, gqlErr := gqlparser.LoadSchema(sources...)
	if gqlErr != nil {
		log.Fatalf("failed to parse schema: %v", gqlErr)
	}

	var queries []*ast.QueryDocument
	for _, filename := range queryFilenames {
		b, err := os.ReadFile(filename)
		if err != nil {
			log.Fatalf("failed to load query %q: %v", filename, err)
		}

		q, gqlErr := gqlparser.LoadQuery(schema, string(b))
		if gqlErr != nil {
			log.Fatalf("failed to parse query %q: %v", filename, gqlErr)
		}

		queries = append(queries, q)
	}

	f := jen.NewFile(pkgName)
	f.HeaderComment("Code generated by gqlclientgen - DO NOT EDIT")

	var typeNames []string
	for _, def := range schema.Types {
		if def.BuiltIn || def == schema.Query || def == schema.Mutation || def == schema.Subscription {
			continue
		}
		typeNames = append(typeNames, def.Name)
	}

	sort.Strings(typeNames)

	for _, name := range typeNames {
		stmt := genDef(schema, schema.Types[name])
		if stmt != nil {
			f.Add(stmt).Line()
		}
	}

	for _, q := range queries {
		for _, op := range q.Operations {
			f.Add(genOp(schema, op)).Line()
		}
	}

	if err := f.Save(outputFilename); err != nil {
		log.Fatalf("failed to save output file: %v", err)
	}
}
