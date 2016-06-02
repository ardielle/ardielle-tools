// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"bufio"
	"fmt"
	"github.com/ardielle/ardielle-go/rdl"
	"log"
	"os"
	"strings"
	"text/template"
)

type javaServerGenerator struct {
	registry rdl.TypeRegistry
	schema   *rdl.Schema
	name     string
	writer   *bufio.Writer
	err      error
	banner   string
	ns       string
	async    bool
	base     string
}

// GenerateJavaServer generates the server code for the RDL-defined service
func GenerateJavaServer(banner string, schema *rdl.Schema, outdir string, ns string, base string) error {
	reg := rdl.NewTypeRegistry(schema)
	packageDir, err := javaGenerationDir(outdir, schema, ns)
	if err != nil {
		return err
	}
	cName := capitalize(string(schema.Name))

	async := false
	for _, r := range schema.Resources {
		if r.Async != nil && *r.Async {
			async = true
			break
		}
	}

	//FooHandler interface
	out, file, _, err := outputWriter(packageDir, cName, "Handler.java")
	if err != nil {
		return err
	}
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, async, base}
	gen.processTemplate(javaServerHandlerTemplate)
	out.Flush()
	file.Close()

	for _, r := range schema.Resources {
		if r.Async != nil && *r.Async {
			javaServerMakeAsyncResultModel(banner, schema, reg, outdir, r, ns, base)
		} else if len(r.Outputs) > 0 {
			javaServerMakeResultModel(banner, schema, reg, outdir, r, ns, base)
		}
	}

	//ResourceContext interface
	s := "ResourceContext"
	out, file, _, err = outputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, async, base}
	gen.processTemplate(javaServerContextTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//FooResources Jax-RS glue
	out, file, _, err = outputWriter(packageDir, cName, "Resources.java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, async, base}
	gen.processTemplate(javaServerTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//Note: to enable jackson's pretty printer:
	//import com.fasterxml.jackson.jaxrs.annotation.JacksonFeatures;
	//import com.fasterxml.jackson.databind.SerializationFeature;
	//for each resource, add this annotation:
	//   @JacksonFeatures(serializationEnable =  { SerializationFeature.INDENT_OUTPUT })

	//FooServer - an optional server wrapper that sets up Jetty9/Jersey2 to run Foo
	out, file, _, err = outputWriter(packageDir, cName, "Server.java")
	if err != nil {
		return err
	}
	gen = &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, async, base}
	gen.processTemplate(javaServerInitTemplate)
	out.Flush()
	file.Close()
	if gen.err != nil {
		return gen.err
	}

	//ResourceException - the throawable wrapper for alternate return types
	s = "ResourceException"
	out, file, _, err = outputWriter(packageDir, s, ".java")
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
	s = "ResourceError"
	out, file, _, err = outputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	err = javaGenerateResourceError(schema, out, ns)
	out.Flush()
	file.Close()
	return err
}

func javaServerMakeAsyncResultModel(banner string, schema *rdl.Schema, reg rdl.TypeRegistry, outdir string, r *rdl.Resource, ns string, base string) error {
	cName := capitalize(string(r.Type))
	packageDir, err := javaGenerationDir(outdir, schema, ns)
	if err != nil {
		return err
	}
	methName, _ := javaMethodName(reg, r)
	s := capitalize(methName) + "Result"
	out, file, _, err := outputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	rType := javaType(reg, rdl.TypeRef(r.Type), false, "", "")
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, true, base}
	funcMap := template.FuncMap{
		"header":     func() string { return javaGenerationHeader(gen.banner) },
		"package":    func() string { return javaGenerationPackage(gen.schema, ns) },
		"openBrace":  func() string { return "{" },
		"name":       func() string { return uncapitalize(string(safeTypeVarName(r.Type))) },
		"cName":      func() string { return string(rType) },
		"resultArgs": func() string { return gen.resultArgs(r) },
		"resultSig":  func() string { return gen.resultSignature(r) },
		"rName": func() string {
			return capitalize(s)
		},
		"pathParamsKey":    func() string { return gen.makePathParamsKey(r) },
		"pathParamsDecls":  func() string { return gen.makePathParamsDecls(r) },
		"pathParamsSig":    func() []string { return gen.makePathParamsSig(r) },
		"pathParamsAssign": func() string { return gen.makePathParamsAssign(r) },
		"headerParams":     func() []string { return gen.makeHeaderParams(r) },
		"headerParamsSig":  func() []string { return gen.makeHeaderParamsSig(r) },
		"headerAssign":     func() string { return gen.makeHeaderAssign(r) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(javaServerAsyncResultTemplate))
	err = t.Execute(gen.writer, gen.schema)
	out.Flush()
	file.Close()
	return err
}

func javaServerMakeResultModel(banner string, schema *rdl.Schema, reg rdl.TypeRegistry, outdir string, r *rdl.Resource, ns string, base string) error {
	cName := capitalize(string(r.Type))
	packageDir, err := javaGenerationDir(outdir, schema, ns)
	if err != nil {
		return err
	}
	methName, _ := javaMethodName(reg, r)
	s := capitalize(methName) + "Result"
	out, file, _, err := outputWriter(packageDir, s, ".java")
	if err != nil {
		return err
	}
	rType := javaType(reg, rdl.TypeRef(r.Type), false, "", "")
	gen := &javaServerGenerator{reg, schema, cName, out, nil, banner, ns, false, base}
	funcMap := template.FuncMap{
		"header":     func() string { return javaGenerationHeader(gen.banner) },
		"package":    func() string { return javaGenerationPackage(gen.schema, ns) },
		"openBrace":  func() string { return "{" },
		"name":       func() string { return uncapitalize(string(safeTypeVarName(r.Type))) },
		"cName":      func() string { return string(rType) },
		"resultArgs": func() string { return gen.resultArgs(r) },
		"resultSig":  func() string { return gen.resultSignature(r) },
		"rName": func() string {
			return capitalize(s)
		},
		"pathParamsKey":    func() string { return gen.makePathParamsKey(r) },
		"pathParamsDecls":  func() string { return gen.makePathParamsDecls(r) },
		"pathParamsSig":    func() []string { return gen.makePathParamsSig(r) },
		"pathParamsAssign": func() string { return gen.makePathParamsAssign(r) },
		"headerParams":     func() []string { return gen.makeHeaderParams(r) },
		"headerParamsSig":  func() []string { return gen.makeHeaderParamsSig(r) },
		"headerAssign":     func() string { return gen.makeHeaderAssign(r) },
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(javaServerResultTemplate))
	err = t.Execute(gen.writer, gen.schema)
	out.Flush()
	file.Close()
	return err
}

func (gen *javaServerGenerator) resultSignature(r *rdl.Resource) string {
	vName := string(safeTypeVarName(r.Type)) + "Object"
	s := javaType(gen.registry, r.Type, false, "", "") + " " + vName
	for _, out := range r.Outputs {
		s += ", " + javaType(gen.registry, out.Type, false, "", "") + " " + javaName(out.Name)
	}
	return s
}

func (gen *javaServerGenerator) resultArgs(r *rdl.Resource) string {
	vName := string(safeTypeVarName(r.Type)) + "Object"
	//void?
	for _, out := range r.Outputs {
		vName += ", " + javaName(out.Name)
	}
	return vName

}

func (gen *javaServerGenerator) makePathParamsKey(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				if s == "" && gen.registry.IsStringTypeName(in.Type) {
					s = javaName(in.Name)
				} else {
					s += " + \".\" + " + javaName(in.Name)
				}
			}
		}
	}
	if s == "" {
		s = "\"" + r.Path + "\"" //If there are no input params, make the path as the key
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsDecls(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jtype := javaType(gen.registry, in.Type, false, "", "")
				s += "\n    private " + jtype + " " + javaName(in.Name) + ";"
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsSig(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jtype := javaType(gen.registry, in.Type, false, "", "")
				s = append(s, jtype+" "+javaName(in.Name))
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsArgs(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				s = append(s, javaName(in.Name))
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makePathParamsAssign(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, in := range r.Inputs {
			if in.PathParam {
				jname := javaName(in.Name)
				s += "\n        this." + jname + " = " + jname + ";"
			}
		}
	}
	return s
}

func (gen *javaServerGenerator) makeHeaderParams(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, out := range r.Outputs {
			s = append(s, javaName(out.Name))
		}
	}
	return s
}

func (gen *javaServerGenerator) makeHeaderParamsSig(r *rdl.Resource) []string {
	s := make([]string, 0)
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, out := range r.Outputs {
			jtype := javaType(gen.registry, out.Type, false, "", "")
			s = append(s, jtype+" "+javaName(out.Name))
		}
	}
	return s
}

func (gen *javaServerGenerator) makeHeaderAssign(r *rdl.Resource) string {
	s := ""
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		for _, out := range r.Outputs {
			jname := javaName(out.Name)
			s += fmt.Sprintf("\n            .header(%q, %s)", out.Header, jname)
		}
	}
	return s
}

