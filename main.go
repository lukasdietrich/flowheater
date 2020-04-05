package main

import (
	"flag"
	"log"
)

func main() {
	var (
		packageName    string
		outputFilename string
	)

	flag.StringVar(&packageName, "package", "./rest", "Filepath of source package")
	flag.StringVar(&outputFilename, "output", "flowheater_gen.go", "Filename of the generated go code")

	flag.Parse()

	// Step 1: Parse the source package and search for annotated services.
	sourcePackage, err := ParsePackage(packageName)
	if err != nil {
		log.Fatalf("parsing source package: %v", err)
	}

	// Step 2: Analyze the annotated services and transform them into a
	//         structured collection.
	serviceCollection, err := AnalyzePackage(sourcePackage)
	if err != nil {
		log.Fatalf("generating router: %v", err)
	}

	// Step 3: Render the aggregrated router information into go code.
	if err := RenderServiceRouter(outputFilename, serviceCollection); err != nil {
		log.Fatalf("rendering router to go code: %v", err)
	}

	log.Printf("Router written to %s", outputFilename)
}
