// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"bufio"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ardielle/ardielle-go/rdl"
)

type serverGenerator struct {
	registry    rdl.TypeRegistry
	schema      *rdl.Schema
	name        string
	writer      *bufio.Writer
	err         error
	banner      string
	prefixEnums bool
	precise     bool
	ns          string
	librdl      string
}

// GenerateGoServer generates the server code for the RDL-defined service
func GenerateGoServer(opts *generateOptions) error {
	banner := opts.banner
	schema := opts.schema
	outdir := opts.dirName
	ns := opts.ns
	librdl := opts.librdl
	prefixEnums := opts.prefixEnums
	precise := opts.preciseTypes
	name := strings.ToLower(string(schema.Name))
	if outdir == "" {
		outdir = "."
		name = name + "_server.go"
	} else if strings.HasSuffix(outdir, ".go") {
		name = filepath.Base(outdir)
		outdir = filepath.Dir(outdir)
	} else {
		name = name + "_server.go"
	}
	filepath := outdir + "/" + name
	out, file, _, err := outputWriter(filepath, "", ".go")
	if err != nil {
		return err
	}
	if file != nil {
		defer func() {
			file.Close()
			err := goFmt(filepath)
			if err != nil {
				fmt.Println("Warning: could not format go code:", err)
			}
		}()
	}
	reg := rdl.NewTypeRegistry(schema)
	gen := &serverGenerator{reg, schema, capitalize(string(schema.Name)), out, nil, banner, prefixEnums, precise, ns, librdl}
	gen.processTemplate(serverTemplate)
	out.Flush()
	return gen.err
}

const serverTemplate = `{{header}}

package {{package}}

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	rdl "{{rdlruntime}}"
	"{{httptreemux}}"
)

var _ = json.Marshal
var _ = ioutil.Discard

//
// Init initializes the {{name}} server with a service identity and an
// implementation ({{cName}}Handler), and returns an http.Handler to serve it.
//
func Init(impl {{cName}}Handler, baseURL string, authz rdl.Authorizer, authns ...rdl.Authenticator) http.Handler {
	u, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		log.Fatal(err)
	}
	b := u.Path
	router := httptreemux.New()
	adaptor := {{name}}Adaptor{impl, authz, authns, b}
{{range .Resources}}
	router.{{uMethod .}}(b+"{{methodPath .}}", func(w http.ResponseWriter, r *http.Request, ps map[string]string) {
		adaptor.{{handlerName .}}(w, r, ps)
	}){{end}}
	router.NotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
		rdl.JSONResponse(w, 404, rdl.ResourceError{Code: http.StatusNotFound, Message: "Not Found"})
	}
	log.Printf("Initialized {{name}} service at '%s'\n", baseURL)
	return router
}

//
// {{cName}}Handler is the interface that the service implementation must conform to
//
type {{cName}}Handler interface {{openBrace}}{{range .Resources}}
	{{methodSig .}}{{end}}
	Authenticate(context *rdl.ResourceContext) bool
}

//
// {{name}}Adaptor - this adapts the http-oriented router calls to the non-http service handler.
//
type {{name}}Adaptor struct {
	impl           {{cName}}Handler
	authorizer     rdl.Authorizer
	authenticators []rdl.Authenticator
	endpoint       string
}

func (adaptor {{name}}Adaptor) authenticate(context *rdl.ResourceContext) bool {
	if adaptor.authenticators != nil {
		for _, authn := range adaptor.authenticators {
			var creds []string
			var ok bool
			header := authn.HTTPHeader()
			if strings.HasPrefix(header, "Cookie.") {
				if cookies, ok2 := context.Request.Header["Cookie"]; ok2 {
					prefix := header[7:] + "="
					for _, c := range cookies {
						if strings.HasPrefix(c, prefix) {
							creds = append(creds, c[len(prefix):])
							ok = true
							break
						}
					}
				}
			} else {
				creds, ok = context.Request.Header[header]
			}
			if ok && len(creds) > 0 {
				principal := authn.Authenticate(creds[0])
				if principal != nil {
					context.Principal = principal
					return true
				}
			}
		}
	}
	if adaptor.impl.Authenticate(context) {
		return true
	}
	log.Println("*** Authentication failed against all authenticator(s)")
	return false
}

func (adaptor {{name}}Adaptor) authorize(context *rdl.ResourceContext, action string, resource string) bool {
	if adaptor.authorizer == nil {
		return true
	}
	if !adaptor.authenticate(context) {
		return false
	}
	ok, err := adaptor.authorizer.Authorize(action, resource, context.Principal)
	if err == nil {
		return ok
	}
	log.Println("*** Error when trying to authorize:", err)
	return false
}

func intFromString(s string) int64 {
	var n int64 = 0
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

func floatFromString(s string) float64 {
	var n float64 = 0
	_, _ = fmt.Sscanf(s, "%g", &n)
	return n
}
{{range .Resources}}
func (adaptor {{name}}Adaptor) {{handlerSig .}} {
	context := &rdl.ResourceContext{Writer: writer, Request: request, Params: params, Principal: nil}
{{handlerBody .}}
}
{{end}}`