const javaServerHandlerTemplate = `{{header}}
package {{package}};
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import com.yahoo.rdl.*;

//
// {{cName}}Handler is the interface that the service implementation must implement
//
public interface {{cName}}Handler {{openBrace}} {{range .Resources}}
    {{methodSig .}};{{end}}
    public ResourceContext newResourceContext(HttpServletRequest request, HttpServletResponse response);
}
`
const javaServerResultTemplate = `{{header}}
package {{package}};
import com.yahoo.rdl.*;
import javax.ws.rs.core.Response;
import javax.ws.rs.WebApplicationException;

public final class {{rName}} {
    private ResourceContext context;{{pathParamsDecls}}
    private int code; //normal result

    {{rName}}(ResourceContext context) {
        this.context = context;
        this.code = 0;
    }

    public boolean isAsync() {
        return false;
    }

    public void done(int code, {{cName}} {{name}}{{range headerParamsSig}}, {{.}}{{end}}) {
        Response resp = Response.status(code).entity({{name}}){{headerAssign}}
            .build();
        throw new WebApplicationException(resp);
    }

    public void done(int code) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)){{range headerParams}}, ""{{end}});
    }

    public void done(int code{{range headerParamsSig}}, {{.}}{{end}}) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)){{range headerParams}}, {{.}}{{end}});
    }

    public void done(int code, Object entity{{range headerParamsSig}}, {{.}}{{end}}) {
        this.code = code;
        //to do: check if the exception is declared, and that the entity is of the declared type
        WebApplicationException err = new WebApplicationException(Response.status(code).entity(entity){{headerAssign}}
          .build());
        throw err; //not optimal
    }

}
`
const javaServerAsyncResultTemplate = `{{header}}
package {{package}};
import java.util.Collection;
import java.util.Map;
import java.util.HashMap;
import javax.ws.rs.container.AsyncResponse;
import javax.ws.rs.container.TimeoutHandler;
import javax.ws.rs.core.Response;
import javax.ws.rs.WebApplicationException;
import java.util.concurrent.TimeUnit;
import com.yahoo.rdl.*;

public final class {{rName}} implements TimeoutHandler {
    private AsyncResponse async;
    private ResourceContext context;{{pathParamsDecls}}
    private int code; //normal result
    private int timeoutCode;

    {{rName}}(ResourceContext context, {{range pathParamsSig}}{{.}}, {{end}}AsyncResponse async) {
        this.context = context;
        this.async = async;{{pathParamsAssign}}
        this.code = 0;
        this.timeoutCode = 0;
    }

    public boolean isAsync() {
        return async != null;
    }

    public void done(int code, {{cName}} {{name}}{{range headerParamsSig}}, {{.}}{{end}}) {
        Response resp = Response.status(code).entity({{name}}){{headerAssign}}
            .build();
        if (async == null) {
            throw new WebApplicationException(resp);
        }
        async.resume(resp);
    }

    public void done(int code) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)){{range headerParams}}, ""{{end}});
    }

    public void done(int code{{range headerParamsSig}}, {{.}}{{end}}) {
        done(code, new ResourceError().code(code).message(ResourceException.codeToString(code)){{range headerParams}}, {{.}}{{end}});
    }

    public void done(int code, Object entity{{range headerParamsSig}}, {{.}}{{end}}) {
        this.code = code;
        //to do: check if the exception is declared, and that the entity is of the declared type
        WebApplicationException err = new WebApplicationException(Response.status(code).entity(entity){{headerAssign}}
            .build());
        if (async == null) {
            throw err; //not optimal
        }
        async.resume(err);
    }

    private static Map<String, Map<AsyncResponse, {{rName}}>> waiters = new HashMap<String, Map<AsyncResponse, {{rName}}>>();
    
    public void wait({{range pathParamsSig}}{{.}}, {{end}}int timeout, int normalStatus, int timeoutStatus) {
        async.setTimeout(timeout, TimeUnit.SECONDS);
        this.code = normalStatus;
        this.timeoutCode = timeoutStatus;
        synchronized (waiters) {
            Map<AsyncResponse, {{rName}}> m = waiters.get({{pathParamsKey}});
            if (m == null) {
                m = new HashMap<AsyncResponse, {{rName}}>();
                waiters.put({{pathParamsKey}}, m);
            }
            m.put(async, this);
            async.setTimeoutHandler(this);
        }
    }

    public void handleTimeout(AsyncResponse ar) {
        //the timeout is per-request.
        {{rName}} result = null;
        synchronized (waiters) {
            Map<AsyncResponse, {{rName}}> m = waiters.get({{pathParamsKey}});
            if (m != null) {
                result = m.remove(ar);
            }
        }
        if (result != null) {
            result.done(timeoutCode);
        }
    }

    //this get called to notifyAll of changed state
    public static void notify({{range pathParamsSig}}{{.}}, {{end}}{{resultSig}}) {
        Collection<{{rName}}> results = null;
        synchronized (waiters) {
            Map<AsyncResponse, {{rName}}> m = waiters.remove({{pathParamsKey}});
            if (m != null) {
                results = m.values();
            }
        }
        if (results != null) {
            for ({{rName}} result : results) {
                result.done(result.code, {{resultArgs}});
            }
        }
    }
}
`

