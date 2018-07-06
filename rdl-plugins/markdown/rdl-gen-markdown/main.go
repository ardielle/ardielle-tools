// Copyright 2015 Yahoo Inc.
// Licensed under the terms of the Apache version 2.0 license. See LICENSE file for terms.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ardielle/ardielle-go/rdl"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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
			ExportToMarkdown(&schema, *pOutdir)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "*** %v\n", err)
	os.Exit(1)
}

func capitalize(text string) string {
	return strings.ToUpper(text[0:1]) + text[1:]
}

func uncapitalize(text string) string {
	return strings.ToLower(text[0:1]) + text[1:]
}

func leftJustified(text string, width int) string {
	return text + spaces(width-len(text))
}

func formatBlock(s string, leftCol int, rightCol int, prefix string) string {
	if s == "" {
		return ""
	}
	tab := spaces(leftCol)
	var buf bytes.Buffer
	max := 80
	col := leftCol
	lines := 1
	tokens := strings.Split(s, " ")
	for _, tok := range tokens {
		toklen := len(tok)
		if col+toklen >= max {
			buf.WriteString("\n")
			lines++
			buf.WriteString(tab)
			buf.WriteString(prefix)
			buf.WriteString(tok)
			col = leftCol + 3 + toklen
		} else {
			if col == leftCol {
				col += len(prefix)
				buf.WriteString(prefix)
			} else {
				buf.WriteString(" ")
			}
			buf.WriteString(tok)
			col += toklen + 1
		}
	}
	buf.WriteString("\n")
	emptyPrefix := strings.Trim(prefix, " ")
	pad := tab + emptyPrefix + "\n"
	return pad + buf.String() + pad
}

func spaces(count int) string {
	return stringOfChar(count, ' ')
}

func stringOfChar(count int, b byte) string {
	buf := make([]byte, 0, count)
	for i := 0; i < count; i++ {
		buf = append(buf, b)
	}
	return string(buf)
}

func optionalAnyToString(any interface{}) string {
	if any == nil {
		return "null"
	}
	switch v := any.(type) {
	case *bool:
		return fmt.Sprintf("%v", *v)
	case *int8:
		return fmt.Sprintf("%d", *v)
	case *int16:
		return fmt.Sprintf("%d", *v)
	case *int32:
		return fmt.Sprintf("%d", *v)
	case *int64:
		return fmt.Sprintf("%d", *v)
	case *float32:
		return fmt.Sprintf("%g", *v)
	case *float64:
		return fmt.Sprintf("%g", *v)
	case *string:
		return *v
	case bool:
		return fmt.Sprintf("%v", v)
	case int8:
		return fmt.Sprintf("%d", v)
	case int16:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case string:
		return fmt.Sprintf("%v", v)
	default:
		panic("optionalAnyToString")
	}
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

//ExportToMarkdown exports a markdown rendering of the schema
func ExportToMarkdown(schema *rdl.Schema, outdir string) error {
	out, file, _, err := outputWriter(outdir, string(schema.Name), ".md")
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}
	registry := rdl.NewTypeRegistry(schema)
	category := "schema"
	if schema.Resources != nil {
		category = "API"
	}
	title := category
	if schema.Name != "" {
		title = "The " + capitalize(string(schema.Name)) + " " + category
	} else {
		title = capitalize(category)
	}
	fmt.Fprintf(out, "# %s\n\n", title)
	if schema.Comment != "" {
		fmt.Fprintf(out, "%s", formatBlock(schema.Comment, 0, 80, ""))
	}

	var rows [][]string
	if schema.Namespace != "" {
		rows = append(rows, []string{"namespace", string(schema.Namespace)})
	}
	if schema.Version != nil {
		rows = append(rows, []string{"version", fmt.Sprintf("%d", *schema.Version)})
	}
	if len(rows) > 0 {
		fmt.Fprintf(out, "This %s has the following attributes:\n\n", category)
		formatTable(out, []string{"Attribute", "Value"}, rows)
	}

	if schema.Resources != nil {
		fmt.Fprintf(out, "\n## Resources\n")
		groups := groupResources(schema.Resources)
		for _, entry := range groups {
			group := entry.name
			lstRez := entry.resources
			fmt.Fprintf(out, "\n### [%s](#TypeDef_%s)\n", group, group)
			//too much? formatType(out, schema, schema.FindType(group))
			for _, rez := range lstRez {
				//ideally, sort by method here to be consistent
				formatResource(out, registry, rez)
			}
		}
	}

	if len(schema.Types) > 0 {
		fmt.Fprintf(out, "\n## Types\n")
		for _, typeDef := range schema.Types {
			formatType(out, registry, typeDef)
		}
	}
	out.Flush()
	return nil
}

