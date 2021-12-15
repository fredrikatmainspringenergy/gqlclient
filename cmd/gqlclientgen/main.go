package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

const usage = `usage: gqlclientgen <schema> <package> <output>`

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
		return jen.Qual("git.sr.ht/~emersion/gqlclient", "Upload")
	case "Any":
		return jen.Interface()
	default:
		if def.BuiltIn {
			panic(fmt.Sprintf("unsupported built-in type: %s", def.Name))
		}
		return jen.Id(def.Name)
	}
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println(usage)
		os.Exit(1)
	}
	schemaPath := os.Args[1]
	pkgName := os.Args[2]
	outputPath := os.Args[3]

	b, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("failed to load schema: %v", err)
	}

	src := ast.Source{Name: schemaPath, Input: string(b)}
	schema, gqlErr := gqlparser.LoadSchema(&src)
	if gqlErr != nil {
		log.Fatalf("failed to parse schema: %v", gqlErr)
	}

	f := jen.NewFile(pkgName)

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
		switch def.Kind {
		case ast.Scalar:
			switch def.Name {
			case "Time", "Map", "Upload", "Any":
				// Convenience types
			default:
				f.Type().Id(def.Name).String()
			}
		case ast.Enum:
			f.Type().Id(def.Name).String()
			var defs []jen.Code
			for _, val := range def.EnumValues {
				nameWords := strings.Split(strings.ToLower(val.Name), "_")
				for i := range nameWords {
					nameWords[i] = strings.Title(nameWords[i])
				}
				name := strings.Join(nameWords, "")
				defs = append(defs, jen.Id(def.Name+name).Id(def.Name).Op("=").Lit(val.Name))
			}
			f.Const().Defs(defs...)
		case ast.Object:
			var fields []jen.Code
			for _, field := range def.Fields {
				if field.Name == "__schema" || field.Name == "__type" {
					continue // TODO
				}
				name := strings.Title(field.Name)
				fields = append(fields, jen.Id(name).Add(genType(schema, field.Type)))
			}
			f.Type().Id(def.Name).Struct(fields...)
		case ast.Interface:
			f.Type().Id(def.Name).Interface() // TODO
		default:
			panic(fmt.Sprintf("unsupported definition kind: %s", def.Kind))
		}
	}

	if err := f.Save(outputPath); err != nil {
		log.Fatalf("failed to save output file: %v", err)
	}
}