const javaServerContextTemplate = `{{header}}
package {{package}};
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;

//
// ResourceContext
//
public interface ResourceContext {
    public HttpServletRequest request();
    public HttpServletResponse response();
    public void authenticate();
    public void authorize(String action, String resource, String trustedDomain);
}
`

const javaServerInitTemplate = `{{header}}
package {{package}};
import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.servlet.ServletContextHandler;
import org.eclipse.jetty.servlet.ServletHolder;
import org.glassfish.hk2.utilities.binding.AbstractBinder;
import org.glassfish.jersey.server.ResourceConfig;
import org.glassfish.jersey.servlet.ServletContainer;

public class {{cName}}Server {
    {{cName}}Handler handler;

    public {{cName}}Server({{cName}}Handler handler) {
        this.handler = handler;
    }

    public void run(int port) {
        try {
            Server server = new Server(port);
            ServletContextHandler handler = new ServletContextHandler();
            handler.setContextPath("");
            ResourceConfig config = new ResourceConfig({{cName}}Resources.class).register(new Binder());
            handler.addServlet(new ServletHolder(new ServletContainer(config)), "/*");
            server.setHandler(handler);
            server.start();
            server.join();
        } catch (Exception e) {
            System.err.println("*** " + e);
        }
    }

    class Binder extends AbstractBinder {
        @Override
        protected void configure() {
            bind(handler).to({{cName}}Handler.class);
        }
    }
}
`