type entry struct {
	name string
	resources []*rdl.Resource
}

func groupResources(resources []*rdl.Resource) []*entry {
	groups := map[string][]*rdl.Resource{}
	result := make([]*entry, 0)
	for _, rez := range resources {
		rtype := string(rez.Type)
		if ent, ok := groups[rtype]; ok {
			groups[rtype] = append(ent, rez)
		} else {
			ent = []*rdl.Resource{rez}
			groups[rtype] = ent
			result = append(result, &entry{name: rtype})
		}
	}
	for _, entry := range result {
		entry.resources = groups[entry.name]
	}
	return result
}

func formatType(out io.Writer, registry rdl.TypeRegistry, typeDef *rdl.Type) {
	tName, _, tComment := rdl.TypeInfo(typeDef)
	fmt.Fprintf(out, "\n### <a name=\"TypeDef_%s\">%s</a>\n", tName, tName)
	if tComment != "" {
		fmt.Fprintf(out, "%s", formatBlock(tComment, 0, 80, ""))
	}
	types := typeStack(registry, typeDef)
	name := string(tName)
	switch typeDef.Variant {
	case rdl.TypeVariantStringTypeDef:
		formatStringType(out, registry, name, types)
	case rdl.TypeVariantNumberTypeDef:
		formatNumberType(out, registry, name, types)
	case rdl.TypeVariantStructTypeDef:
		formatStructType(out, registry, types)
	case rdl.TypeVariantUnionTypeDef:
		tt := typeDef.UnionTypeDef
		formatUnionType(out, registry, tt)
	case rdl.TypeVariantEnumTypeDef:
		tt := typeDef.EnumTypeDef
		formatEnumType(out, registry, tt)
	case rdl.TypeVariantArrayTypeDef:
		tt := typeDef.ArrayTypeDef
		name = name + "&lt;" + string(tt.Items) + "&gt;"
		formatArrayType(out, registry, name, types)
	case rdl.TypeVariantMapTypeDef:
		tt := typeDef.MapTypeDef
		name = name + "&lt;" + string(tt.Keys) + "," + string(tt.Items) + "&gt;"
		formatMapType(out, registry, name, types)
	case rdl.TypeVariantAliasTypeDef:
		formatAliasType(out, registry, typeDef.AliasTypeDef)
	case rdl.TypeVariantBytesTypeDef:
		formatBytesType(out, registry, name, types)
	case rdl.TypeVariantBaseType:
		//never happens
	}
}

func typeStack(registry rdl.TypeRegistry, typeDef *rdl.Type) []*rdl.Type {
	var types []*rdl.Type
	types = append(types, typeDef)
	tName, tType, _ := rdl.TypeInfo(typeDef)
	for tName != rdl.TypeName(tType) {
		supertype := registry.FindType(tType)
		types = append(types, supertype)
		tName, tType, _ = rdl.TypeInfo(supertype)
	}
	return types
}

func annotate(registry rdl.TypeRegistry, typename rdl.TypeRef) string {
	t := registry.FindType(typename)
	if t != nil {
		tName, tType, _ := rdl.TypeInfo(t)
		if tType != rdl.TypeRef(tName) {
			return "[" + string(typename) + "](#TypeDef_" + string(typename) + ")"
		}
	}
	return string(typename)
}

