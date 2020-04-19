package main

import (
	"flag"
	"log"
	"path/filepath"
)

func main() {
	var (
		packageFolder string
	)

	flag.StringVar(&packageFolder, "package", "./rest", "Filepath of source package")
	flag.Parse()

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
