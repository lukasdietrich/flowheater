package main

import "fmt"

type ServiceCollection struct {
	PackageName string
	Services    []*Service
}

func AnalyzePackage(source *SourcePackage) (*ServiceCollection, error) {
	var (
		services []*Service
	)

	for _, serviceDeclaration := range source.Services() {
		service, err := analyzeService(serviceDeclaration)
		if err != nil {
			return nil, fmt.Errorf("analyzing service %s: %v",
				serviceDeclaration.Name(), err)
		}

		services = append(services, service)
	}

	return &ServiceCollection{
		PackageName: source.Name(),
		Services:    services,
	}, nil
}

type Service struct {
	TypeName  string
	Path      string
	Endpoints []*Endpoint
}

type Endpoint struct {
	Service  *Service
	FuncName string
	Path     string
}

func (e *Endpoint) WrapperFunc() string {
	return fmt.Sprintf("_handle_%s_%s", e.Service.TypeName, e.FuncName)
}

func analyzeService(declaration ServiceDeclaration) (*Service, error) {
	var (
		endpoints []*Endpoint
		service   = Service{
			TypeName: declaration.Name(),
			Path:     declaration.Path(),
		}
	)

	for _, endpointDeclaration := range declaration.Endpoints() {
		endpoint, err := analyzeEndpoint(endpointDeclaration)
		if err != nil {
			return nil, fmt.Errorf("analyzing endpoint %s: %v",
				endpointDeclaration.Name(), err)
		}

		endpoint.Service = &service
		endpoints = append(endpoints, endpoint)
	}

	service.Endpoints = endpoints
	return &service, nil
}

func analyzeEndpoint(declaration EndpointDeclaration) (*Endpoint, error) {
	return &Endpoint{
		FuncName: declaration.Name(),
		Path:     declaration.Path(),
	}, nil
}