func formatStructType(out io.Writer, registry rdl.TypeRegistry, types []*rdl.Type) {
	topType := types[0].StructTypeDef
	var rows [][]string
	for i := len(types) - 1; i >= 0; i-- {
		switch types[i].Variant {
		case rdl.TypeVariantStructTypeDef:
			t := types[i].StructTypeDef
			for _, f := range t.Fields {
				fn := f.Name
				ft := annotate(registry, f.Type)
				if f.Keys != "" {
					ft = ft + "&lt;" + annotate(registry, f.Keys) + "," + annotate(registry, f.Items) + "&gt;"
				} else if f.Items != "" {
					ft = ft + "&lt;" + annotate(registry, f.Items) + "&gt;"
					//					} else if f.Variants != nil {
					//						ft = ft + "&lt;" + *f.Variants + "&gt;"
				}
				fo := ""
				if f.Optional {
					fo = "optional"
				}
				if f.Default != nil {
					s := optionalAnyToString(f.Default)
					if fo == "" {
						fo = "default=" + s
					} else {
						fo += ", default=" + s
					}
				}
				fc := ""
				if f.Comment != "" {
					fc += f.Comment
				}
				ff := ""
				if t != topType {
					ff = "[from [" + string(t.Name) + "](#TypeDef_" + string(t.Name) + ")]"
				}
				row := []string{string(fn), ft, fo, fc, ff}
				rows = append(rows, row)
			}
			//case *TypeDef:
			//fmt.Println("generic struct alias:", t)
		}
	}
	if rows == nil {
		fmt.Fprintf(out, "`%s` is a `Struct` with no specified fields\n\n", topType.Name)

	} else {
		fmt.Fprintf(out, "`%s` is a `Struct` type with the following fields:\n\n", topType.Name)
		formatTable(out, []string{"Name", "Type", "Options", "Description", "Notes"}, rows)
	}
}

func formatUnionType(out io.Writer, registry rdl.TypeRegistry, typeDef *rdl.UnionTypeDef) {
	fmt.Fprintf(out, "`%s` is a `Union` of following types:\n\n", typeDef.Name)
	var rows [][]string
	for _, vn := range typeDef.Variants {
		row := []string{annotate(registry, vn)}
		rows = append(rows, row)
	}
	formatTable(out, []string{"Variant"}, rows)
}

func formatArrayType(out io.Writer, registry rdl.TypeRegistry, name string, types []*rdl.Type) {
	var options [][]string
	var size, minSize, maxSize *[]string
	topType := types[0].ArrayTypeDef
	for i := len(types) - 1; i >= 0; i-- {
		c := ""
		t := types[i].ArrayTypeDef
		if t != nil {
			if t != topType {
				c = "[from [" + string(t.Name) + "](#" + string(t.Name) + ")]"
			}
			if t.Size != nil {
				size = &[]string{"minSize", fmt.Sprintf("%d", *t.Size), c}
			}
			if t.MinSize != nil {
				minSize = &[]string{"minSize", fmt.Sprintf("%d", *t.MinSize), c}
			}
			if t.MaxSize != nil {
				maxSize = &[]string{"maxSize", fmt.Sprintf("%d", *t.MaxSize), c}
			}
		}
	}
	if size != nil {
		options = append(options, *size)
	}
	if minSize != nil {
		options = append(options, *minSize)
	}
	if maxSize != nil {
		options = append(options, *maxSize)
	}
	items := topType.Items
	if len(options) > 0 {
		fmt.Fprintf(out, "`%s` is an `Array` of `%s` with the following options:\n\n", name, items)
		formatTable(out, []string{"Option", "Value", "Notes"}, options)
	} else {
		fmt.Fprintf(out, "`%s` is an `Array` of `%s`.\n\n", topType.Name, items)
	}
}

