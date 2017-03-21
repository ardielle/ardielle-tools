// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

//
// export and RDL schema to Swagger 2.0 (http://swagger.io)
//

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ardielle/ardielle-go/rdl"
	"github.com/ardielle/ardielle-tools/rdl-plugins/swagger"
)

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	basePath := flag.String("b", "", "Base path")
	flag.Parse()
	data, err := ioutil.ReadAll(os.Stdin)
	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			ExportToSwagger(&schema, *pOutdir, *basePath)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "*** %v\n", err)
	os.Exit(1)
}

func outputWriter(outdir string, name string, ext string) (*bufio.Writer, *os.File, string, error) {
	sname := "anonymous"
	if strings.HasSuffix(outdir, ext) {
		name = filepath.Base(outdir)
		sname = name[:len(name)-len(ext)]
		outdir = filepath.Dir(outdir)
	}
	if name != "" {
		sname = name
	}
	if outdir == "" {
		return bufio.NewWriter(os.Stdout), nil, sname, nil
	}
	outfile := sname
	if !strings.HasSuffix(outfile, ext) {
		outfile += ext
	}
	path := filepath.Join(outdir, outfile)
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, "", err
	}
	writer := bufio.NewWriter(f)
	return writer, f, sname, nil
}

// ExportToSwagger exports the RDL schema to Swagger 2.0 format,
//   and serves it up on the specified server endpoint is provided, or outputs to stdout otherwise.
func ExportToSwagger(schema *rdl.Schema, outdir string, basePath string) error {
	sname := string(schema.Name)
	swaggerData, err := genSwagger(schema, basePath)
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(swaggerData, "", "    ")
	if err != nil {
		return err
	}
	//if the outdir is of the form hostname:port, then serve it up, otherwise write it to a file
	i := strings.Index(outdir, ":")
	if i < 0 {
		if outdir == "" {
			fmt.Printf("%s\n", string(j))
			return nil
		}
		out, file, _, err := outputWriter(outdir, sname, "_swagger.json")
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", string(j))
		out.Flush()
		if file != nil {
			file.Close()
		}
		return err
	}
	var endpoint string
	if i > 0 {
		endpoint = outdir
	} else {
		endpoint = "localhost" + outdir
	}
	filename := "/rdl-generated.json"
	if sname != "" {
		filename = "/" + sname + ".json"
	}
	fmt.Println("Serving Swagger resource here: 'http://" + endpoint + filename + "'. Ctrl-C to stop.")
	http.HandleFunc(filename, func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h["Access-Control-Allow-Origin"] = []string{"*"}
		h["Content-Type"] = []string{"application/json"}
		w.WriteHeader(200)
		fmt.Fprint(w, string(j))
	})
	return http.ListenAndServe(outdir, nil)
}