const javaServerTemplate = `{{header}}
package {{package}};
import com.yahoo.rdl.*;
import javax.ws.rs.*;
import javax.ws.rs.core.*;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import javax.inject.Inject;{{asyncImports}}

@Path("{{rootPath}}")
public class {{cName}}Resources {
{{range .Resources}}
    @{{uMethod .}}
    @Path("{{methodPath .}}")
    {{handlerSig .}} {{openBrace}}
{{handlerBody .}}    }
{{end}}

    WebApplicationException typedException(int code, ResourceException e, Class<?> eClass) {
        Object data = e.getData();
        Object entity = eClass.isInstance(data) ? data : null;
        if (entity != null) {
            return new WebApplicationException(Response.status(code).entity(entity).build());
        } else {
            return new WebApplicationException(code);
        }
    }

    @Inject private {{cName}}Handler delegate;
    @Context private HttpServletRequest request;
    @Context private HttpServletResponse response;
    
}
`

func makeJavaTypeRef(reg rdl.TypeRegistry, t *rdl.Type) string {
	switch t.Variant {
	case rdl.TypeVariantAliasTypeDef:
		typedef := t.AliasTypeDef
		return javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantStringTypeDef:
		typedef := t.StringTypeDef
		return javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantNumberTypeDef:
		typedef := t.NumberTypeDef
		return javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantArrayTypeDef:
		typedef := t.ArrayTypeDef
		return javaType(reg, typedef.Type, false, typedef.Items, "")
	case rdl.TypeVariantMapTypeDef:
		typedef := t.MapTypeDef
		return javaType(reg, typedef.Type, false, typedef.Items, typedef.Keys)
	case rdl.TypeVariantStructTypeDef:
		typedef := t.StructTypeDef
		return javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantEnumTypeDef:
		typedef := t.EnumTypeDef
		return javaType(reg, typedef.Type, false, "", "")
	case rdl.TypeVariantUnionTypeDef:
		return "Object" //fix
	}
	return "?" //never happens
}