func formatMapType(out io.Writer, registry rdl.TypeRegistry, name string, types []*rdl.Type) {
	var options [][]string
	var size, minSize, maxSize *[]string
	topType := types[0].MapTypeDef
	for i := len(types) - 1; i >= 0; i-- {
		t := types[i].MapTypeDef
		if t != nil {
			c := ""
			if t != topType {
				c = "[from [" + string(t.Name) + "](#" + string(t.Name) + ")]"
			}
			if t.Size != nil {
				size = &[]string{"minSize", fmt.Sprintf("%d", *t.Size), c}
			}
			if t.MinSize != nil {
				minSize = &[]string{"minSize", fmt.Sprintf("%d", *t.MinSize), c}
			}
			if t.MaxSize != nil {
				maxSize = &[]string{"maxSize", fmt.Sprintf("%d", *t.MaxSize), c}
			}
		}
	}
	if size != nil {
		options = append(options, *size)
	}
	if minSize != nil {
		options = append(options, *minSize)
	}
	if maxSize != nil {
		options = append(options, *maxSize)
	}
	keys := topType.Keys
	items := topType.Items
	if len(options) > 0 {
		fmt.Fprintf(out, "`%s` is a Map of `%s` to `%s` with the following options:\n\n", name, keys, items)
		formatTable(out, []string{"Option", "Value", "Notes"}, options)
	} else {
		fmt.Fprintf(out, "`%s` is a Map of `%s` to `%s`.\n\n", topType.Name, keys, items)
	}
}

func formatStringType(out io.Writer, registry rdl.TypeRegistry, name string, types []*rdl.Type) {
	var options [][]string
	var pattern, values, minSize, maxSize *[]string
	baseType := types[len(types)-1]
	topType := types[0].StringTypeDef
	for i := len(types) - 1; i >= 0; i-- {
		switch types[i].Variant {
		case rdl.TypeVariantStringTypeDef:
			t := types[i].StringTypeDef
			c := ""
			if t != topType {
				c = "[from [" + string(t.Name) + "](#" + string(t.Name) + ")]"
			}
			if t.Pattern != "" {
				pattern = &[]string{"pattern", "`\"" + t.Pattern + "`\"", c}
			}
			if len(t.Values) > 0 {
				values = &[]string{"values", "`\"" + strings.Join(t.Values, ", ") + "`\"", c}
			}
			if t.MinSize != nil {
				minSize = &[]string{"minSize", fmt.Sprintf("%d", *t.MinSize), c}
			}
			if t.MaxSize != nil {
				maxSize = &[]string{"maxSize", fmt.Sprintf("%d", *t.MaxSize), c}
			}
		}
	}
	if pattern != nil {
		options = append(options, *pattern)
	}
	if values != nil {
		options = append(options, *values)
	}
	if minSize != nil {
		options = append(options, *minSize)
	}
	if maxSize != nil {
		options = append(options, *maxSize)
	}
	if len(options) > 0 {
		fmt.Fprintf(out, "`%s` is a `%s` type with the following options:\n\n", name, baseType)
		formatTable(out, []string{"Option", "Value", "Notes"}, options)
	} else {
		fmt.Fprintf(out, "`%s` is a `%s` type.\n\n", topType.Name, baseType)
	}
}

func formatNumberType(out io.Writer, registry rdl.TypeRegistry, name string, types []*rdl.Type) {
	var options [][]string
	var minVal, maxVal *[]string
	baseType, _, _ := rdl.TypeInfo(types[len(types)-1])
	topType := types[0].NumberTypeDef
	for i := len(types) - 1; i >= 0; i-- {
		switch types[i].Variant {
		case rdl.TypeVariantNumberTypeDef:
			t := types[i].NumberTypeDef
			c := ""
			if t != topType {
				c = "[from [" + string(t.Name) + "](#" + string(t.Name) + ")]"
			}
			if t.Min != nil {
				minVal = &[]string{"min", fmt.Sprintf("%v", *t.Min), c}
			}
			if t.Max != nil {
				maxVal = &[]string{"max", fmt.Sprintf("%v", *t.Max), c}
			}
		}
	}
	if minVal != nil {
		options = append(options, *minVal)
	}
	if maxVal != nil {
		options = append(options, *maxVal)
	}
	if len(options) > 0 {
		fmt.Fprintf(out, "`%s` is a `%s` type with the following options:\n\n", name, baseType)
		formatTable(out, []string{"Option", "Value", "Notes"}, options)
	} else {
		fmt.Fprintf(out, "`%s` is a `%s` type.\n\n", topType.Name, baseType)
	}
}

