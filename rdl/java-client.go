// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"bufio"
	"github.com/ardielle/ardielle-go/rdl"
	"strings"
	"text/template"
)

type javaClientGenerator struct {
	registry rdl.TypeRegistry
	schema   *rdl.Schema
	name     string
	writer   *bufio.Writer
	err      error
	banner   string
	ns       string
	base     string
}

// GenerateJavaClient generates the client code to talk to the server
func GenerateJavaClient(banner string, schema *rdl.Schema, outdir string, ns string, base string) error {
	reg := rdl.NewTypeRegistry(schema)
	packageDir, err := javaGenerationDir(outdir, schema, ns)
	if err != nil {
		return err
	}
	cName := capitalize(string(schema.Name))

	out, file, _, err := outputWriter(packageDir, cName, "Client.java")
	if err != nil {
		return err
	}
	gen := &javaClientGenerator{reg, schema, cName, out, nil, banner, ns, base}
	gen.processTemplate(javaClientTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//ResourceException - the throawable wrapper for alternate return types
	out, file, _, err = outputWriter(packageDir, "ResourceException", ".java")
	if err != nil {
		return err
	}
	err = javaGenerateResourceException(schema, out, ns)
	out.Flush()
	file.Close()
	if err != nil {
		return err
	}

	//ResourceError - the default data object for an error
	out, file, _, err = outputWriter(packageDir, "ResourceError", ".java")
	if err != nil {
		return err
	}
	err = javaGenerateResourceError(schema, out, ns)
	out.Flush()
	file.Close()
	return err
}

func (gen *javaClientGenerator) processTemplate(templateSource string) error {
	commentFun := func(s string) string {
		return formatComment(s, 0, 80)
	}
	funcMap := template.FuncMap{
		"header":     func() string { return javaGenerationHeader(gen.banner) },
		"package":    func() string { return javaGenerationPackage(gen.schema, gen.ns) },
		"comment":    commentFun,
		"methodSig":  func(r *rdl.Resource) string { return gen.clientMethodSignature(r) },
		"methodBody": func(r *rdl.Resource) string { return gen.clientMethodBody(r) },
		"name":       func() string { return gen.name },
		"cName":      func() string { return capitalize(gen.name) },
		"lName":      func() string { return uncapitalize(gen.name) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(templateSource))
	return t.Execute(gen.writer, gen.schema)
}

func (gen *javaClientGenerator) resourcePath(r *rdl.Resource) string {
	path := r.Path
	i := strings.Index(path, "?")
	if i >= 0 {
		path = path[0:i]
	}
	return path
}

const javaClientTemplate = `{{header}}
package {{package}};
import com.yahoo.rdl.*;
import javax.ws.rs.client.*;
import javax.ws.rs.*;
import javax.ws.rs.core.*;
import javax.net.ssl.HostnameVerifier;

public class {{cName}}Client {
    Client client;
    WebTarget base;
    String credsHeader;
    String credsToken;

    public {{cName}}Client(String url) {
        client = ClientBuilder.newClient();
        base = client.target(url);
    }

    public {{cName}}Client(String url, HostnameVerifier hostnameVerifier) {
        client = ClientBuilder.newBuilder()
            .hostnameVerifier(hostnameVerifier)
            .build();
        base = client.target(url);
    }

    public void close() {
        client.close();
    }

    public {{cName}}Client setProperty(String name, Object value) {
        client = client.property(name, value);
        return this;
    }

    public {{cName}}Client addCredentials(String header, String token) {
        credsHeader = header;
        credsToken = token;
        return this;
    }
{{range .Resources}}
    {{methodSig .}} {
        {{methodBody .}}
    }
{{end}}
}
`

func (gen *javaClientGenerator) clientMethodSignature(r *rdl.Resource) string {
	reg := gen.registry
	returnType := javaType(reg, r.Type, false, "", "")
	methName, params := javaMethodName(reg, r)
	sparams := ""
	if len(params) > 0 {
		sparams = strings.Join(params, ", ")
	}
	return "public " + returnType + " " + methName + "(" + sparams + ")"
}

func (gen *javaClientGenerator) clientMethodBody(r *rdl.Resource) string {
	reg := gen.registry
	returnType := javaType(reg, r.Type, false, "", "")
	path := r.Path
	s := "WebTarget target = base.path(\"" + path + "\")"
	entityName := ""
	q := ""
	h := ""
	for _, in := range r.Inputs {
		iname := javaName(in.Name)
		if in.PathParam {
			s += "\n            .resolveTemplate(\"" + iname + "\", " + iname + ")"
		} else if in.QueryParam != "" {
			q += "\n        if (" + iname + " != null) {"
			q += "\n            target = target.queryParam(\"" + in.QueryParam + "\", " + iname + ");"
			q += "\n        }"
		} else if in.Header != "" {
			h += "\n        if (" + iname + " != null) {"
			h += "\n            invocationBuilder = invocationBuilder.header(\"" + in.Header + "\", " + iname + ");"
			h += "\n        }"
		} else { //the entity
			entityName = iname
		}
	}
	s += ";"
	if q != "" {
		s += q
	}
	s += "\n        Invocation.Builder invocationBuilder = target.request(\"application/json\");"
	if h != "" {
		s += h
	}
	s += "\n"
	switch r.Method {
	case "PUT", "POST":
		s += "        Response response = invocationBuilder." + strings.ToLower(r.Method) + "(Entity.entity(" + entityName + ", \"application/json\"));\n"
	default:
		s += "        Response response = invocationBuilder." + strings.ToLower(r.Method) + "();\n"
	}
	s += "        int code = response.getStatus();\n"
	s += "        String status = Response.Status.fromStatusCode(code).toString();\n"
	expected := "status.equals(\"" + r.Expected + "\")"
	for _, alt := range r.Alternatives {
		expected += "|| status.equals(\"" + alt + "\")"
	}
	s += "        if (" + expected + ") {\n"
	s += "            return response.readEntity(" + returnType + ".class);\n"
	if r.Exceptions != nil {
		for k, v := range r.Exceptions {
			s += "        } else if (status.equals(\"" + k + "\")) {\n"
			s += "            throw new ResourceException(code, response.readEntity(" + v.Type + ".class));\n"
		}
	}
	s += "        } else {\n"
	s += "            throw new ResourceException(code, response.readEntity(Object.class));\n"
	s += "        }\n"
	return s
}
