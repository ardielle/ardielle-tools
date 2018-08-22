// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"github.com/ardielle/ardielle-go/gen/gomodel"
)

//note as of v1.6, the guts of model generation was moved into ardielle-go/gen/... to encourage reuse

// GenerateGoModel generates the model code for the types defined in the RDL schema.
func GenerateGoModel(opts *generateOptions) error {
	schema := opts.schema
	outdir := opts.dirName
	banner := opts.banner
	return gomodel.Generate(schema, &gomodel.GeneratorParams{
		Outdir:         outdir,
		Banner:         banner,
		Namespace:      opts.ns,
		UntaggedUnions: opts.untaggedUnions,
		LibRdl:         opts.librdl,
		PrefixEnums:    opts.prefixEnums,
		PreciseTypes:   opts.preciseTypes,
	})
}
