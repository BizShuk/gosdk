package main

import (
	"flag"
	"fmt"
	"go/ast"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bizshuk/gosdk/service"
)

var (
	typeNames   = flag.String("type", "", "comma-separated list of type names; must be set")
	output      = flag.String("output", "", "output file name; default srcdir/<type>_string.go")
	trimprefix  = flag.String("trimprefix", "", "trim the `prefix` from the generated constant names")
	linecomment = flag.Bool("linecomment", false, "use line comment text as printed text when present")
	buildTags   = flag.String("tags", "", "comma-separated list of build tags to apply")
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of stringer:\n")
	fmt.Fprintf(os.Stderr, "\tstringer [flags] -type T [directory]\n")
	fmt.Fprintf(os.Stderr, "\tstringer [flags] -type T files... # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "For more information, see:\n")
	fmt.Fprintf(os.Stderr, "\thttps://pkg.go.dev/golang.org/x/tools/cmd/stringer\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("stringer: ")
	flag.Usage = Usage
	flag.Parse()
	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*typeNames, ",")
	var tags []string
	if len(*buildTags) > 0 {
		tags = strings.Split(*buildTags, ",")
	}

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	// Parse the package once.
	var dir string
	g := GeneratorEx{}

	g.SetTrimPrefix(*trimprefix)
	g.SetLineComment(*linecomment)

	// TODO(suzmue): accept other patterns for packages (directories, list of files, import paths, etc).
	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
	} else {
		if len(tags) != 0 {
			log.Fatal("-tags option applies only to directories, not when files are specified")
		}
		dir = filepath.Dir(args[0])
	}

	g.ParsePackage(args, tags)

	// Print the header and package clause.
	g.Printf("// Code generated by \"stringer %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " "))
	g.Printf("\n")
	g.Printf("package %s", g.GetPackage().GetName())
	g.Printf("\n")
	g.Printf("import \"strconv\"\n") // Used by all methods.

	// Run generate for each type.
	for _, typeName := range types {
		g.generate(typeName)
	}

	// Format the output.
	src := g.Format()

	// Write to file.
	outputName := *output
	if outputName == "" {
		baseName := fmt.Sprintf("%s_string.go", types[0])
		outputName = filepath.Join(dir, strings.ToLower(baseName))
	}
	err := os.WriteFile(outputName, src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}

type GeneratorEx struct {
	service.Generator
}

func (g *GeneratorEx) generate(typeName string) {
	g.Generator.Generate(typeName)

	values := make([]service.Value, 0, 100)
	for _, file := range g.GetPackage().GetFile() {
		// Set the state for this run of the walker.
		file.SetTypeName(typeName)
		file.SetValues(nil)
		if file.GetFile() != nil {
			ast.Inspect(file.GetFile(), file.GenDecl)
			values = append(values, file.GetValues()...)
		}
	}
	runs := service.SplitIntoRuns(values)

	g.buildListFn(runs, typeName)
	g.buildValueListFn(runs, typeName)
	g.buildMapFn(runs, typeName)
	g.buildValueMapFn(runs, typeName)
}

func (g *GeneratorEx) buildListFn(runs [][]service.Value, typeName string) {
	g.Printf("\n")
	g.Printf("var _%s_list = []string{\n", typeName)
	for _, run := range runs {
		for i := range run {
			g.Printf("\t\"%s\",\n", run[i].Name())
		}
	}
	g.Printf("}\n")
	g.Printf(listFnTemplate, typeName)
}

// Argument to format is the type name.
const listFnTemplate = `func %[1]sList() []string {
	return _%[1]s_list
}
`

func (g *GeneratorEx) buildValueListFn(runs [][]service.Value, typeName string) {
	g.Printf("\n")
	g.Printf("var _%s_value_list = []int64{\n", typeName)

	for _, run := range runs {
		for i := range run {
			g.Printf("\t%d,\n", run[i].Value())
		}
	}

	g.Printf("}\n")
	g.Printf(valueListFnTemplate, typeName)
}

// Argument to format is the type name.
const valueListFnTemplate = `func %[1]sValueList() []int64 {
	return _%[1]s_value_list
}
`

func (g *GeneratorEx) buildMapFn(runs [][]service.Value, typeName string) {
	g.Printf("\n")
	g.Printf("var _%s_map = map[string]int64{\n", typeName)
	for _, run := range runs {
		for i := range run {
			g.Printf("\t\"%s\": %d,\n", run[i].Name(), run[i].Value())
		}
	}
	g.Printf("}\n")
	g.Printf(mapFnTemplate, typeName)
}

// Argument to format is the type name.
const mapFnTemplate = `func %[1]sMap() map[string]int64 {
	return _%[1]s_map
}
`

func (g *GeneratorEx) buildValueMapFn(runs [][]service.Value, typeName string) {
	g.Printf("\n")
	g.Printf("var _%s_value_map = map[int64]string{\n", typeName)
	for _, run := range runs {
		for i := range run {
			g.Printf("\t%d: \"%s\",\n", run[i].Value(), run[i].Name())
		}
	}
	g.Printf("}\n")
	g.Printf(valueMapFnTemplate, typeName)
}

// Argument to format is the type name.
const valueMapFnTemplate = `func %[1]sValueMap() map[int64]string {
	return _%[1]s_value_map
}
`