func formatAliasType(out io.Writer, registry rdl.TypeRegistry, typeDef *rdl.AliasTypeDef) {
	fmt.Fprintf(out, "`%s` is an alias of type `%s`\n", typeDef.Name, typeDef.Type)
}

func formatBytesType(out io.Writer, registry rdl.TypeRegistry, name string, types []*rdl.Type) {
	var options [][]string
	var minSize, maxSize *[]string
	baseType := types[len(types)-1]
	topType := types[0].BytesTypeDef
	for i := len(types) - 1; i >= 0; i-- {
		switch types[i].Variant {
		case rdl.TypeVariantBytesTypeDef:
			t := types[i].BytesTypeDef
			c := ""
			if t != topType {
				c = "[from [" + string(t.Name) + "](#" + string(t.Name) + ")]"
			}
			if t.MinSize != nil {
				minSize = &[]string{"minSize", fmt.Sprintf("%d", *t.MinSize), c}
			}
			if t.MaxSize != nil {
				maxSize = &[]string{"maxSize", fmt.Sprintf("%d", *t.MaxSize), c}
			}
		}
	}
	if minSize != nil {
		options = append(options, *minSize)
	}
	if maxSize != nil {
		options = append(options, *maxSize)
	}
	if len(options) > 0 {
		fmt.Fprintf(out, "`%s` is a `%s` type with the following options:\n\n", name, baseType)
		formatTable(out, []string{"Option", "Value", "Notes"}, options)
	} else {
		fmt.Fprintf(out, "`%s` is a `%s` type.\n\n", topType.Name, baseType)
	}
}

func formatEnumType(out io.Writer, registry rdl.TypeRegistry, typeDef *rdl.EnumTypeDef) {
	fmt.Fprintf(out, "`%s` is an `Enum` of the following values:\n\n", typeDef.Name)
	var rows [][]string
	for _, elem := range typeDef.Elements {
		vn := string(elem.Symbol)
		s := elem.Comment
		row := []string{vn, s}
		rows = append(rows, row)
	}
	formatTable(out, []string{"Value", "Description"}, rows)
}

func formatTable(out io.Writer, header []string, rows [][]string) {
	columns := len(header)
	widths := make([]int, columns)
	for i := 0; i < columns; i++ {
		if len(header[i]) > widths[i] {
			widths[i] = len(header[i])
		}
		for j := 0; j < len(rows); j++ {
			row := rows[j][i]
			if len(row) > widths[i] {
				widths[i] = len(row)
			}
		}
	}
	s := "|"
	for i, h := range header {
		s += fmt.Sprintf(" %s |", leftJustified(h, widths[i]))
	}
	fmt.Fprintf(out, "%s\n", s)
	s = "|"
	for i := range header {
		s += fmt.Sprintf("-%s-|", stringOfChar(widths[i], '-'))
	}
	fmt.Fprintf(out, "%s\n", s)
	for _, row := range rows {
		s = "|"
		for i, r := range row {
			s += fmt.Sprintf(" %s |", leftJustified(r, widths[i]))
		}
		fmt.Fprintf(out, "%s\n", s)
	}
	fmt.Fprintf(out, "\n")
}

