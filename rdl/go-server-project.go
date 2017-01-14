package main

import (
	//	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	//	"reflect"
	//	"sort"
	"strings"
	"text/template"

	"github.com/ardielle/ardielle-go/rdl"
)

//func GenerateGoCommand(...) -> generate a go command that uses the go client in the same project.

func GenerateGoServerProject(rdlSrcPath string, banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {

	//1. establish directory structure
	if strings.HasSuffix(outdir, ".go") {
		return fmt.Errorf("Output must be a directory: %q", outdir)
	}
	rdlSrcFile := filepath.Base(rdlSrcPath)
	//rdlSrcDir := filepath.Dir(rdlSrcPath)
	rdlDstDir := filepath.Join(outdir, "rdl")
	rdlDstPath := filepath.Join(rdlDstDir, rdlSrcFile)
	err := os.MkdirAll(rdlDstDir, 0755)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadFile(rdlSrcPath)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(rdlDstPath, data, 0644)
	if err != nil {
		return err
	}
	name := strings.ToLower(string(schema.Name))
	gendir := filepath.Join(outdir, name)
	err = os.MkdirAll(gendir, 0755)
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
	daemondir := filepath.Join(outdir, name+"d")
	err = os.MkdirAll(daemondir, 0755)
	if err != nil {
		return err
	}
	err = GenerateGoServerMain(banner, schema, daemondir, ns, librdl, prefixEnums, preciseTypes, untaggedUnions)
	if err != nil {
		return err
	}
	//		err = GenerateGoClient(banner, schema, dirName, ns, librdl, prefixEnums, preciseTypes)
	//2. generate go model
	//3. generate go server
	//4. generate go client
	//5. generate go command to call client
	//the end result should be a project that can be tested.
	return nil
}

func GenerateGoServerMain(banner string, schema *rdl.Schema, outdir string, ns string, librdl string, prefixEnums bool, preciseTypes bool, untaggedUnions []string) error {
	out, file, _, err := outputWriter(outdir, "main", ".go")
	if err != nil {
		return err
	}
	if file != nil {
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
				return "../" + name
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
		//		"client":      func() string { return name + "Client" },
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
	name := strings.ToLower(string(reg.Name()))
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	returnSpec := "error"
	//fixme: no content *with* output headers
	if !noContent {
		gtype := goType2(reg, r.Type, false, "", "", precise, true, name)
		returnSpec = "(" + gtype
		if r.Outputs != nil {
			for _, o := range r.Outputs {
				otype := goType2(reg, o.Type, false, "", "", precise, true, name)
				returnSpec += ", " + otype
			}
		}
		returnSpec += ", error)"
	}
	methName, params := goMethodName2(reg, r, precise, name)
	paramSpec := "context *rdl.ResourceContext"
	if len(params) > 0 {
		paramSpec = paramSpec + ", " + strings.Join(params, ", ")
	}
	return capitalize(methName) + "(" + paramSpec + ") " + returnSpec
}

func goMethodBodyImpl(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) string {
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	if noContent {
		return "\treturn &rdl.ResourceError{Code: 501, Message: \"Not Implemented\"}"
	}
	return "\treturn nil, &rdl.ResourceError{Code: 501, Message: \"Not Implemented\"}"
}

var serverMainTemplate = `{{header}}

package main

import (
	"net/http"

	{{package}} "{{module}}"
	rdl "{{rdlruntime}}"
)

func main() {
	endpoint := "localhost:4080"
	url := "http://" + endpoint + "/{{package}}"
	impl := new({{impl}})
	handler := {{package}}.Init(impl, url, impl)
	http.ListenAndServe(endpoint, handler)
}

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
