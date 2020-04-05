package main

import (
	"strings"

	"github.com/wzshiming/gotype"
)

const (
	aPath   = "path"
	aMethod = "method"
)

// Annotations is a map of key-value pairs.
// The annotations are parsed from comments searching from bottom to top
// for lines of the format "<Key>: <Value>".
type Annotations map[string]string

func parseAnnotations(node gotype.Type) Annotations {
	var (
		comment     = strings.TrimSpace(node.Doc().Text())
		lines       = strings.Split(comment, "\n")
		annotations = make(Annotations)
	)

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]

		if !strings.ContainsRune(line, ':') {
			break
		}

		var (
			parts = strings.SplitN(line, ":", 2)
			key   = strings.TrimSpace(strings.ToLower(parts[0]))
			value = strings.TrimSpace(parts[1])
		)

		annotations[key] = value
	}

	return annotations
}

// Get returns the first value associated with a given key. Keys are not
// case-sensitive and only the first occurance is saved. If no value for the
// key exists, an empty string is returned.
func (a Annotations) Get(key string) string {
	return a[strings.ToLower(key)]
}

// Exists tests for the presence of a given key. Keys are not case-sensitive.
func (a Annotations) Exists(key string) bool {
	_, ok := a[strings.ToLower(key)]
	return ok
}

// SourcePackage is a collection of annotated services and their endpoints
// found in a user provided go source package.
type SourcePackage struct {
	node     gotype.Type
	services []ServiceDeclaration
}

func ParsePackage(packageName string) (*SourcePackage, error) {
	importer := gotype.NewImporter()
	node, err := importer.Import(packageName, "")
	if err != nil {
		return nil, err
	}

	return &SourcePackage{
		node:     node,
		services: findServiceDeclarations(node),
	}, nil
}

// Name returns the package name.
func (s *SourcePackage) Name() string {
	return s.node.Name()
}

// Services returns a slice of declared services.
func (s *SourcePackage) Services() []ServiceDeclaration {
	return s.services
}

func findServiceDeclarations(pkgNode gotype.Type) []ServiceDeclaration {
	var services []ServiceDeclaration

	for i, length := 0, pkgNode.NumChild(); i < length; i++ {
		if node := pkgNode.Child(i); node.Kind() == gotype.Struct {
			if a := parseAnnotations(node); a.Exists(aPath) {
				services = append(services, ServiceDeclaration{
					node:        node,
					annotations: a,
					endpoints:   findEndpointDeclarations(node),
				})
			}
		}
	}

	return services
}

func findEndpointDeclarations(serviceNode gotype.Type) []EndpointDeclaration {
	var endpoints []EndpointDeclaration

	for i, length := 0, serviceNode.NumMethod(); i < length; i++ {
		node := serviceNode.Method(i)
		if a := parseAnnotations(node); a.Exists(aPath) {
			endpoints = append(endpoints, EndpointDeclaration{
				node:        node,
				annotations: a,
			})
		}
	}

	return endpoints
}

// ServiceDeclaration captures the information of a struct type, that is
// annotated with at least "Path: <path-value>".
type ServiceDeclaration struct {
	node        gotype.Type
	annotations Annotations
	endpoints   []EndpointDeclaration
}

func (s *ServiceDeclaration) Name() string {
	return s.node.Name()
}

func (s *ServiceDeclaration) Path() string {
	return s.annotations[aPath]
}

func (s *ServiceDeclaration) Endpoints() []EndpointDeclaration {
	return s.endpoints
}

// EndpointDeclaration captures the information of a method, that is annotated
// annotated with at least "Path: <path-value>".
type EndpointDeclaration struct {
	node        gotype.Type
	annotations Annotations
}

func (e *EndpointDeclaration) Name() string {
	return e.node.Name()
}

func (e *EndpointDeclaration) Path() string {
	return e.annotations[aPath]
}