func genSwagger(schema *rdl.Schema, basePath string) (*swagger.Doc, error) {
	reg := rdl.NewTypeRegistry(schema)
	sname := string(schema.Name)
	swag := new(swagger.Doc)
	swag.Swagger = "2.0"
	swag.Schemes = []string{}
	//swag.Host = "localhost"

	title := "API"
	if sname != "" {
		title = "The " + sname + " API"
		basePath += "/" + sname
	}
	swag.Info = new(swagger.Info)
	swag.Info.Title = title
	if schema.Version != nil {
		swag.Info.Version = fmt.Sprintf("%d", *schema.Version)
		basePath += "/v" + fmt.Sprintf("%d", *schema.Version)
	}
	if schema.Base != "" {
		basePath = schema.Base
	}
	swag.BasePath = basePath
	if schema.Comment != "" {
		swag.Info.Description = schema.Comment
	}
	swag.BasePath = basePath
	if len(schema.Resources) > 0 {
		//paths := make(map[string]map[string]*swagger.Operation)
		paths := make(map[string]*swagger.PathItem)
		for _, r := range schema.Resources {
			path := r.Path
			actions, ok := paths[path]
			if !ok {
				actions = new(swagger.PathItem)
				paths[path] = actions
			}
			meth := strings.ToLower(r.Method)
			var action *swagger.Operation
			switch meth {
			case "get":
				action = actions.Get
				if action == nil {
					action = swagger.NewOperation()
					actions.Get = action
				}
			case "put":
				action = actions.Put
				if action == nil {
					action = swagger.NewOperation()
					actions.Put = action
				}
			case "post":
				action = actions.Post
				if action == nil {
					action = swagger.NewOperation()
					actions.Post = action
				}
			case "delete":
				action = actions.Delete
				if action == nil {
					action = swagger.NewOperation()
					actions.Delete = action
				}
			case "options":
				action = actions.Options
				if action == nil {
					action = swagger.NewOperation()
					actions.Options = action
				}
			case "patch":
				action = actions.Patch
				if action == nil {
					action = swagger.NewOperation()
					actions.Patch = action
				}
			}
			action.Summary = r.Comment
			tag := string(r.Type)       //fixme: RDL has no tags, the type is actually too fine grain for this
			action.Tags = []string{tag} //multiple tags include the resource in multiple sections
			action.Produces = []string{"application/json"}
			var ins []*swagger.Parameter
			if len(r.Inputs) > 0 {
				if r.Method == "POST" || r.Method == "PUT" {
					action.Consumes = []string{"application/json"}
				}
				for _, in := range r.Inputs {
					param := new(swagger.Parameter)
					param.Name = string(in.Name)
					param.Description = in.Comment
					required := true
					if in.Optional {
						required = false
					}
					param.Required = required
					if in.PathParam {
						param.In = "path"
					} else if in.QueryParam != "" {
						param.In = "query"
						param.Name = in.QueryParam //swagger has no formal arg concept
					} else if in.Header != "" {
						//swagger has no header params
						continue
					} else {
						param.In = "body"
					}
					ptype, pformat, ref := makeSwaggerTypeRef(reg, in.Type)
					param.Type = ptype
					param.Format = pformat
					param.Schema = ref

					if strings.Contains(in.QueryParam, "[]") {
						param.CollectionFormat = "multi"
					}

					ins = append(ins, param)
				}
				action.Parameters = ins
			}
			responses := make(map[string]*swagger.Response)
			expected := r.Expected
			addSwaggerResponse(responses, string(r.Type), expected, "")
			if len(r.Alternatives) > 0 {
				for _, alt := range r.Alternatives {
					addSwaggerResponse(responses, string(r.Type), alt, "")
				}
			}
			if len(r.Exceptions) > 0 {
				for sym, errdef := range r.Exceptions {
					errType := errdef.Type //xxx
					addSwaggerResponse(responses, errType, sym, errdef.Comment)
				}
			}
			action.Responses = responses
			//responses -> r.expected and r.exceptions
			//security -> r.auth
			//r.outputs?
			//action.description?
			action.OperationID = strings.ToLower(r.Method) + string(r.Type)
			switch meth {
			case "get":
				actions.Get = action
			case "put":
				actions.Put = action
			case "post":
				actions.Post = action
			case "delete":
				actions.Delete = action
			case "options":
				actions.Options = action
			case "patch":
				actions.Patch = action
			}
		}
		swag.Paths = paths
	}
	if len(schema.Types) > 0 {
		defs := make(map[string]swagger.Type)
		for _, t := range schema.Types {
			ref := makeSwaggerTypeDef(reg, t)
			if ref != nil {
				tName, _, _ := rdl.TypeInfo(t)
				defs[string(tName)] = ref
			}
		}
		if true {
			props := make(map[string]swagger.Type)
			codeType := make(swagger.Type)
			t := "integer"
			codeType["type"] = t
			f := "int32"
			codeType["format"] = f
			props["code"] = codeType
			msgType := make(swagger.Type)
			t2 := "string"
			msgType["type"] = t2
			props["message"] = msgType
			prop := make(swagger.Type)
			prop["required"] = []string{"code", "message"}
			prop["properties"] = props
			defs["ResourceError"] = prop
		}
		swag.Definitions = defs
	}
	return swag, nil
}

func addSwaggerResponse(responses map[string]*swagger.Response, errType string, sym string, errComment string) {
	code := rdl.StatusCode(sym)
	var schema swagger.Type
	if sym != "NO_CONTENT" {
		schema = make(swagger.Type)
		schema["$ref"] = "#/definitions/" + errType
	}
	description := rdl.StatusMessage(sym)
	if errComment != "" {
		description += " - " + errComment
	}
	responses[code] = &swagger.Response{description, schema}
}

