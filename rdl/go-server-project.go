package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ardielle/ardielle-go/rdl"
)

func GenerateGoServerProject(rdlSrcPath string, banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {

	//1. establish directory structure
	if strings.HasSuffix(outdir, ".go") {
		return fmt.Errorf("Output must be a directory: %q", outdir)
	}
	name := strings.ToLower(string(schema.Name))
	if outdir == "" {
		outdir = "."
	}

	gendir := outdir

	err := os.MkdirAll(gendir, 0755)
	if err != nil {
		return err
	}
	err = GenerateGoModel(banner, schema, gendir, "", librdl, prefixEnums, preciseTypes, untaggedUnions)
	if err != nil {
		return err
	}
	err = GenerateGoServer(banner, schema, gendir, "", librdl, prefixEnums, preciseTypes)
	if err != nil {
		return err
	}
	err = GenerateGoClient(banner, schema, gendir, "", librdl, prefixEnums, preciseTypes)
	if err != nil {
		return err
	}
	err = GenerateGoDaemonGenerate(banner, schema, gendir, "")
	if err != nil {
		return err
	}
	implpath := filepath.Join(gendir, name + ".go")
	if !fileExists(implpath) {
		err = GenerateGoDaemonImpl(banner, schema, gendir, ns, librdl, prefixEnums, preciseTypes, untaggedUnions)
		if err != nil {
			return err
		}
	}

	cmddir := filepath.Join(outdir, "cmd")
	daemondir := filepath.Join(cmddir, name+"d")
	err = os.MkdirAll(daemondir, 0755)
	if err != nil {
		return err
	}
	err = GenerateGoDaemonMain(banner, schema, daemondir, ns, librdl, prefixEnums, preciseTypes, untaggedUnions)
	if err != nil {
		return err
	}
	clidir := filepath.Join(cmddir, name)
	err = os.MkdirAll(clidir, 0755)
	if err != nil {
		return err
	}
	err = GenerateGoCLIMain(banner, schema, clidir, ns, librdl, prefixEnums, preciseTypes, untaggedUnions)
	if err != nil {
		return err
	}
	return nil
}

func GenerateGoDaemonGenerate(banner string, schema *rdl.Schema, outdir string, ns string) error {
	filepath := outdir + "/generate.go"
	if fileExists(filepath) {
		return nil
	}
	out, file, _, err := outputWriter(filepath, "", ".go")
	if err != nil {
		return err
	}
	if file != nil {
		defer func() {
			file.Close()
			err := goFmt(filepath)
			if err != nil {
				fmt.Println("Warning: could not format go code:", err, filepath)
			}
		}()
	}
   funcMap := template.FuncMap{
		"header":      func() string { return generationHeader(banner) },
		"package":     func() string { return generationPackage(schema, "") },
	}
	t := template.Must(template.New("FOO").Funcs(funcMap).Parse(serverGenerateTemplate))
	err = t.Execute(out, schema)
	if err != nil {
		fmt.Println("whoops:", err)
		return err
	}
	out.Flush()
	return nil
}

var serverGenerateTemplate = `{{header}}
package {{package}}

//go:generate rdl -sp generate --ns github.com/boynton/store go-server-project store.rdl

`

func GenerateGoDaemonMain(banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {
	filepath := outdir + "/main.go"
	out, file, _, err := outputWriter(filepath, "", ".go")
	if err != nil {
		return err
	}
	if file != nil {
		defer func() {
			file.Close()
			err := goFmt(filepath)
			if err != nil {
				fmt.Println("Warning: could not format go code:", err, filepath)
			}
		}()
	}
	registry := rdl.NewTypeRegistry(schema)
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
	name := strings.ToLower(string(schema.Name))
	fieldFun := func(f rdl.StructFieldDef) string {
		optional := f.Optional
		fType := goType(registry, f.Type, optional, f.Items, f.Keys, preciseTypes, true)
		fName := capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}
	funcMap := template.FuncMap{
		"impl": func() string { return capitalize(name) + "Impl" },
		"module": func() string {
			if ns == "" {
				return "../.."
			} else {
				return ns
			}
		},
		"rdlruntime":  func() string { return librdl },
		"header":      func() string { return generationHeader(banner) },
		"package":     func() string { return generationPackage(schema, "") },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return flattenedFields(registry, t) },
		"typeRef":     func(t *rdl.Type) string { return makeTypeRef(registry, t, preciseTypes) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"method_sig":  func(r *rdl.Resource) string { return goMethodSignatureImpl(registry, r, preciseTypes) },
		"method_body": func(r *rdl.Resource) string { return goMethodBodyImpl(registry, r, preciseTypes) },
	}
	t := template.Must(template.New("FOO").Funcs(funcMap).Parse(serverMainTemplate))
	err = t.Execute(out, schema)
	if err != nil {
		return err
	}
	out.Flush()
	return nil
}

func goMethodSignatureImpl(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) string {
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	returnSpec := "error"
	//fixme: no content *with* output headers
	if !noContent {
		gtype := goType2(reg, r.Type, false, "", "", precise, true, "")
		returnSpec = "(" + gtype
		if r.Outputs != nil {
			for _, o := range r.Outputs {
				otype := goType2(reg, o.Type, false, "", "", precise, true, "")
				returnSpec += ", " + otype
			}
		}
		returnSpec += ", error)"
	}
	methName, params := goMethodName2(reg, r, precise, "")
	paramSpec := "context *rdl.ResourceContext"
	if len(params) > 0 {
		paramSpec = paramSpec + ", " + strings.Join(params, ", ")
	}
	return capitalize(methName) + "(" + paramSpec + ") " + returnSpec
}

func goMethodBodyImpl(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) string {
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	methName, params := goMethodName2(reg, r, precise, "")
	args := make([]string, 0)
	slots := ""
	for _, sig := range params {
		tmp := strings.Split(sig, " ")
		args = append(args, tmp[0])
		slots = slots + "%v"
	}
	s := "\tfmt.Printf(\"" + methName + "(" + slots + ")\\n\", " + strings.Join(args, ", ") + ")\n"
	if noContent {
		return s + "\treturn &rdl.ResourceError{Code: 501, Message: \"Not Implemented\"}"
	}
	return s + "\treturn nil, &rdl.ResourceError{Code: 501, Message: \"Not Implemented\"}"
}

var serverMainTemplate = `{{header}}

package main

import (
	"net/http"

	{{package}} "{{module}}"
)

func main() {
	endpoint := "localhost:4080"
	url := "http://" + endpoint + "/{{package}}"
	impl := new({{package}}.{{impl}})
	handler := {{package}}.Init(impl, url, impl)
	http.ListenAndServe(endpoint, handler)
}
`

func GenerateGoDaemonImpl(banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {
	name := strings.ToLower(string(schema.Name))
	filepath := outdir + "/" + name + ".go"
	out, file, _, err := outputWriter(filepath, "", ".go")
	if err != nil {
		return err
	}
	if file != nil {
		defer func() {
			file.Close()
			err := goFmt(filepath)
			if err != nil {
				fmt.Println("Warning: could not format go code:", err, filepath)
			}
		}()
	}
	registry := rdl.NewTypeRegistry(schema)
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
		fType := goType(registry, f.Type, optional, f.Items, f.Keys, preciseTypes, true)
		fName := capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}
	funcMap := template.FuncMap{
		"impl": func() string { return capitalize(name) + "Impl" },
		"module": func() string {
			if ns == "" {
				return "../.."
			} else {
				return ns
			}
		},
		"rdlruntime":  func() string { return librdl },
		"header":      func() string { return generationHeader(banner) },
		"package":     func() string { return generationPackage(schema, "") },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return flattenedFields(registry, t) },
		"typeRef":     func(t *rdl.Type) string { return makeTypeRef(registry, t, preciseTypes) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"method_sig":  func(r *rdl.Resource) string { return goMethodSignatureImpl(registry, r, preciseTypes) },
		"method_body": func(r *rdl.Resource) string { return goMethodBodyImpl(registry, r, preciseTypes) },
	}
	t := template.Must(template.New("FOO").Funcs(funcMap).Parse(serverImplTemplate))
	err = t.Execute(out, schema)
	if err != nil {
		return err
	}
	out.Flush()
	return nil
}

var serverImplTemplate = `{{header}}                                                                                                
package {{package}}

import(
	"fmt"

	rdl "{{rdlruntime}}"
)

type {{impl}} struct{}
{{range .Resources}}
func (impl {{impl}}) {{method_sig .}} {
{{method_body .}}
}
{{end}}
//Authenticate - required by the framework. If returning true, you should set context.Principal to a valid object
func (impl *{{impl}}) Authenticate(context *rdl.ResourceContext) bool {
	return false
}

//Authorize - required by the framework. Enforce authorization here.
func (impl *{{impl}}) Authorize(action string, resource string, principal rdl.Principal) (bool, error) {
	return true, nil
}
`

func GenerateGoCLIMain(banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {
	filepath := outdir + "/main.go"
	if fileExists(filepath) {
		return nil
	}
	out, file, _, err := outputWriter(filepath, "", ".go")
	if err != nil {
		return err
	}
	if file != nil {
		defer func() {
			file.Close()
			err := goFmt(filepath)
			if err != nil {
				fmt.Println("Warning: could not format go code:", err, filepath)
			}
		}()
		defer file.Close()
	}
	registry := rdl.NewTypeRegistry(schema)
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
	name := strings.ToLower(string(schema.Name))
	fieldFun := func(f rdl.StructFieldDef) string {
		optional := f.Optional
		fType := goType(registry, f.Type, optional, f.Items, f.Keys, preciseTypes, true)
		fName := capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}
	funcMap := template.FuncMap{
		"impl": func() string { return capitalize(name) + "Impl" },
		"module": func() string {
			if ns == "" {
				return "../.."
			} else {
				return ns
			}
		},
		"rdlruntime":  func() string { return librdl },
		"header":      func() string { return generationHeader(banner) },
		"package":     func() string { return generationPackage(schema, "") },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return flattenedFields(registry, t) },
		"typeRef":     func(t *rdl.Type) string { return makeTypeRef(registry, t, preciseTypes) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"method_sig":  func(r *rdl.Resource) string { return goMethodSignature(registry, r, preciseTypes) },
		"method_body": func(r *rdl.Resource) string { return goMethodBodyImpl(registry, r, preciseTypes) },
	}
	t := template.Must(template.New("FOO").Funcs(funcMap).Parse(cliMainTemplate))
	err = t.Execute(out, schema)
	if err != nil {
		return err
	}
	out.Flush()
	return nil
}

var cliMainTemplate = `{{header}}

package main

import (
	"fmt"

	{{package}} "{{module}}"
)

func main() {
	endpoint := "localhost:4080"
	url := "http://" + endpoint + "/{{package}}"
	client := {{package}}.NewClient(url, nil)
	fmt.Println("client:", client)
	//to do: implement a generic handler for CLI to each API call.
	/*
	The following methods are supported by the client:
{{range .Resources}}
	client.{{method_sig .}}
{{end}}
	*/
}
`