func (gen *javaServerGenerator) processTemplate(templateSource string) error {
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
		fType := javaType(gen.registry, f.Type, optional, f.Items, f.Keys)
		fName := capitalize(string(f.Name))
		option := ""
		if optional {
			option = ",omitempty"
		}
		fAnno := "`json:\"" + string(f.Name) + option + "\"`"
		return fmt.Sprintf("%s %s%s", fName, fType, fAnno)
	}
	funcMap := template.FuncMap{
		"header":      func() string { return javaGenerationHeader(gen.banner) },
		"package":     func() string { return javaGenerationPackage(gen.schema, gen.ns) },
		"openBrace":   func() string { return "{" },
		"field":       fieldFun,
		"flattened":   func(t *rdl.Type) []*rdl.StructFieldDef { return flattenedFields(gen.registry, t) },
		"typeRef":     func(t *rdl.Type) string { return makeJavaTypeRef(gen.registry, t) },
		"basename":    basenameFunc,
		"comment":     commentFun,
		"uMethod":     func(r *rdl.Resource) string { return strings.ToUpper(r.Method) },
		"methodSig":   func(r *rdl.Resource) string { return gen.serverMethodSignature(r) },
		"handlerSig":  func(r *rdl.Resource) string { return gen.handlerSignature(r) },
		"handlerBody": func(r *rdl.Resource) string { return gen.handlerBody(r) },
		"client":      func() string { return gen.name + "Client" },
		"server":      func() string { return gen.name + "Server" },
		"name":        func() string { return gen.name },
		"cName":       func() string { return capitalize(gen.name) },
		"methodName":  func(r *rdl.Resource) string { return strings.ToLower(string(r.Method)) + string(r.Type) + "Handler" }, //?
		"methodPath":  func(r *rdl.Resource) string { return gen.resourcePath(r) },
		"rootPath":    func() string { return javaGenerationRootPath(gen.schema, gen.base) },
		"rName": func(r *rdl.Resource) string {
			return capitalize(strings.ToLower(string(r.Method))) + string(r.Type) + "Result"
		},
		"asyncImports": func() string {
			if gen.async {
				return "\nimport javax.ws.rs.container.AsyncResponse;\nimport javax.ws.rs.container.Suspended;"
			}
			return ""
		},
	}
	t := template.Must(template.New(gen.name).Funcs(funcMap).Parse(templateSource))
	return t.Execute(gen.writer, gen.schema)
}

func (gen *javaServerGenerator) resourcePath(r *rdl.Resource) string {
	path := r.Path
	i := strings.Index(path, "?")
	if i >= 0 {
		path = path[0:i]
	}
	return path
}

