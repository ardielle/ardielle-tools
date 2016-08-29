# ardielle-tools [![Build Status](https://travis-ci.org/ardielle/ardielle-tools.svg?branch=master)](https://travis-ci.org/ardielle/ardielle-tools)

RDL tools

## Installation

To install from source, you need go v1.4.2 or greater installed, and do this:

    go get github.com/ardielle/ardielle-tools/...

The binaries will be installed into $GOPATH/bin, and the source into $GOPATH/src

## Usage

The [rdl](rdl) contains the main `rdl` command line tool and core code generators that
the tool itself depends on. Other generators are in the [rdl-gen](rdl-gen) dcirectory.

The `rdl` tool has the following usage:

	Usage: rdl [OPTIONS] COMMAND [arg...]

	Parse and process an RDL file.

	Options:
	  -p           show errors and non-exported results in a prettier way (default is false)
	  -w           suppress warnings (default is false)
	  -s           parse in strict mode (default is false)

	Commands:
	  help
	  version
	  parse <schemafile.rdl>
	  validate <datafile.json> <schemafile.rdl> [<typename>]
	  generate [-elt] [-o <outfile>] <generator> <schema.rdl>

	Generator Options:
	  -o path         Use the directory or file as output for generation. Default is stdout.
	  -e              Generate Enum constants prefixed by the type name to avoid collisions (default is false)
	  --ns namespace  Use the specified namespace for code generation. Default is to use the namespace in the schema.
	  -t              Generate precise type models, i.e. model string and numeric subtypes in Go (default is false)
	  -l package      Generate code that imports this package as 'rdl' for base type impl (instead of standard rdl library)
	  -u type         Generate the specified union type to JSON serialize as an untagged union. Default is a tagged.
	  -x key=value    Set options for external generator, e.g. -x e=true -xfoo=bar will send -e true --foo bar to external generator.

	Generators (accepted arguments to the generate command):

	  json        Generate the JSON representation of the schema
	  go-model    Generate the Go code for the types in the schema
	  go-client   Generate the Go code for a client to the resources in the schema
	  go-server   Generate the Go code for a server implementation  of the resources in the schema
	  java-model  Generate the Java code for the types in the schema
	  java-client Generate the Java code for a client to the resources in the schema
	  java-server Generate the Java code for a server implementation  of the resources in the schema
	  markdown    Generate the markdown representation of the schema and its comments
	  swagger     Generage the swagger resource for the schema. If the outfile is an endpoint, serve it via HTTP.
	
	  <name>      Invoke an external generator named 'rdl-gen-<name>', searched for in your $PATH. The
	              generator is passed the -o flag if it was set, and the JSON representation of the schema
	              is written to its stdin. You can override the default external generators this way.



## License

Copyright 2015 Yahoo Inc.

Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.
