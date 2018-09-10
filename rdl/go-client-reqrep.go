package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"

	"github.com/ardielle/ardielle-go/gen/gomodel"
	"github.com/ardielle/ardielle-go/rdl"
)

type reqRepClientGenerator struct {
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

const rrClientTemplate = `{{header}}

package {{package}}

import (
	"bytes"
	"encoding/json"
	"fmt"
	rdl "{{rdlruntime}}"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"context"
)

var _ = json.Marshal
var _ = fmt.Printf
var _ = rdl.BaseTypeAny
var _ = ioutil.NopCloser

type {{client}} struct {
	URL         string
	Transport   http.RoundTripper
	CredsHeader *string
	CredsToken  *string
	Timeout     time.Duration
}

// NewClient creates and returns a new HTTP client object for the {{.Name}} service
func NewClient(url string, transport http.RoundTripper) {{client}} {
	return {{client}}{url, transport, nil, nil, 0}
}

// AddCredentials adds the credentials to the client for subsequent requests.
func (client *{{client}}) AddCredentials(header string, token string) {
	client.CredsHeader = &header
	client.CredsToken = &token
}

func (client {{client}}) getClient() *http.Client {
	var c *http.Client
	if client.Transport != nil {
		c = &http.Client{Transport: client.Transport}
	} else {
		c = &http.Client{}
	}
	if client.Timeout > 0 {
		c.Timeout = client.Timeout
	}
	return c
}

func (client {{client}}) addAuthHeader(req *http.Request) {
	if client.CredsHeader != nil && client.CredsToken != nil {
		if strings.HasPrefix(*client.CredsHeader, "Cookie.") {
			req.Header.Add("Cookie", (*client.CredsHeader)[7:]+"="+*client.CredsToken)
		} else {
			req.Header.Add(*client.CredsHeader, *client.CredsToken)
		}
	}
}

func (cl {{client}}) httpDo(ctx context.Context, req *http.Request) (*http.Response, error) {
   client := cl.getClient()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
	   // get context error if there is one
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
		}
	}
	return resp, err
}


func (client {{client}}) httpGet(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
	return client.httpDo(ctx, req)
}

func (client {{client}}) httpDelete(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
	return client.httpDo(ctx, req)
}

func (client {{client}}) httpPut(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error) {
	var contentReader io.Reader
	if body != nil {
		contentReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest("PUT", url, contentReader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-type", "application/json")
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
   return client.httpDo(ctx, req)
}

func (client {{client}}) httpPost(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error) {
	var contentReader io.Reader
	if body != nil {
		contentReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest("POST", url, contentReader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-type", "application/json")
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
   return client.httpDo(ctx, req)
}

func (client {{client}}) httpPatch(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error) {
	var contentReader io.Reader
	if body != nil {
		contentReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest("PATCH", url, contentReader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-type", "application/json")
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
   return client.httpDo(ctx, req)
}

func (client {{client}}) httpOptions(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error) {
	var contentReader io.Reader = nil
	if body != nil {
		contentReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest("OPTIONS", url, contentReader)
	if err != nil {
		return nil, err
	}
	if contentReader != nil {
		req.Header.Add("Content-type", "application/json")
	}
	client.addAuthHeader(req)
    if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
   return client.httpDo(ctx, req)
}

func appendHeader(headers map[string]string, name, val string) map[string]string {
   if val == "" {
      return headers
   }
   if headers == nil {
      headers = make(map[string]string)
   }
   headers[name] = val
   return headers
}

func encodeStringParam(name string, val string, def string) string {
	if val == def {
		return ""
	}
	return "&" + name + "=" + url.QueryEscape(val)
}
func encodeBoolParam(name string, b bool, def bool) string {
	if b == def {
		return ""
	}
	return fmt.Sprintf("&%s=%v", name, b)
}
func encodeInt8Param(name string, i int8, def int8) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.Itoa(int(i))
}
func encodeInt16Param(name string, i int16, def int16) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.Itoa(int(i))
}
func encodeInt32Param(name string, i int32, def int32) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.Itoa(int(i))
}
func encodeInt64Param(name string, i int64, def int64) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.FormatInt(i, 10)
}
func encodeFloat32Param(name string, i float32, def float32) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.FormatFloat(float64(i), 'g', -1, 32)
}
func encodeFloat64Param(name string, i float64, def float64) string {
	if i == def {
		return ""
	}
	return "&" + name + "=" + strconv.FormatFloat(i, 'g', -1, 64)
}
func encodeOptionalEnumParam(name string, e interface{}) string {
	if e == nil {
		return "\"\""
	}
	return fmt.Sprintf("&%s=%v", name, e)
}
func encodeOptionalBoolParam(name string, b *bool) string {
	if b == nil {
		return ""
	}
	return fmt.Sprintf("&%s=%v", name, *b)
}
func encodeOptionalInt32Param(name string, i *int32) string {
	if i == nil {
		return ""
	}
	return "&" + name + "=" + strconv.Itoa(int(*i))
}
func encodeOptionalInt64Param(name string, i *int64) string {
	if i == nil {
		return ""
	}
	return "&" + name + "=" + strconv.Itoa(int(*i))
}
func encodeParams(objs ...string) string {
	s := strings.Join(objs, "&")
	if s == "" {
		return s
	}
	return "?" + s[1:]
}
{{range methods}}
type {{.RequestName}} struct {
{{range .Inputs}}   {{.Name}} {{.TypeName}}
{{end}}
}

type {{.ResponseName}} struct {
{{range .Outputs}}   {{.Name}} {{.TypeName}}
{{end}}
}

func (client {{client}}) {{.Signature}} {
	var response {{.ResponseName}}
	var headers map[string]string
	{{range .Inputs}}{{if (ne .Header  "")}}
	    headers = appendHeader(headers, "{{.Header}}", req.{{.Name}})
   {{end}}{{end}}
   url := client.URL + {{.URLExpression}}
   {{.Invocation}}
   if err != nil {
       return nil, err
   }
   defer resp.Body.Close()
   switch resp.StatusCode {
   {{.ResponseCases}}
   default:
      var errobj rdl.ResourceError
	  err = json.NewDecoder(resp.Body).Decode(&errobj)
	  if err != nil {
		  return nil, err
	  }
	   if errobj.Code == 0 {
	      errobj.Code = resp.StatusCode
	   }
	   if errobj.Message == "" {
	      errobj.Message = string(outputBytes)
	   }
	   return nil, errobj
   }
   {{.ParseOutputHeaders}}
   return &response, nil
	//end loop
}
{{end}}`

func (gen *reqRepClientGenerator) emitClient() error {
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

	methodFunc := func() []*reqRepMethod {
		output := make([]*reqRepMethod, 0, len(gen.schema.Resources))
		for _, r := range gen.schema.Resources {
			output = append(output, gen.convertResource(gen.registry, r, gen.precise))
		}
		return output
	}

	funcMap := template.FuncMap{
		"rdlruntime": func() string { return gen.librdl },
		"header":     func() string { return generationHeader(gen.banner) },
		"package":    func() string { return generationPackage(gen.schema, gen.ns) },
		"basename":   basenameFunc,
		"comment":    commentFun,
		"methods":    methodFunc,
		"client":     func() string { return gen.name + "Client" },
	}
	t := template.Must(template.New("REQREP_CLIENT_TEMPLATE").Funcs(funcMap).Parse(rrClientTemplate))
	var output bytes.Buffer
	if err := t.Execute(gen.writer, gen.schema); err != nil {
		return err
	} else if data, err := format.Source(output.Bytes()); err != nil {
		return err
	} else if _, err := gen.writer.Write(data); err != nil {
		return err
	}
	gen.writer.Flush()
	return nil
}

type reqRepVar struct {
	Name                      string
	TypeName                  string
	ArrayType                 bool
	EncodeParameterExpression string
	QueryParameter            string
	PathParameter             bool
	Header                    string
	Comment                   string
}

func (r *reqRepVar) IsBody() bool {
	return r.QueryParameter == "" && !r.PathParameter && r.Header == ""
}

type reqRepMethod struct {
	Resource        *rdl.Resource
	Name            string
	Method          string
	PathExpression  []string
	QueryExpression []string
	Comment         string
	RequestName     string
	ResponseName    string
	Inputs          []*reqRepVar
	Outputs         []*reqRepVar
}

func (m *reqRepMethod) Signature() string {
	return fmt.Sprintf("%s(ctx context.Context, req *%s) (*%s, error)", m.Name, m.RequestName, m.ResponseName)
}

func (m *reqRepMethod) URLExpression() string {
	// TODO: include Query Parameters
	exprs := m.PathExpression[:]
	if len(m.QueryExpression) > 0 {
		exprs = append(exprs, "encodeParams("+strings.Join(m.QueryExpression, ",")+")")
	}
	return fmt.Sprintf("fmt.Sprint(%s)", strings.Join(exprs, ","))
}

func (m *reqRepMethod) Invocation() string {
	method := capitalize(strings.ToLower(m.Method))
	findBodyParam := func() string {
		bodyParam := ""
		for _, in := range m.Inputs {
			if in.IsBody() {
				bodyParam = "req." + in.Name
				break
			}
		}
		return bodyParam
	}
	var s string
	switch method {
	case "Get", "Delete":
		s = "\tresp, err := client.http" + method + "(ctx, url, headers)\n"
	case "Put", "Post", "Patch":
		bodyParam := findBodyParam()
		if bodyParam == "" {
			s = "\tvar contentBytes []byte\n"
		} else {
			s = "\tcontentBytes, err := json.Marshal(" + bodyParam + ")\n"
			s += "\tif err != nil {\n\t\treturn nil, err\n\t}\n"
		}
		s += "\tresp, err := client.http" + method + "(ctx, url, headers, contentBytes)\n"
	case "Options":
		bodyParam := findBodyParam()
		if bodyParam != "" {
			s = "\tcontentBytes, err := json.Marshal(" + bodyParam + ")\n"
			s += "\tif err != nil {\n\t\treturn nil, err\n\t}\n"
			s += "\tresp, err := client.http" + method + "(ctx, url, headers, contentBytes)\n"
		} else {
			s = "\tresp, err := client.http" + method + "(ctx, url, headers, nil)\n"
		}
	}
	return s
}

type codeGen struct {
	buf bytes.Buffer
}

func (c *codeGen) Code() ([]byte, error) {
	return c.buf.Bytes(), nil
}

func (c *codeGen) CodeString() (string, error) {
	if b, err := c.Code(); err != nil {
		return "", err
	} else {
		return string(b), nil
	}

}

func (c *codeGen) Block(lines string) {
	c.Println(lines)
}

func (c *codeGen) Println(line string) {
	c.buf.WriteString(line + "\n")
}

func (c *codeGen) Printf(format string, args ...interface{}) {
	c.Println(fmt.Sprintf(format, args...))
}

func (m *reqRepMethod) ResponseCases() (string, error) {
	var s codeGen
	r := m.Resource
	expected := make(map[string]bool)
	expected[r.Expected] = true
	for _, e := range r.Alternatives {
		expected[e] = true
	}
	for expect, _ := range expected {
		code := rdl.StatusCode(expect)
		s.Printf("case %s:", code)
		switch expect {
		case "NO_CONTENT":
			fallthrough
		case "NOT_MODIFIED":
			// no body
		default:
			// decode body
			s.Println("if err := json.NewDecoder(resp.Body).Decode(&response.Body); err != nil {")
			s.Println("   return nil, err")
			s.Println("}")
		}
	}
	return s.CodeString()
}

func (m *reqRepMethod) ParseOutputHeaders() (string, error) {
	//here, define the output headers
	var code codeGen
	for _, out := range m.Outputs {
		if out.Header != "" {
			if out.TypeName != "string" {
				code.Printf("response.%s = %s(resp.Header.Get(rdl.FoldHttpHeaderName(%q))", out.Name, out.TypeName, out.Header)
			} else {
				code.Printf("response.%s = resp.Header.Get(rdl.FoldHttpHeaderName(%q)", out.Name, out.Header)
			}
		}
	}
	return code.CodeString()
}

func (rr *reqRepClientGenerator) convertInput(reg rdl.TypeRegistry, v *rdl.ResourceInput, precise bool) *reqRepVar {
	if v.Context != "" { //legacy field, to be removed
		return nil
	}
	res := &reqRepVar{
		Name:           capitalize(goName(string(v.Name))),
		Comment:        v.Comment,
		QueryParameter: v.QueryParam,
		PathParameter:  v.PathParam,
		Header:         v.Header,
		TypeName:       gomodel.GoType2(reg, v.Type, v.Optional, "", "", precise, true, ""),
	}
	valueExpr := fmt.Sprintf("req.%s", res.Name)
	if reg.IsArrayTypeName(v.Type) && res.QueryParameter != "" {
		res.EncodeParameterExpression = fmt.Sprintf("encodeListParam(\"%s\", %s)", res.QueryParameter, valueExpr)
	} else if res.QueryParameter != "" {
		baseType := reg.BaseTypeName(v.Type)
		if v.Optional && baseType != "String" {
			res.EncodeParameterExpression = "encodeOptional" + string(baseType) + "Param(\"" + res.QueryParameter + "\", " + valueExpr + ")"
		} else {
			def := goLiteral(v.Default, string(baseType))
			if baseType == "Enum" {
				def = "\"" + def + "\""
				res.EncodeParameterExpression = "encodeStringParam(\"" + res.QueryParameter + "\", " + valueExpr + ".String(), " + def + ")"
			} else {
				res.EncodeParameterExpression = "encode" + string(baseType) + "Param(\"" + res.QueryParameter + "\", " + strings.ToLower(string(baseType)) + "(" + valueExpr + "), " + def + ")"
			}
		}
	}

	return res
}

func (rr *reqRepClientGenerator) convertOutput(reg rdl.TypeRegistry, v *rdl.ResourceOutput, precise bool) *reqRepVar {
	return &reqRepVar{
		Name:      capitalize(goName(string(v.Name))),
		Header:    v.Header,
		Comment:   v.Comment,
		ArrayType: reg.IsArrayTypeName(v.Type),
		TypeName:  gomodel.GoType2(reg, v.Type, v.Optional, "", "", precise, true, ""),
	}
}

func (rr *reqRepClientGenerator) convertResource(reg rdl.TypeRegistry, r *rdl.Resource, precise bool) *reqRepMethod {
	var method reqRepMethod
	bodyType := string(gomodel.SafeTypeVarName(r.Type))
	for _, v := range r.Inputs {
		if input := rr.convertInput(reg, v, precise); input != nil {
			method.Inputs = append(method.Inputs, input)
			if input.IsBody() {
				bodyType = input.TypeName
			}
		}
	}
	noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	if !noContent {
		method.Outputs = append(method.Outputs, &reqRepVar{
			Name:     "Body",
			TypeName: gomodel.GoType(reg, r.Type, false, "", "", precise, true),
		})
	}
	for _, v := range r.Outputs {
		if input := rr.convertOutput(reg, v, precise); input != nil {
			method.Inputs = append(method.Inputs, input)
		}
	}
	method.Resource = r
	method.Name = string(r.Name)
	method.Method = r.Method
	method.Comment = r.Comment
	if method.Name == "" {
		methodTypeName := bodyType
		if strings.HasPrefix(methodTypeName, "*") {
			methodTypeName = methodTypeName[1:]
		}
		method.Name = capitalize(strings.ToLower(string(r.Method)) + methodTypeName)
	} else {
		method.Name = capitalize(method.Name)
	}
	method.RequestName = method.Name + "Request"
	method.ResponseName = method.Name + "Response"
	findPathVariable := func(name string) *reqRepVar {
		for _, input := range method.Inputs {
			if input.PathParameter && strings.EqualFold(input.Name, name) {
				return input
			}
		}
		return nil
	}
	addPathLiteral := func(s string) {
		if s == "" {
			return
		}
		method.PathExpression = append(method.PathExpression, fmt.Sprintf("%#v", s))
	}
	addPathVariable := func(name string) {
		v := findPathVariable(name)
		if v == nil {
			// what to do? drop for now.
			return
		}
		method.PathExpression = append(method.PathExpression, fmt.Sprintf("req.%s", v.Name))
	}
	addQueryParameter := func(v *reqRepVar) {
		method.QueryExpression = append(method.QueryExpression, v.EncodeParameterExpression)
	}
	chunks := strings.Split(r.Path, "{")
	for _, chunk := range chunks {
		closeBrace := strings.Index(chunk, "}")
		rest := chunk
		if closeBrace >= 0 {
			varName := chunk[:closeBrace]
			rest = chunk[closeBrace+1:]
			addPathVariable(varName)
		}
		addPathLiteral(rest)
	}
	for _, v := range method.Inputs {
		if v.QueryParameter != "" {
			addQueryParameter(v)
		}
	}

	return &method
}
