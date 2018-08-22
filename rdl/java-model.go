// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"github.com/ardielle/ardielle-go/gen/javamodel"
	"github.com/ardielle/ardielle-go/rdl"
)

func GenerateJavaModel(banner string, schema *rdl.Schema, outdir string, ns string, options []string) error {
	getSetters := javaGenerationBoolOptionSet(options, "getsetters")
	return javamodel.Generate(schema, &javamodel.GeneratorParams{
		Outdir:     outdir,
		Banner:     banner,
		Namespace:  ns,
		GetSetters: getSetters,
	})
}
