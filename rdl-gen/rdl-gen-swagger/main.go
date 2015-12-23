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
	"github.com/ardielle/ardielle-go/rdl"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	pOutdir := flag.String("o", ".", "Output directory")
	flag.String("s", "", "RDL source file")
	flag.Parse()
	data, err := ioutil.ReadAll(os.Stdin)
	if err == nil {
		var schema rdl.Schema
		err = json.Unmarshal(data, &schema)
		if err == nil {
			ExportToSwagger(&schema, *pOutdir)
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
func ExportToSwagger(schema *rdl.Schema, outdir string) error {
	sname := string(schema.Name)
	swaggerData, err := swagger(schema)
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

func swagger(schema *rdl.Schema) (*SwaggerDoc, error) {
	reg := rdl.NewTypeRegistry(schema)
	sname := string(schema.Name)
	swag := new(SwaggerDoc)
	swag.Swagger = "2.0"
	swag.Schemes = []string{"http"}
	//swag.Host = "localhost"
	swag.BasePath = "/api"

	title := "API"
	if sname != "" {
		title = "The " + sname + " API"
		swag.BasePath = "/api/" + sname
	}
	swag.Info = new(SwaggerInfo)
	swag.Info.Title = title
	if schema.Version != nil {
		swag.Info.Version = fmt.Sprintf("%d", *schema.Version)
		swag.BasePath += "/v" + fmt.Sprintf("%d", *schema.Version)
	}
	if schema.Comment != "" {
		swag.Info.Description = schema.Comment
	}
	if len(schema.Resources) > 0 {
		paths := make(map[string]map[string]*SwaggerAction)
		for _, r := range schema.Resources {
			path := r.Path
			actions, ok := paths[path]
			if !ok {
				actions = make(map[string]*SwaggerAction)
				paths[path] = actions
			}
			meth := strings.ToLower(r.Method)
			action, ok := actions[meth]
			if !ok {
				action = new(SwaggerAction)
			}
			action.Summary = r.Comment
			tag := string(r.Type)       //fixme: RDL has no tags, the type is actually too fine grain for this
			action.Tags = []string{tag} //multiple tags include the resource in multiple sections
			action.Produces = []string{"application/json"}
			var ins []*SwaggerParameter
			if len(r.Inputs) > 0 {
				if r.Method == "POST" || r.Method == "PUT" {
					action.Consumes = []string{"application/json"}
				}
				for _, in := range r.Inputs {
					param := new(SwaggerParameter)
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
					ins = append(ins, param)
				}
				action.Parameters = ins
			}
			responses := make(map[string]*SwaggerResponse)
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
			//action.operationId IGNORE

			actions[meth] = action
			paths[path] = actions
		}
		swag.Paths = paths
	}
	if len(schema.Types) > 0 {
		defs := make(map[string]*SwaggerType)
		for _, t := range schema.Types {
			ref := makeSwaggerTypeDef(reg, t)
			if ref != nil {
				tName, _, _ := rdl.TypeInfo(t)
				defs[string(tName)] = ref
			}
		}
		if true {
			props := make(map[string]*SwaggerType)
			codeType := new(SwaggerType)
			t := "integer"
			codeType.Type = t
			f := "int32"
			codeType.Format = f
			props["code"] = codeType
			msgType := new(SwaggerType)
			t2 := "string"
			msgType.Type = t2
			props["message"] = msgType
			prop := new(SwaggerType)
			prop.Required = []string{"code", "message"}
			prop.Properties = props
			defs["ResourceError"] = prop
		}
		swag.Definitions = defs
	}
	return swag, nil
}

func addSwaggerResponse(responses map[string]*SwaggerResponse, errType string, sym string, errComment string) {
	code := rdl.StatusCode(sym)
	var schema *SwaggerType
	if sym != "NO_CONTENT" {
		schema = new(SwaggerType)
		schema.Ref = "#/definitions/" + errType
	}
	description := rdl.StatusMessage(sym)
	if errComment != "" {
		description += " - " + errComment
	}
	responses[code] = &SwaggerResponse{description, schema}
}

func makeSwaggerTypeRef(reg rdl.TypeRegistry, itemTypeName rdl.TypeRef) (string, string, *SwaggerType) {
	itype := string(itemTypeName)
	switch reg.FindBaseType(itemTypeName) {
	case rdl.BaseTypeInt8:
		return "string", "byte", nil
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
		s := new(SwaggerType)
		s.Ref = "#/definitions/" + itype
		return "", "", s
	}
}

func makeSwaggerTypeDef(reg rdl.TypeRegistry, t *rdl.Type) *SwaggerType {
	st := new(SwaggerType)
	bt := reg.BaseType(t)
	switch t.Variant {
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		st.Description = typedef.Comment
		props := make(map[string]*SwaggerType)
		var required []string
		if len(typedef.Fields) > 0 {
			for _, f := range typedef.Fields {
				if f.Optional {
					required = append(required, string(f.Name))
				}
				ft := reg.FindType(f.Type)
				fbt := reg.BaseType(ft)
				prop := new(SwaggerType)
				prop.Description = f.Comment
				switch fbt {
				case rdl.BaseTypeArray:
					prop.Type = "array"
					if f.Items != "" {
						fitems := string(f.Items)
						items := new(SwaggerType)
						switch fitems {
						case "String":
							items.Type = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items.Type = "integer"
							items.Format = strings.ToLower(fitems)
						default:
							items.Ref = "#/definitions/" + fitems
						}
						prop.Items = items
					}
				case rdl.BaseTypeString:
					prop.Type = strings.ToLower(fbt.String())
				case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
					prop.Type = "integer"
					prop.Format = strings.ToLower(fbt.String())
				case rdl.BaseTypeStruct:
					prop.Type = "#/definitions/" + string(f.Type)
				case rdl.BaseTypeMap:
					prop.Type = "object"
					if f.Items != "" {
						fitems := string(f.Items)
						items := new(SwaggerType)
						switch f.Items {
						case "String":
							items.Type = strings.ToLower(fitems)
						case "Int32", "Int64", "Int16":
							items.Type = "integer"
							items.Format = strings.ToLower(fitems)
						default:
							items.Ref = "#/definitions/" + fitems
						}
						prop.AdditionalProperties = items
					}
				default:
					prop.Type = "_" + string(f.Type) + "_" //!
				}
				props[string(f.Name)] = prop
			}
		}
		st.Properties = props
		if len(required) > 0 {
			st.Required = required
		}
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		st.Type = bt.String()
		if typedef.Items != "Any" {
			items := new(SwaggerType)
			switch reg.FindBaseType(typedef.Items) {
			case rdl.BaseTypeString:
				items.Type = strings.ToLower(string(typedef.Items))
			case rdl.BaseTypeInt32, rdl.BaseTypeInt64, rdl.BaseTypeInt16:
				items.Type = "integer"
				items.Format = strings.ToLower(string(typedef.Items))
			default:
				items.Ref = "#/definitions/" + string(typedef.Items)
			}
			st.Items = items
		}
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		var tmp []string
		for _, el := range typedef.Elements {
			tmp = append(tmp, string(el.Symbol))
		}
		st.Enum = tmp
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

// SwaggerDoc is a representation of the top level object in swagger 2.0
type SwaggerDoc struct {
	Swagger string       `json:"swagger"`
	Info    *SwaggerInfo `json:"info"`
	//Host        string                               `json:"host"`
	BasePath    string                               `json:"basePath"`
	Schemes     []string                             `json:"schemes"`
	Paths       map[string]map[string]*SwaggerAction `json:"paths,omitempty"`
	Security    *map[string][]string                 `json:"security,omitempty"`
	Definitions map[string]*SwaggerType              `json:"definitions,omitempty"`
}

// SwaggerInfo -
type SwaggerInfo struct {
	Title          string          `json:"title"`
	Version        string          `json:"version"`
	Description    string          `json:"description,omitempty"`
	TermsOfService string          `json:"termsOfService,omitempty"`
	Contact        *SwaggerContact `json:"contact,omitempty"`
	License        *SwaggerLicense `json:"license,omitempty"`
}

// SwaggerContact -
type SwaggerContact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// SwaggerLicense -
type SwaggerLicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// SwaggerAction -
type SwaggerAction struct {
	Tags        []string                    `json:"tags,omitempty"`
	Summary     string                      `json:"summary,omitempty"`
	Description string                      `json:"description,omitempty"`
	OperationID string                      `json:"operationId,omitempty"`
	Consumes    []string                    `json:"consumes,omitempty"`
	Produces    []string                    `json:"produces,omitempty"`
	Parameters  []*SwaggerParameter         `json:"parameters,omitempty"`
	Responses   map[string]*SwaggerResponse `json:"responses,omitempty"`
	Security    map[string][]string         `json:"security,omitempty"`
}

// SwaggerParameter -
type SwaggerParameter struct {
	Name        string       `json:"name"`
	In          string       `json:"in"`
	Schema      *SwaggerType `json:"schema,omitempty"`
	Type        string       `json:"type,omitempty"`
	Format      string       `json:"format,omitempty"`
	Items       *SwaggerType `json:"items,omitempty"`
	Description string       `json:"description,omitempty"`
	Required    bool         `json:"required"`
}

// SwaggerResponse -
type SwaggerResponse struct {
	Description string       `json:"description,omitempty"`
	Schema      *SwaggerType `json:"schema,omitempty"`
}

// SwaggerType -
type SwaggerType struct {
	Properties           map[string]*SwaggerType `json:"properties,omitempty"`
	Required             []string                `json:"required,omitempty"`
	Type                 string                  `json:"type,omitempty"`
	Format               string                  `json:"format,omitempty"`
	Pattern              string                  `json:"pattern,omitempty"`
	Description          string                  `json:"description,omitempty"`
	Items                *SwaggerType            `json:"items,omitempty"`
	Ref                  string                  `json:"$ref,omitempty"`
	Enum                 []string                `json:"enum,omitempty"`
	AdditionalProperties *SwaggerType            `json:"additionalProperties,omitempty"`
}

/*
 * Swagger 1.4

type SwaggerResource struct {
	ApiVersion     string  `json:"apiVersion"`
	SwaggerVersion string `json:"swaggerVersion"`
	BasePath       string `json:"basePath"`
	ResourcePath   string `json:"resourcePath"`
	Produces       []string `json:"produces,omitempty"`
	Apis           []SwaggerApi
}

type SwaggerApi struct {
	Path string `json:"path"`
	Operations []SwaggerOperation `json:"operations"`
}

type SwaggerOperation struct {
	Method string `json:"method"`
	Summary string `json:"summary"`
	Notes string `json:"notes"`
	Type string `json:"type"`
	Nickname string `json:"nickname"`
	Authorizations SwaggerAuthorization `json:"authorizations,omitempty"`
	Parameters []SwaggerParameter `json:"parameters,omitempty"`
	ResponseMessages []SwaggerResponseMessage `json:"responseMessages,omitempty"`
}

type SwaggerParameter struct {
	Name string `json:"name"`
	Description *string `json:"description,omitempty"`
	Required bool `json:"required"`
	Type string `json:"type"`
	ParamType string `json:"paramType"`
	AllowMultiple bool `json:"allowMultiple"`
}

type SwaggerResponseMessage struct {
	Code int32 `json:"code"`
	Message string `json:"message"`
}

type SwaggerAuthorization struct {
	Oauth2 []SwaggerOauth2 `json:"oauth2,omitempty"`
}

type SwaggerOauth2 struct {
	Scope string `json:"scope"`
	Description *string `json:"description,omitempty"`
}
*/