func formatResource(out io.Writer, registry rdl.TypeRegistry, rez *rdl.Resource) {
	fmt.Fprintf(out, "\n#### %s %s\n", strings.ToUpper(rez.Method), rez.Path)
	if rez.Comment != "" {
		fmt.Fprintf(out, "%s", formatBlock(rez.Comment, 0, 80, ""))
	}
	if len(rez.Inputs) > 0 {
		var rows [][]string
		for _, f := range rez.Inputs {
			fn := string(f.Name)
			ft := annotate(registry, f.Type)
			fs := ""
			if f.PathParam {
				fs = "path"
			} else if f.QueryParam != "" {
				fs = "query: " + f.QueryParam
			} else if f.Header != "" {
				fs = "header: " + f.Header
				//			} else if f.Context != "" {
				//				fs = "context: " + f.Context
			} else {
				fs = "body"
			}
			fo := ""
			if f.Optional {
				fo = "optional"
			}
			if f.Default != nil {
				s := optionalAnyToString(f.Default)
				if fo == "" {
					fo = "default=" + s
				} else {
					fo += ", default=" + s
				}
			}
			if f.Pattern != "" {
				if fo != "" {
					fo += ", "
				}
				fo += "pattern: " + f.Pattern
			}
			if f.Flag {
				if fo != "" {
					fo += ", "
				}
				fo += "flag"
			}
			fc := ""
			if f.Comment != "" {
				fc += f.Comment
			}
			row := []string{fn, ft, fs, fo, fc}
			rows = append(rows, row)
		}
		if rows != nil {
			fmt.Fprintf(out, "\n#### Request parameters:\n\n")
			formatTable(out, []string{"Name", "Type", "Source", "Options", "Description"}, rows)
		}
	}
	if len(rez.Outputs) > 0 {
		var rows [][]string
		for _, f := range rez.Outputs {
			fn := string(f.Name)
			ft := annotate(registry, f.Type)
			fd := "header: " + f.Header
			fo := "false"
			if f.Optional {
				fo = "true"
			}
			fc := ""
			if f.Comment != "" {
				fc = f.Comment
			}
			row := []string{fn, ft, fd, fo, fc}
			rows = append(rows, row)
		}
		if rows != nil {
			fmt.Fprintf(out, "\n#### Response parameters:\n\n")
			formatTable(out, []string{"Name", "Type", "Destination", "Optional", "Description"}, rows)
		}
	}
	fmt.Fprintf(out, "\n#### Responses:\n\n")
	var results [][]string
	if rez.Expected != "OK" {
		e := rez.Expected
		s := ""
		if e != "NO_CONTENT" {
			s = annotate(registry, rez.Type)
		}
		results = append(results, []string{rdl.StatusCode(e) + " " + rdl.StatusMessage(e), s})
	} else {
		results = append(results, []string{"200 " + rdl.StatusMessage("OK"), string(rez.Type)})
	}
	if len(rez.Alternatives) > 0 {
		for _, v := range rez.Alternatives {
			s := ""
			if v != "NO_CONTENT" {
				s = annotate(registry, rez.Type)
			}
			results = append(results, []string{rdl.StatusCode(v) + " " + rdl.StatusMessage(v), s})
		}
	}
	fmt.Fprintf(out, "Expected:\n\n")
	formatTable(out, []string{"Code", "Type"}, results)

	if len(rez.Exceptions) > 0 {
		var rows [][]string
		for ec, edef := range rez.Exceptions {
			etype := edef.Type
			et := annotate(registry, rdl.TypeRef(etype))
			ecomment := edef.Comment
			row := []string{rdl.StatusCode(ec) + " " + rdl.StatusMessage(ec), et, ecomment}
			rows = append(rows, row)
		}
		if rows != nil {
			sort.Sort(byCode(rows))
			fmt.Fprintf(out, "\nException:\n\n")
			formatTable(out, []string{"Code", "Type", "Comment"}, rows)
		}
	}
}

type byCode [][]string

func (a byCode) Len() int           { return len(a) }
func (a byCode) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCode) Less(i, j int) bool { return a[i][0] < a[j][0] }