func makeSwaggerTypeRef(reg rdl.TypeRegistry, itemTypeName rdl.TypeRef) (string, string, swagger.Type) {
	itype := string(itemTypeName)
	switch reg.FindBaseType(itemTypeName) {
	case rdl.BaseTypeInt8:
		return "string", "byte", nil //?
	case rdl.BaseTypeInt16, rdl.BaseTypeInt32, rdl.BaseTypeInt64:
		return "integer", strings.ToLower(itype), nil
	case rdl.BaseTypeFloat32:
		return "number", "float", nil
	case rdl.BaseTypeFloat64:
		return "number", "double", nil
	case rdl.BaseTypeString:
		return "string", "", nil
	case rdl.BaseTypeTimestamp:
		return "string", "date-time", nil
	case rdl.BaseTypeUUID, rdl.BaseTypeSymbol:
		return "string", strings.ToLower(itype), nil
	default:
		s := make(swagger.Type)
		s["$ref"] = "#/definitions/" + itype
		return "", "", s
	}
}

func makeSwaggerTypeDef(reg rdl.TypeRegistry, t *rdl.Type) swagger.Type {
	st := make(swagger.Type)
	bt := reg.BaseType(t)
	switch t.Variant {
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		st["description"] = typedef.Comment
		props := make(map[string]swagger.Type)
		var required []string
		if len(typedef.Fields) > 0 {
			for _, f := range typedef.Fields {
				if !f.Optional {
					required = append(required, string(f.Name))
				}
				ft := reg.FindType(f.Type)
				fbt := reg.BaseType(ft)
				prop := make(swagger.Type)
				prop["description"] = f.Comment
				switch fbt {
				case rdl.BaseTypeArray:
					prop["type"] = "array"
					if ft.Variant == rdl.TypeVariantArrayTypeDef && f.Items == "" {
						f.Items = ft.ArrayTypeDef.Items
					}
					if f.Items != "" {
						fitems := string(f.Items)
						items := make(swagger.Type)
						switch fitems {
						case "String":
							items["type"] = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items["type"] = "integer"
							items["format"] = strings.ToLower(fitems)
						default:
							items["$ref"] = "#/definitions/" + fitems
						}
						prop["items"] = items
					}
				case rdl.BaseTypeString:
					prop["type"] = strings.ToLower(fbt.String())
				case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
					prop["type"] = "integer"
					prop["format"] = strings.ToLower(fbt.String())
				case rdl.BaseTypeStruct:
					prop["$ref"] = "#/definitions/" + string(f.Type)
				case rdl.BaseTypeMap:
					prop["type"] = "object"
					if f.Items != "" {
						fitems := string(f.Items)
						items := make(swagger.Type)
						switch f.Items {
						case "String":
							items["type"] = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items["type"] = "integer"
							items["format"] = strings.ToLower(fitems)
						default:
							items["$ref"] = "#/definitions/" + fitems
						}
						prop["additionalProperties"] = items
					}
				default:
					prop["type"] = "_" + string(f.Type) + "_" //!
				}
				props[string(f.Name)] = prop
			}
		}
		st["properties"] = props
		if len(required) > 0 {
			st["required"] = required
		}
	case rdl.TypeVariantMapTypeDef:
		typedef := t.MapTypeDef
		st["type"] = "object"
		if typedef.Items != "Any" {
			items := make(swagger.Type)
			switch reg.FindBaseType(typedef.Items) {
			case rdl.BaseTypeString:
				items["type"] = strings.ToLower(string(typedef.Items))
			case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
				items["type"] = "integer"
				items["format"] = strings.ToLower(string(typedef.Items))
			default:
				items["$ref"] = "#/definitions/" + string(typedef.Items)
			}
			st["additionalProperties"] = items
		}
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		st["type"] = bt.String()
		if typedef.Items != "Any" {
			items := make(swagger.Type)
			switch reg.FindBaseType(typedef.Items) {
			case rdl.BaseTypeString:
				items["type"] = strings.ToLower(string(typedef.Items))
			case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
				items["type"] = "integer"
				items["format"] = strings.ToLower(string(typedef.Items))
			default:
				items["$ref"] = "#/definitions/" + string(typedef.Items)
			}
			st["items"] = items
		}
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		var tmp []string
		for _, el := range typedef.Elements {
			tmp = append(tmp, string(el.Symbol))
		}
		st["enum"] = tmp
	case rdl.TypeVariantUnionTypeDef:
		typedef := t.UnionTypeDef
		fmt.Println("[" + typedef.Name + ": Swagger doesn't support unions]")
	default:
		switch bt {
		case rdl.BaseTypeString, rdl.BaseTypeInt16, rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeFloat32, rdl.BaseTypeFloat64:
			return nil
		default:
			panic(fmt.Sprintf("whoops: %v", t))
		}
	}
	return st
}
