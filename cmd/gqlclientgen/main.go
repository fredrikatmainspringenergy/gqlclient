package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
  -n <package>  Go package name, defaults to the dirname of the output file.
  -d            Omit deprecated fields and enum values
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

func genDescription(s string) jen.Code {
	if s == "" {
		return jen.Null()
	}
	s = "// " + strings.ReplaceAll(s, "\n", "\n// ")
	return jen.Comment(s).Line()
}

func genType(schema *ast.Schema, t *ast.Type) jen.Code {
	var prefix []jen.Code

	toplevel := true
	for t.Elem != nil {
		prefix = append(prefix, jen.Index())
		toplevel = false
		t = t.Elem
	}

	def, ok := schema.Types[t.NamedType]
	if !ok {
		panic(fmt.Sprintf("unknown type name %q", t.NamedType))
	}

	var gen jen.Code
	switch def.Name {
	// Standard types
	case "Int":
		gen = jen.Int32()
	case "Float":
		gen = jen.Float64()
	case "String":
		gen = jen.String()
	case "Boolean":
		gen = jen.Bool()
	case "ID":
		gen = jen.String()
	// Convenience types
	case "Time":
		gen = jen.Qual(gqlclient, "Time")
	case "Map":
		gen = jen.Map(jen.String()).Interface()
	case "Upload":
		gen = jen.Qual(gqlclient, "Upload")
	case "Any":
		gen = jen.Interface()
	default:
		if def.BuiltIn {
			panic(fmt.Sprintf("unsupported built-in type: %s", def.Name))
		}
		gen = jen.Id(def.Name)
	}

	if !t.NonNull {
		switch def.Name {
		case "ID", "Time", "Map", "Any":
			// These don't need a pointer, they have a recognizable zero value
		default:
			prefix = append(prefix, jen.Op("*"))
		}
	} else if toplevel {
		switch def.Kind {
		case ast.Object, ast.Interface:
			// Required to deal with recursive types
			prefix = append(prefix, jen.Op("*"))
		}
	}

	return jen.Add(prefix...).Add(gen)
}

func hasDeprecated(list ast.DirectiveList) bool {
	return list.ForName("deprecated") != nil
}

func genDef(schema *ast.Schema, def *ast.Definition, omitDeprecated bool) *jen.Statement {
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
			if omitDeprecated && hasDeprecated(val.Directives) {
				continue
			}
			nameWords := strings.Split(strings.ToLower(val.Name), "_")
			for i := range nameWords {
				nameWords[i] = strings.Title(nameWords[i])
			}
			name := strings.Join(nameWords, "")
			desc := genDescription(val.Description)
			defs = append(defs,
				jen.Add(desc).Id(def.Name+name).Id(def.Name).Op("=").Lit(val.Name),
			)
		}
		return jen.Add(
			jen.Type().Id(def.Name).String(),
			jen.Line(),
			jen.Const().Defs(defs...),
		)
	case ast.Object, ast.InputObject:
		var fields []jen.Code
		for _, field := range def.Fields {
			if omitDeprecated && hasDeprecated(field.Directives) {
				continue
			}
			if field.Name == "__schema" || field.Name == "__type" {
				continue // TODO
			}
			name := strings.Title(field.Name)
			jsonTag := field.Name
			if !field.Type.NonNull {
				jsonTag += ",omitempty"
			}
			tag := jen.Tag(map[string]string{"json": jsonTag})
			desc := genDescription(field.Description)
			fields = append(fields,
				jen.Add(desc).Id(name).Add(genType(schema, field.Type)).Add(tag),
			)
		}
		return jen.Type().Id(def.Name).Struct(fields...)
	case ast.Interface, ast.Union:
		possibleTypes := schema.GetPossibleTypes(def)

		var typeNames []string
		for _, typ := range possibleTypes {
			typeNames = append(typeNames, typ.Name)
		}

		var fields []jen.Code
		for _, field := range def.Fields {
			if omitDeprecated && hasDeprecated(field.Directives) {
				continue
			}
			if field.Name == "__schema" || field.Name == "__type" {
				continue // TODO
			}
			name := strings.Title(field.Name)
			jsonTag := field.Name
			if !field.Type.NonNull {
				jsonTag += ",omitempty"
			}
			tag := jen.Tag(map[string]string{"json": jsonTag})
			desc := genDescription(field.Description)
			fields = append(fields,
				jen.Add(desc).Id(name).Add(genType(schema, field.Type)).Add(tag),
			)
		}
		if len(fields) > 0 {
			fields = append(fields, jen.Line())
		}
		fields = append(fields,
			jen.Comment("Underlying value of the GraphQL "+strings.ToLower(string(def.Kind))),
			jen.Id("Value").Id(def.Name+"Value").Tag(map[string]string{"json": "-"}),
		)

		var cases []jen.Code
		for _, typ := range possibleTypes {
			cases = append(cases, jen.Case(jen.Lit(typ.Name)).Block(
				jen.Id("base").Dot("Value").Op("=").New(jen.Id(typ.Name)),
			))
		}

		errPrefix := fmt.Sprintf("gqlclient: %v %v: ", strings.ToLower(string(def.Kind)), def.Name)
		switch def.Kind {
		case ast.Interface:
			cases = append(cases, jen.Case(jen.Lit("")).Block(
				jen.Return(jen.Nil()),
			))
		case ast.Union:
			cases = append(cases, jen.Case(jen.Lit("")).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit(errPrefix+"missing __typename field"))),
			))
		}
		cases = append(cases,
			jen.Default().Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit(errPrefix+"unknown __typename %q"), jen.Id("data").Dot("TypeName"))),
			),
		)

		var stmts []jen.Code
		stmts = append(stmts, jen.Type().Id(def.Name).Struct(fields...))
		stmts = append(stmts, jen.Line())
		stmts = append(stmts, jen.Func().Params(
			jen.Id("base").Op("*").Id(def.Name),
		).Id("UnmarshalJSON").Params(
			jen.Id("b").Index().Byte(),
		).Params(
			jen.Id("error"),
		).Block(
			jen.Type().Id("Raw").Id(def.Name),
			jen.Var().Id("data").Struct(
				jen.Op("*").Id("Raw"),
				jen.Id("TypeName").String().Tag(map[string]string{"json": "__typename"}),
			),
			jen.Id("data").Dot("Raw").Op("=").Parens(jen.Op("*").Id("Raw")).Parens(jen.Id("base")),
			jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(
				jen.Id("b"),
				jen.Op("&").Id("data"),
			),
			jen.If(jen.Id("err").Op("!=").Nil()).Block(jen.Return(jen.Id("err"))),
			jen.Switch(jen.Id("data").Dot("TypeName")).Block(cases...),
			jen.Return(jen.Qual("encoding/json", "Unmarshal").Call(
				jen.Id("b"),
				jen.Id("base").Dot("Value"),
			)),
		))
		stmts = append(stmts, jen.Line())
		stmts = append(stmts, jen.Comment(def.Name+"Value is one of: "+strings.Join(typeNames, " | ")).Line())
		stmts = append(stmts, jen.Type().Id(def.Name+"Value").Interface(
			jen.Id("is"+def.Name).Params(),
		))
		return jen.Add(stmts...)
	default:
		panic(fmt.Sprintf("unsupported definition kind: %s", def.Kind))
	}
}

