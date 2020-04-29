package main

import (
	"flag"
	"log"
	"path/filepath"
)

var (
	packageFolder        string
	customErrorHandler   bool
	customRequestReader  bool
	customResponseWriter bool
)

func init() {
	log.SetFlags(0)

	flag.StringVar(&packageFolder,
		"package",
		"./rest",
		"Filepath of source package")

	flag.BoolVar(&customErrorHandler,
		"custom-error-handler",
		false,
		"Enable custom error handler as a parameter on the router")

	flag.BoolVar(&customRequestReader,
		"custom-request-reader",
		false,
		"Enable custom request reader as a parameter on the router")

	flag.BoolVar(&customResponseWriter,
		"custom-response-writer",
		false,
		"Enable custom response writer as a parameter on the router")

	flag.Parse()
}

func main() {
	// Step 1: Parse the source package and search for annotated services.
	sourcePackage, err := ParsePackage(packageFolder)
	if err != nil {
		log.Fatalf("ERROR: parsing source package: %v", err)
	}

	// Step 2: Analyze the annotated services and transform them into a
	//         structured collection.
	serviceCollection, err := AnalyzePackage(sourcePackage)
	if err != nil {
		log.Fatalf("ERROR: generating router: %v", err)
	}

	outputFilename := filepath.Join(sourcePackage.Filepath(), "flowheater_gen.go")

	// Step 3: Render the aggregrated router information into go code.
	if err := RenderServiceRouter(outputFilename, serviceCollection); err != nil {
		log.Fatalf("ERROR: rendering router to go code: %v", err)
	}

	log.Printf("Router written to %s", outputFilename)
}