func (gen *javaServerGenerator) handlerBody(r *rdl.Resource) string {
	async := r.Async != nil && *r.Async
	resultWrapper := len(r.Outputs) > 0 || async
	returnType := "void"
	if !resultWrapper {
		returnType = javaType(gen.registry, r.Type, false, "", "")
	}
	s := "        try {\n"
	s += "            ResourceContext context = this.delegate.newResourceContext(this.request, this.response);\n"
	var fargs []string
	bodyName := ""
	if r.Auth != nil {
		if r.Auth.Authenticate {
			s += "            context.authenticate();\n"
		} else if r.Auth.Action != "" && r.Auth.Resource != "" {
			resource := r.Auth.Resource
			i := strings.Index(resource, "{")
			for i >= 0 {
				j := strings.Index(resource[i:], "}")
				if j < 0 {
					break
				}
				j += i
				resource = resource[0:i] + "\" + " + resource[i+1:j] + " + \"" + resource[j+1:]
				i = strings.Index(resource, "{")
			}
			resource = "\"" + resource + "\""
			s += fmt.Sprintf("            context.authorize(%q, %s, null);\n", r.Auth.Action, resource)
			//what about the domain variant?
		} else {
			log.Println("*** Badly formed auth spec in resource input:", r)
		}
	}
	for _, in := range r.Inputs {
		name := string(in.Name)
		if in.QueryParam != "" {
			//if !(in.Optional || in.Default != nil) {
			//	log.Println("RDL error: queryparam must either be optional or have a default value:", in.Name, "in resource", r)
			//}
			fargs = append(fargs, name)
		} else if in.PathParam {
			fargs = append(fargs, name)
		} else if in.Header != "" {
			fargs = append(fargs, name)
		} else {
			bodyName = name
			fargs = append(fargs, bodyName)
		}
	}
	methName, _ := javaMethodName(gen.registry, r)
	sargs := ""
	if len(fargs) > 0 {
		sargs = ", " + strings.Join(fargs, ", ")
	}
	if resultWrapper {
		a := "null"
		if async {
			a = "asyncResp"
		}
		s, _ := javaMethodName(gen.registry, r)
		rName := capitalize(s) + "Result"
		pathParamsArgs := strings.Join(gen.makePathParamsArgs(r), ", ")
		if pathParamsArgs == "" {
			pathParamsArgs = "null"
		}
		if async {
			s += "            " + rName + " result = new " + rName + "(context, " + pathParamsArgs + ", " + a + ");\n"
		} else {
			s += "            " + rName + " result = new " + rName + "(context);\n"
		}
		sargs += ", result"
		s += "            this.delegate." + methName + "(context" + sargs + ");\n"
	} else {
		s += "            " + returnType + " e = this.delegate." + methName + "(context" + sargs + ");\n"
		noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
		if len(r.Outputs) > 0 {
			for _, o := range r.Outputs {
				s += fmt.Sprintf("            this.response.addHeader(%q, e.%s);\n", o.Header, o.Name)
			}
		}
		if noContent {
			s += "            return null;\n"
		} else {
			s += "            return e;\n"
		}
	}
	s += "        } catch (ResourceException e) {\n"
	s += "            int code = e.getCode();\n"
	s += "            switch (code) {\n"
	if len(r.Alternatives) > 0 {
		for _, alt := range r.Alternatives {
			s += "            case ResourceException." + alt + ":\n"
		}
		s += "                throw typedException(code, e, " + returnType + ".class);\n"
	}
	if r.Exceptions != nil && len(r.Exceptions) > 0 {
		for ecode, edef := range r.Exceptions {
			etype := edef.Type
			s += "            case ResourceException." + ecode + ":\n"
			s += "                throw typedException(code, e, " + etype + ".class);\n"
		}
	}
	s += "            default:\n"
	s += "                System.err.println(\"*** Warning: undeclared exception (\" + code + \") for resource " + methName + "\");\n"
	s += "                throw typedException(code, e, ResourceError.class);\n" //? really
	s += "            }\n"
	s += "        }\n"
	return s
}

func (gen *javaServerGenerator) paramInit(qname string, pname string, ptype rdl.TypeRef, pdefault *interface{}) string {
	reg := gen.registry
	s := ""
	gtype := javaType(reg, ptype, false, "", "")
	switch ptype {
	case "String":
		if pdefault == nil {
			s += "\t" + pname + " := optionalStringParam(request, \"" + qname + "\", nil)\n"
		} else {
			def := fmt.Sprintf("%v", pdefault)
			s += "\tvar " + pname + "Optional " + gtype + " = " + def + "\n"
			s += "\t" + pname + " := optionalStringParam(request, \"" + qname + "\", " + pname + "Optional)\n"
		}
	case "Int32":
		if pdefault == nil {
			s += "\t" + pname + ", err := optionalInt32Param(request, \"" + qname + "\", nil)\n"
			s += "\tif err != nil {\n\t\tjsonResponse(writer, 400, err)\n\t\treturn\n\t}\n"
		} else {
			def := "0"
			switch v := (*pdefault).(type) {
			case *float64:
				def = fmt.Sprintf("%v", *v)
			default:
				panic("fix me")
			}
			s += "\t" + pname + ", err := requiredInt32Param(request, \"" + qname + "\", " + def + ")\n"
			s += "\tif err != nil {\n\t\tjsonResponse(writer, 400, err)\n\t\treturn\n\t}\n"
		}
	default:
		panic("fix me")
	}
	return s
}