func collectFragments(frags map[*ast.FragmentDefinition]struct{}, selSet ast.SelectionSet) {
	for _, sel := range selSet {
		switch sel := sel.(type) {
		case *ast.Field:
			if sel.Name != sel.Alias {
				panic(fmt.Sprintf("field aliases aren't supported"))
			}
			collectFragments(frags, sel.SelectionSet)
		case *ast.FragmentSpread:
			frags[sel.Definition] = struct{}{}
			collectFragments(frags, sel.Definition.SelectionSet)
		case *ast.InlineFragment:
			collectFragments(frags, sel.SelectionSet)
		default:
			panic(fmt.Sprintf("unsupported selection type: %T", sel))
		}
	}
}

func genOp(schema *ast.Schema, op *ast.OperationDefinition) *jen.Statement {
	frags := make(map[*ast.FragmentDefinition]struct{})
	collectFragments(frags, op.SelectionSet)

	var fragList ast.FragmentDefinitionList
	for frag := range frags {
		fragList = append(fragList, frag)
	}

	var query ast.QueryDocument
	query.Operations = ast.OperationList{op}
	query.Fragments = fragList

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
	var omitDeprecated bool
	flag.Var((*stringSliceFlag)(&schemaFilenames), "s", "schema filename")
	flag.Var((*stringSliceFlag)(&queryFilenames), "q", "query filename")
	flag.StringVar(&pkgName, "n", "", "package name")
	flag.StringVar(&outputFilename, "o", "", "output filename")
	flag.BoolVar(&omitDeprecated, "d", false, "omit deprecated fields")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	if len(schemaFilenames) == 0 || outputFilename == "" || len(flag.Args()) > 0 {
		flag.Usage()
		os.Exit(1)
	}

	if pkgName == "" {
		abs, err := filepath.Abs(outputFilename)
		if err != nil {
			log.Fatalf("failed to get absolute output filename: %v", err)
		}
		pkgName = filepath.Base(filepath.Dir(abs))
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
	f.HeaderComment("Code generated by gqlclientgen - DO NOT EDIT.")

	var typeNames []string
	for _, def := range schema.Types {
		if def.BuiltIn || def == schema.Query || def == schema.Mutation || def == schema.Subscription {
			continue
		}
		typeNames = append(typeNames, def.Name)
	}

	sort.Strings(typeNames)

	for _, name := range typeNames {
		def := schema.Types[name]
		stmt := genDef(schema, def, omitDeprecated)
		if stmt != nil {
			f.Add(genDescription(def.Description), stmt).Line()
		}
		for _, typ := range schema.GetImplements(def) {
			f.Func().Params(genType(schema, ast.NamedType(def.Name, nil))).Id("is" + typ.Name).Params().Block().Line()
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
