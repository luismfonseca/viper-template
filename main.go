package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type Type struct {
	Kind                 string
	TemplateName         string
	TemplateDefaultValue interface{}
	Fields               []Type
}

//Root: map[string]interface{}
//Leaf: map[string]defaultValue
func (t *Type) ToMap() interface{} {
	if len(t.Fields) == 0 {
		return t.TemplateDefaultValue
	} else {
		m := make(map[string]interface{})
		for _, k := range t.Fields {
			m[k.TemplateName] = k.ToMap()
		}
		return m
	}
}

type TemplateOptions struct {
	RootTypeName string
	ToJson       bool
}

var opts TemplateOptions

func setupCli() *cli.App {
	app := cli.NewApp()
	app.Name = "viper-template"
	app.Usage = "Generate template file from Viper Config"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "type",
			Usage:       "the root type of the config",
			Destination: &opts.RootTypeName,
		},
		cli.BoolFlag{
			Name:        "json",
			Usage:       "output template as JSON",
			Destination: &opts.ToJson,
		},
	}

	return app
}

func findRootTypeSpec(rootTypeName string, decls []ast.Decl) (*ast.StructType, error) {
	for _, d := range decls {
		switch tyd := d.(type) {
		case *ast.GenDecl:
			for _, s := range tyd.Specs {
				switch tyspec := s.(type) {
				case *ast.TypeSpec:
					if tyspec.Name.Name == rootTypeName {
						return tyspec.Type.(*ast.StructType), nil
					}

				}
			}
		}
	}

	return nil, errors.New("Couldn't find roottype '" + opts.RootTypeName + "'.")
}

func parseStructType(templateName string, structType *ast.StructType) Type {
	t := Type{
		Kind:         "struct",
		TemplateName: templateName,
	}

	for _, f := range structType.Fields.List {

		tagWithoutTickets := f.Tag.Value[1 : len(f.Tag.Value)-1]
		tag := reflect.StructTag(tagWithoutTickets)
		templateName := tag.Get("mapstructure")

		switch x := f.Type.(type) {
		case *ast.StructType:
			t.Fields = append(t.Fields, parseStructType(templateName, x))
		case *ast.StarExpr:
			resolvedPrt := x.X.(*ast.Ident).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType)

			t.Fields = append(t.Fields, parseStructType(templateName, resolvedPrt))
		case *ast.Ident:
			t.Fields = append(t.Fields, Type{Kind: string(x.Name), TemplateName: templateName})
		default:
			t.Fields = append(t.Fields, Type{Kind: "unknown", TemplateName: templateName})
		}
	}
	return t
}

func readExistingJsonTemplate(filename string) (map[string]interface{}, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(fileBytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func mergeMapValues(a, b map[string]interface{}) {
	for k, _ := range a {

		switch amap := a[k].(type) {
		case map[string]interface{}:
			if b[k] != nil {
				switch bmap := b[k].(type) {
				case map[string]interface{}:
					mergeMapValues(amap, bmap)
				}
			}
		default:
			if b[k] != nil {
				a[k] = b[k]
			}
		}
	}
}

func main() {
	app := setupCli()
	app.Action = func(c *cli.Context) error {

		// 1. Parse file
		filename := c.Args().Get(0)
		parsedFile, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		// 2. Find root type
		rootTypeSpec, err := findRootTypeSpec(opts.RootTypeName, parsedFile.Decls)
		if err != nil {
			return err
		}

		// 3. Parse the structure to a map[string]interface{}
		rootType := parseStructType("", rootTypeSpec)
		rootTypeMap := rootType.ToMap().(map[string]interface{})

		// 4. Read existing json template to use as default values
		templateFilename := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".json.template"
		existingValues, err := readExistingJsonTemplate(templateFilename)
		if err != nil {
			existingValues = make(map[string]interface{})
		}

		// 5. Extract default values from existing JSON
		mergeMapValues(rootTypeMap, existingValues)

		jsonBytes, err := json.MarshalIndent(rootTypeMap, "", "  ")
		if err != nil {
			return err
		}
		jsonBytes = append(jsonBytes, byte('\n'))

		// 6. Write template file
		ioutil.WriteFile(templateFilename, jsonBytes, 0644)

		return nil
	}

	app.Run(os.Args)
}