func (gen *javaServerGenerator) handlerSignature(r *rdl.Resource) string {
	returnType := javaType(gen.registry, r.Type, false, "", "")
	reg := gen.registry
	var params []string
	if r.Async != nil && *r.Async {
		params = append(params, "@Suspended AsyncResponse asyncResp")
		returnType = "void"
	} else if len(r.Outputs) > 0 {
		returnType = "void"
	}
	for _, v := range r.Inputs {
		if v.Context != "" { //ignore these ones
			fmt.Fprintln(os.Stderr, "Warning: v1 style context param ignored:", v.Name, v.Context)
			continue
		}
		k := v.Name
		pdecl := ""
		if v.QueryParam != "" {
			pdecl = fmt.Sprintf("@QueryParam(%q) ", v.QueryParam) + defaultValueAnnotation(v.Default)
		} else if v.PathParam {
			pdecl = fmt.Sprintf("@PathParam(%q) ", k)
		} else if v.Header != "" {
			pdecl = fmt.Sprintf("@HeaderParam(%q) ", v.Header)
		}
		ptype := javaType(reg, v.Type, true, "", "")
		params = append(params, pdecl+ptype+" "+javaName(k))
	}
	spec := "@Produces(MediaType.APPLICATION_JSON)\n"
	switch r.Method {
	case "POST", "PUT":
		spec += "    @Consumes(MediaType.APPLICATION_JSON)\n"
	}

	methName, _ := javaMethodName(reg, r)
	return spec + "    public " + returnType + " " + methName + "(" + strings.Join(params, ", ") + ")"
}

func defaultValueAnnotation(val interface{}) string {
	if val != nil {
		switch v := val.(type) {
		case string:
			return fmt.Sprintf("@DefaultValue(%q) ", v)
		case int8:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int16:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int32:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case int64:
			return fmt.Sprintf("@DefaultValue(\"%d\") ", v)
		case float32:
			return fmt.Sprintf("@DefaultValue(\"%g\") ", v)
		case float64:
			return fmt.Sprintf("@DefaultValue(\"%g\") ", v)
		default:
			return fmt.Sprintf("@DefaultValue(\"%v\") ", v)
		}
	}
	return ""
}

func (gen *javaServerGenerator) handlerReturnType(r *rdl.Resource, methName string, returnType string) string {
	if len(r.Outputs) > 0 || (r.Async != nil && *r.Async) {
		//return capitalize(methName) + "Result"
		return "void"
	}
	return returnType
}

func (gen *javaServerGenerator) serverMethodSignature(r *rdl.Resource) string {
	reg := gen.registry
	returnType := javaType(reg, r.Type, false, "", "")
	//noContent := r.Expected == "NO_CONTENT" && r.Alternatives == nil
	//FIX: if nocontent, return nothing, have a void result, and don't "@Produces" anything
	methName, params := javaMethodName(reg, r)
	sparams := ""
	if len(params) > 0 {
		sparams = ", " + strings.Join(params, ", ")
	}
	returnType = gen.handlerReturnType(r, methName, returnType)
	if returnType == "void" {
		sparams = sparams + ", " + capitalize(methName) + "Result result"
	}
	return "public " + returnType + " " + methName + "(ResourceContext context" + sparams + ")"
}

func javaMethodName(reg rdl.TypeRegistry, r *rdl.Resource) (string, []string) {
	var params []string
	bodyType := string(safeTypeVarName(r.Type))
	for _, v := range r.Inputs {
		if v.Context != "" { //ignore these legacy things
			log.Println("Warning: v1 style context param ignored:", v.Name, v.Context)
			continue
		}
		k := v.Name
		if v.QueryParam == "" && !v.PathParam && v.Header == "" {
			bodyType = string(safeTypeVarName(v.Type))
		}
		//rest_core always uses the boxed type
		optional := true
		params = append(params, javaType(reg, v.Type, optional, "", "")+" "+javaName(k))
	}
	return strings.ToLower(string(r.Method)) + string(bodyType), params
}

func javaName(name rdl.Identifier) string {
	switch name {
	case "type", "default": //other reserved words
		return "_" + string(name)
	default:
		return string(name)
	}
}
