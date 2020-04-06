package main

import (
	"fmt"
	"net/http"
	"strings"
)

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
	Service    *Service
	FuncName   string
	Path       string
	HttpMethod string
	Resolvers  []Resolver
	Params     []string
}

func (e *Endpoint) WrapperFunc() string {
	return fmt.Sprintf("_handle_%s_%s", e.Service.TypeName, e.FuncName)
}

type Resolver struct {
	VarName   string
	ParamName string
	TypeName  string
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
	var (
		resolvers  []Resolver
		params     []string
		httpMethod = http.MethodGet
	)

	if declaration.Annotations().Exists(aMethod) {
		httpMethod = strings.ToUpper(declaration.Annotations().Get(aMethod))
	}

	for i, paramDeclaration := range declaration.InputParams() {
		switch typeName := paramDeclaration.Type(); typeName {
		case "net/http/ResponseWriter":
			params = append(params, "w")

		case "net/http/*Request":
			params = append(params, "r")

		default:
			varName := fmt.Sprintf("param%d", i)
			params = append(params, varName)
			resolvers = append(resolvers, Resolver{
				VarName:   varName,
				ParamName: paramDeclaration.Name(),
				TypeName:  typeName,
			})
		}
	}

	return &Endpoint{
		FuncName:   declaration.Name(),
		Path:       declaration.Path(),
		HttpMethod: httpMethod,
		Resolvers:  resolvers,
		Params:     params,
	}, nil
}