func makeTypeRef(reg rdl.TypeRegistry, t *rdl.Type, precise bool) string {
	switch t.Variant {
	case rdl.TypeVariantAliasTypeDef:
		typedef := t.AliasTypeDef
		return goType(reg, typedef.Type, false, "", "", precise, true)
	case rdl.TypeVariantStringTypeDef:
		typedef := t.StringTypeDef
		return goType(reg, typedef.Type, false, "", "", precise, true)
	case rdl.TypeVariantNumberTypeDef:
		typedef := t.NumberTypeDef
		return goType(reg, typedef.Type, false, "", "", precise, true)
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		return goType(reg, typedef.Type, false, typedef.Items, "", precise, true)
	case rdl.TypeVariantMapTypeDef:
		typedef := t.MapTypeDef
		return goType(reg, typedef.Type, false, typedef.Items, typedef.Keys, precise, true)
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		return goType(reg, typedef.Type, false, "", "", precise, true)
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		return goType(reg, typedef.Type, false, "", "", precise, true)
	case rdl.TypeVariantUnionTypeDef:
		return "interface{}" //! FIX
	}
	return "?" //never happens
}

func (gen *serverGenerator) processTemplate(templateSource string) error {
	commentFun := func(s string) string {
		return formatComment(s, 0, 80)
	}
	basenameFunc := func(s string) string {
		i := strings.LastIndex(s, ".")
		if i >= 0 {
			s = s[i+1:]
		}
		return s
	}
	fieldFun := func(f rdl.StructFieldDef) string {
		optional := f.Optional
		fType := goType(gen.registry, f.Type, optional, f.Items, f.Keys, gen.precise, true)
		fName := capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}
	funcMap := template.FuncMap{
		"httptreemux": func() string { return HttpTreeMuxGoImport },
		"rdlruntime":  func() string { return gen.librdl },
		"header":      func() string { return generationHeader(gen.banner) },
		"package":     func() string { return generationPackage(gen.schema, gen.ns) },
		"openBrace":   func() string { return "{" },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return flattenedFields(gen.registry, t) },
		"typeRef":     func(t *rdl.Type) string { return makeTypeRef(gen.registry, t, gen.precise) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"uMethod":     func(r *rdl.Resource) string { return strings.ToUpper(r.Method) },
		"methodSig":   func(r *rdl.Resource) string { return goServerMethodSignature(gen.registry, r, gen.precise) },
		"handlerName": func(r *rdl.Resource) string {
			n, _ := goMethodName(gen.registry, r, gen.precise)
			return uncapitalize(n) + "Handler"
		},
		"handlerSig": func(r *rdl.Resource) string { return goHandlerSignature(gen.registry, r, gen.precise) },
		"handlerBody": func(r *rdl.Resource) string {
			return goHandlerBody(gen.registry, gen.name, r, gen.precise, gen.prefixEnums)
		},
		"client":     func() string { return gen.name + "Client" },
		"server":     func() string { return gen.name + "Server" },
		"name":       func() string { return gen.name },
		"cName":      func() string { return capitalize(gen.name) },
		"methodName": func(r *rdl.Resource) string { n, _ := goMethodName(gen.registry, r, gen.precise); return n },
		"methodPath": func(r *rdl.Resource) string { return resourcePath(r) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(templateSource))
	return t.Execute(gen.writer, gen.schema)
}

func resourcePath(r *rdl.Resource) string {
	path := r.Path
	i := strings.Index(path, "?")
	if i >= 0 {
		path = path[0:i]
	}
	i = strings.Index(path, "{")
	for i >= 0 {
		j := strings.Index(path[i:], "}")
		if j < 0 {
			break
		}
		j += i
		path = path[0:i] + ":" + path[i+1:j] + path[j+1:]
		i = strings.Index(path, "{")
	}
	return path
}

const authenticateTemplate = `	if !adaptor.authenticate(context) {
		rdl.JSONResponse(writer, 401, rdl.ResourceError{Code: http.StatusUnauthorized, Message: "Unauthorized"})
		return
	}
`
const authorizeTemplate = `	if !adaptor.authorize(context, %q, %s) {
		rdl.JSONResponse(writer, 403, rdl.ResourceError{Code: http.StatusForbidden, Message: "Forbidden"})
		return
	}
`

func goHandlerBody(reg rdl.TypeRegistry, name string, r *rdl.Resource, precise bool, prefixEnums bool) string {
	s := ""
	var fargs []string
	bodyName := ""
	for _, in := range r.Inputs {
		name := "arg" + capitalize(string(in.Name))
		if in.QueryParam != "" {
			qname := in.QueryParam
			if in.Optional || in.Default != nil {
				s += goParamInit(reg, qname, name, in.Type, in.Default, in.Optional, precise, prefixEnums)
			} else {
				log.Printf("RDL error: queryparam '%s' must either be optional or have a default value\n", in.Name)
			}
			fargs = append(fargs, name)
		} else if in.PathParam {
			bt := reg.BaseTypeName(in.Type)
			switch bt {
			case "Enum":
				s += fmt.Sprintf("\t%s := New%s(context.Params[%q])\n", name, in.Type, in.Name)
			case "Int32", "Int64", "Int16", "Int8":
				if precise {
					s += fmt.Sprintf("\t%s := %s(intFromString(context.Params[%q]))\n", name, in.Type, in.Name)
				} else {
					s += fmt.Sprintf("\t%s := intFromString(context.Params[%q])\n", name, in.Name)
				}
			case "Float32", "Float64":
				if precise {
					s += fmt.Sprintf("\t%s := %s(floatFromString(context.Params[%q]))\n", name, in.Type, in.Name)
				} else {
					s += fmt.Sprintf("\t%s := floatFromString(context.Params[%q])\n", name, in.Name)
				}
			default:
				if precise && strings.ToLower(string(in.Type)) != "string" {
					s += fmt.Sprintf("\t%s := %s(context.Params[%q])\n", name, in.Type, in.Name)
				} else {
					s += fmt.Sprintf("\t%s := context.Params[%q]\n", name, in.Name)
				}
			}
			fargs = append(fargs, name)
		} else if in.Header != "" {
			hname := in.Header
			def := ""
			if in.Default != nil {
				switch v := in.Default.(type) {
				case string:
					def = fmt.Sprintf("%q", v)
				default:
					panic(fmt.Sprintf("implement me, default value: %v", in))
				}
				s += "\t" + name + "Optional := " + def + "\n"
				s += fmt.Sprintf("\t%s := rdl.HeaderParam(request, %q, %sOptional)\n", name, hname, name)
			} else if in.Optional {
				s += fmt.Sprintf("\t%s := rdl.OptionalHeaderParam(request, %q)\n", name, hname)
			} else {
				s += fmt.Sprintf("\t%s := rdl.HeaderParam(request, %q, \"\")\n", name, hname)
			}
			fargs = append(fargs, name)
		} else {
			bodyName = name
			pgtype := goType(reg, in.Type, false, "", "", precise, true)
			s += "\tvar " + bodyName + " " + pgtype + "\n"
			s += "\toserr := json.NewDecoder(request.Body).Decode(&" + bodyName + ")\n"
			s += "\tif oserr != nil {\n"
			s += "\t\trdl.JSONResponse(writer, http.StatusBadRequest, rdl.ResourceError{Code: http.StatusBadRequest, Message: \"Bad request: \" + oserr.Error()})\n"
			s += "\t\treturn\n"
			s += "\t}\n"
			fargs = append(fargs, bodyName)
		}
	}
	if r.Auth != nil {
		if r.Auth.Authenticate {
			s += authenticateTemplate
		} else if r.Auth.Action != "" && r.Auth.Resource != "" {
			resource := r.Auth.Resource
			i := strings.Index(resource, "{")
			for i >= 0 {
				j := strings.Index(resource[i:], "}")
				if j < 0 {
					break
				}
				j += i
				val := "string(arg" + capitalize(resource[i+1:j]) + ")"
				resource = resource[0:i] + "\" + " + val + " + \"" + resource[j+1:]
				i = strings.Index(resource, "{")
			}
			resource = "\"" + resource
			if strings.HasSuffix(resource, "+ \"") {
				resource = resource[0 : len(resource)-3]
			} else {
				resource = resource + "\""
			}
			if strings.HasPrefix(resource, "\"\" + ") {
				resource = resource[5:]
			}
			s += fmt.Sprintf(authorizeTemplate, r.Auth.Action, resource)
		} else {
			log.Println("*** Badly formed auth spec in resource input:", r)
		}
	}
	methName, _ := goMethodName(reg, r, precise)
	sargs := ""
	if len(fargs) > 0 {
		sargs = ", " + strings.Join(fargs, ", ")
	}
	outHeaders := ""
	for _, v := range r.Outputs {
		outHeaders += ", " + string(v.Name)
	}
	noContent := r.Expected == "NO_CONTENT" && len(r.Alternatives) == 0
	if noContent {
		s += "\terr" + outHeaders + " := adaptor.impl." + capitalize(methName) + "(context" + sargs + ")\n"
	} else {
		s += "\tdata" + outHeaders + ", err := adaptor.impl." + capitalize(methName) + "(context" + sargs + ")\n"
	}
	s += "\tif err != nil {\n"
	s += "\t\tswitch e := err.(type) {\n"
	s += "\t\tcase *rdl.ResourceError:\n"
	//special case the 304 response, which MUST have an etag in it
	for _, v := range r.Outputs {
		if strings.ToLower(v.Header) == "etag" {
			s += "\t\t\tif e.Code == 304 && " + string(v.Name) + " != \"\" {\n"
			s += "\t\t\t\twriter.Header().Set(\"" + v.Header + "\", " + string(v.Name) + ")\n"
			s += "\t\t\t}\n"
			break
		}
	}

	s += "\t\t\trdl.JSONResponse(writer, e.Code, err)\n"
	s += "\t\tdefault:\n"
	s += "\t\t\trdl.JSONResponse(writer, 500, &rdl.ResourceError{Code: 500, Message: e.Error()})\n"
	s += "\t\t}\n"
	s += "\t} else {\n"
	for _, v := range r.Outputs {
		vname := string(v.Name)
		if v.Optional {
			s += "\t\tif " + vname + " != nil {\n"
			s += "\t\t\twriter.Header().Set(\"" + v.Header + "\", " + vname + ")\n"
			s += "\t\t}\n"
		} else {
			s += "\t\twriter.Header().Set(\"" + v.Header + "\", " + vname + ")\n"
		}
	}
	if noContent { //other non-content responses?
		s += fmt.Sprintf("\t\twriter.WriteHeader(204)\n")
	} else {
		//fixme: handle alternative responses. How deos the handler pass them back?
		s += fmt.Sprintf("\t\trdl.JSONResponse(writer, %s, data)\n", rdl.StatusCode(r.Expected))
	}
	s += "\t}\n"
	return s
}

func goParamInit(reg rdl.TypeRegistry, qname string, pname string, ptype rdl.TypeRef, pdefault interface{}, poptional bool, precise bool, prefixEnums bool) string {
	s := ""
	gtype := goType(reg, ptype, false, "", "", precise, true)
	switch gtype {
	default:
		t := reg.FindType(ptype)
		bt := reg.BaseType(t)
		switch bt {
		case rdl.BaseTypeString:
			if pdefault == nil {
				if precise && gtype != "string" {
					s += "\t" + pname + " := " + gtype + "(rdl.OptionalStringParam(request, \"" + qname + "\"))\n"
				} else {
					s += "\t" + pname + " := rdl.OptionalStringParam(request, \"" + qname + "\")\n"
				}
			} else {
				def := fmt.Sprintf("%q", pdefault)
				if precise && gtype != "string" {
					s += "\t" + pname + "Val, _ := rdl.StringParam(request, \"" + qname + "\", " + def + ")\n"
					s += "\t" + pname + " := " + gtype + "(" + pname + "Val)\n"
				} else {
					s += "\t" + pname + ", _ := rdl.StringParam(request, \"" + qname + "\", " + def + ")\n"
				}
			}
		case rdl.BaseTypeInt32, rdl.BaseTypeInt16, rdl.BaseTypeInt8, rdl.BaseTypeInt64, rdl.BaseTypeFloat32, rdl.BaseTypeFloat64:
			stype := fmt.Sprint(bt)
			if pdefault == nil {
				s += "\t" + pname + ", err := rdl.Optional" + stype + "Param(request, \"" + qname + "\")\n" //!
				s += "\tif err != nil {\n\t\trdl.JSONResponse(writer, 400, err)\n\t\treturn\n\t}\n"
			} else {
				def := "0"
				switch v := pdefault.(type) {
				case float64:
					def = fmt.Sprintf("%v", v)
				default:
					fmt.Println("fix me:", pdefault)
					panic("fix me")
				}
				if precise {
					s += "\t" + pname + "_, err := rdl." + stype + "Param(request, \"" + qname + "\", " + def + ")\n"
				} else {
					s += "\t" + pname + ", err := rdl." + stype + "Param(request, \"" + qname + "\", " + def + ")\n"
				}
				s += "\tif err != nil {\n\t\trdl.JSONResponse(writer, 400, err)\n\t\treturn\n\t}\n"
				if precise {
					s += "\t" + pname + " := " + gtype + "(" + pname + "_)\n"
				}
			}
		case rdl.BaseTypeBool:
			if pdefault == nil {
				s += "\t" + pname + ", err := rdl.OptionalBoolParam(request, \"" + qname + "\")\n"
				s += "\tif err != nil {\n"
				s += "\t\trdl.JSONResponse(writer, 400, err)\n"
				s += "\t\treturn\n"
				s += "\t}\n"
			} else {
				def := fmt.Sprintf("%v", pdefault)
				s += "\tvar " + pname + "Optional " + gtype + " = " + def + "\n"
				s += "\t" + pname + ", err := rdl.BoolParam(request, \"" + qname + "\", " + pname + "Optional)\n"
				s += "\tif err != nil {\n"
				s += "\t\trdl.JSONResponse(writer, 400, err)\n"
				s += "\t\treturn\n"
				s += "\t}\n"
			}
		case rdl.BaseTypeEnum:
			if pdefault == nil {
				s += fmt.Sprintf("\tvar %s *%s\n", pname, gtype)
				s += fmt.Sprintf("\t%sOptional := rdl.OptionalStringParam(request, %q)\n", pname, qname)
				s += fmt.Sprintf("\tif %sOptional != \"\" {\n", pname)
				s += "\t\tp" + pname + " := New" + gtype + "(" + pname + "Optional)\n"
				s += "\t\t" + pname + " = &p" + pname + "\n"
				s += "\t}\n"
			} else {
				if prefixEnums {
					pdefault = gtype + SnakeToCamel(fmt.Sprint(pdefault))
				}
				s += fmt.Sprintf("\t%sOptional, _ := rdl.StringParam(request, %q, %v.String())\n", pname, qname, pdefault)
				if poptional {
					s += "\tp" + pname + " := New" + gtype + "(" + pname + "Optional)\n"
					s += "\t" + pname + " := &p" + pname + "\n"
				} else {
					s += "\t" + pname + " := New" + gtype + "(" + pname + "Optional)\n"
				}
			}
		default:
			fmt.Println("fix me:", pname, "of type", gtype, "with base type", bt)
			panic("fix me")
		}
	}
	return s
}

func goHandlerSignature(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) string {
	methName, _ := goMethodName(reg, r, precise)
	args := "writer http.ResponseWriter, request *http.Request, params map[string]string"
	return methName + "Handler(" + args + ")"
}

func goServerMethodSignature(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) string {
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	returnSpec := "error"
	if !noContent {
		gtype := goType(reg, r.Type, false, "", "", precise, true)
		outHeaders := ""
		for _, v := range r.Outputs {
			outHeaders += ", " + goType(reg, v.Type, false, "", "", precise, true)
		}
		returnSpec = "(" + gtype + outHeaders + ", error)"
	}
	methName, params := goMethodName(reg, r, precise)
	sparams := ""
	if len(params) > 0 {
		sparams = ", " + strings.Join(params, ", ")
	}
	return capitalize(methName) + "(context *rdl.ResourceContext" + sparams + ") " + returnSpec
}

func goMethodName(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) (string, []string) {
	return goMethodName2(reg, r, precise, "")
}

func goMethodName2(reg rdl.TypeRegistry, r *rdl.Resource, precise bool, packageName string) (string, []string) {
	var params []string
	bodyType := string(safeTypeVarName(r.Type))
	for _, v := range r.Inputs {
		if v.Context != "" { //legacy field, to be removed
			continue
		}
		k := v.Name
		if v.QueryParam == "" && !v.PathParam && v.Header == "" {
			bodyType = string(v.Type)
		}
		optional := false
		if v.Optional {
			optional = true
		}
		params = append(params, goName(string(k))+" "+goType2(reg, v.Type, optional, "", "", precise, true, packageName))
	}
	meth := string(r.Name)
	if meth == "" {
		meth = strings.ToLower(string(r.Method)) + bodyType
	} else {
		meth = uncapitalize(meth)
	}
	return meth, params
}

func goName(name string) string {
	switch name {
	case "type", "default": //other reserved words
		return "_" + name
	default:
		return name
	}
}
