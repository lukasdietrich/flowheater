package main

import (
	"fmt"
	"net/http"
	"strings"
)

type ServiceCollection struct {
	PackageName string
	Services    []*Service
	Resolvers   []Resolver
}

func AnalyzePackage(source *SourcePackage) (*ServiceCollection, error) {
	resolvableTypes, err := analyzeResolvers(source.resolvers)
	if err != nil {
		return nil, err
	}

	var services []*Service

	for _, serviceDeclaration := range source.Services() {
		service, err := analyzeService(serviceDeclaration, resolvableTypes)
		if err != nil {
			return nil, fmt.Errorf("analyzing service %s: %v",
				serviceDeclaration.Name(), err)
		}

		services = append(services, service)
	}

	return &ServiceCollection{
		PackageName: source.Name(),
		Services:    services,
		Resolvers:   findUsedResolvers(services),
	}, nil
}

func findUsedResolvers(services []*Service) []Resolver {
	var (
		resolverNameSet = make(map[string]bool)
		resolverSlice   []Resolver
	)

	for _, service := range services {
		for _, endpoint := range service.Endpoints {
			for _, param := range endpoint.InputParams {
				if param.ParamKind == KindResolveParam {
					if !resolverNameSet[param.Resolver] {
						resolverNameSet[param.Resolver] = true
						resolverSlice = append(resolverSlice, Resolver{
							TypeName: param.Resolver,
						})
					}
				}
			}
		}
	}

	return resolverSlice
}

type Service struct {
	TypeName  string
	Path      string
	Endpoints []*Endpoint
}

type Endpoint struct {
	Service      *Service
	FuncName     string
	Path         string
	HttpMethod   string
	InputVars    []InputVar
	InputParams  InputParamSlice
	ReturnsValue bool
	ReturnsError bool
}

func (e *Endpoint) WrapperFunc() string {
	return fmt.Sprintf("_handle_%s_%s", e.Service.TypeName, e.FuncName)
}

type Resolver struct {
	TypeName string
}

type ResolvableType struct {
	TypeName    string
	TypePackage string

	Resolver       ResolverDeclaration
	ReturnsError   bool
	ReturnsPointer int
}

type ResolvableSlice []ResolvableType

func analyzeResolvers(declarations []ResolverDeclaration) (ResolvableSlice, error) {
	var r ResolvableSlice

	for _, decl := range declarations {
		rt, err := analyzeResolver(decl)
		if err != nil {
			return nil, fmt.Errorf("analyzing resolver %s: %v", decl.Name(), err)
		}

		r = append(r, *rt)
	}

	return r, nil
}

func analyzeResolver(decl ResolverDeclaration) (*ResolvableType, error) {
	values := decl.OutputParams()

	switch len(values) {
	case 1:
		if isErrorParam(values[0]) {
			return nil, fmt.Errorf("resolver must return a non-error value")
		}

	case 2:
		if !isErrorParam(values[1]) {
			return nil, fmt.Errorf("second return value must be an error")
		}

	default:
		return nil, fmt.Errorf("a resolver can only return 1 or 2 values: a value or a value and an error")
	}

	return &ResolvableType{
		TypeName:       values[0].TypeName(),
		TypePackage:    values[0].TypePackage(),
		Resolver:       decl,
		ReturnsError:   len(values) > 1,
		ReturnsPointer: values[0].PointerDepth(),
	}, nil
}

func (r ResolvableSlice) FindResolver(param ParamDeclaration) (*ResolvableType, bool) {
	for _, rt := range r {
		if rt.TypeName == param.TypeName() && rt.TypePackage == param.TypePackage() {
			return &rt, true
		}
	}

	return nil, false
}

type InputVar struct {
	VarName      string
	PointerDepth int
}

const (
	_                = iota
	KindStringParam  // Directly extract param from path
	KindConvertParam // Convert string to builtin type
	KindResolveParam // Resolve param from other params and/or dependencies
	KindPayloadParam // Unmarshal payload into param
)

type InputParam struct {
	ParamKind    int
	ParamName    string // Name of the param as declared on a method
	VarName      string // The assigned var name of the resolved value
	TypeName     string
	TypePackage  string
	InputVars    []InputVar
	Resolver     string
	ReturnsError bool
}

type InputParamSlice []InputParam

func (i *InputParamSlice) movePayloadLast() {
	for k, p := range *i {
		if p.ParamKind == KindPayloadParam {
			slice := *i

			slice = append(slice[:k], slice[k+1:]...)
			slice = append(slice, p)

			*i = slice
			break
		}
	}
}

func (i *InputParamSlice) appendParam(param InputParam) string {
	param.VarName = fmt.Sprintf("param%d", len(*i))
	*i = append(*i, param)
	return param.VarName
}

func (i *InputParamSlice) resolveParams(decls []ParamDeclaration, resolvables ResolvableSlice) ([]InputVar, error) {
	var inputVars []InputVar

	for _, decl := range decls {
		inputVar, err := i.resolveParam(decl, resolvables)
		if err != nil {
			return nil, err
		}

		inputVars = append(inputVars, *inputVar)
	}

	return inputVars, nil
}

func (i *InputParamSlice) resolveParam(decl ParamDeclaration, resolvables ResolvableSlice) (*InputVar, error) {
	if decl.PointerDepth() > 1 {
		return nil, fmt.Errorf("%s: pointers of pointers are not supported", decl.Name())
	}

	if v := i.resolveNativeParam(decl); v != nil {
		return v, nil
	}

	if rt, ok := resolvables.FindResolver(decl); ok {
		return i.resolveResolvableParam(decl, rt, resolvables)
	}

	if decl.IsBuiltIn() {
		return i.resolveBuiltinParam(decl)
	} else {
		return i.resolvePayloadParam(decl)
	}

	return nil, fmt.Errorf("could not resolve param '%s %s'", decl.Name(), decl.TypeName())
}

func (i *InputParamSlice) resolveNativeParam(decl ParamDeclaration) *InputVar {
	if decl.TypePackage() == "net/http" {
		if decl.TypeName() == "Request" && decl.PointerDepth() == 1 {
			return &InputVar{VarName: "r"}
		}

		if decl.TypeName() == "ResponseWriter" && decl.PointerDepth() == 0 {
			return &InputVar{VarName: "w"}
		}
	}

	if decl.TypePackage() == "context" {
		if decl.TypeName() == "Context" && decl.PointerDepth() == 0 {
			return &InputVar{VarName: "r.Context()"}
		}
	}

	return nil
}

func (i *InputParamSlice) resolveResolvableParam(decl ParamDeclaration, rt *ResolvableType, resolvables ResolvableSlice) (*InputVar, error) {
	var (
		exists  bool
		varName string
	)

	for _, p := range *i {
		if p.ParamKind == KindResolveParam && p.TypeName == rt.TypeName && p.TypePackage == rt.TypePackage {
			exists = true
			varName = p.VarName
			break
		}
	}

	if !exists {
		inputVars, err := i.resolveParams(rt.Resolver.InputParams(), resolvables)
		if err != nil {
			return nil, err
		}

		varName = i.appendParam(InputParam{
			ParamKind:    KindResolveParam,
			ParamName:    decl.Name(),
			TypeName:     rt.TypeName,
			TypePackage:  rt.TypePackage,
			InputVars:    inputVars,
			Resolver:     rt.Resolver.Name(),
			ReturnsError: rt.ReturnsError,
		})
	}

	return &InputVar{
		VarName:      varName,
		PointerDepth: decl.PointerDepth() - rt.ReturnsPointer,
	}, nil
}

func (i *InputParamSlice) resolveBuiltinParam(decl ParamDeclaration) (*InputVar, error) {
	var (
		existStringVar  bool
		existConvertVar bool
		varName         string
		kind            int
	)

	if decl.TypeName() == "string" {
		kind = KindStringParam
	} else {
		kind = KindConvertParam
	}

	for _, p := range *i {
		if p.ParamKind == KindStringParam && p.ParamName == decl.Name() {
			varName = p.VarName
			existStringVar = true
			break
		}
	}

	if !existStringVar {
		varName = i.appendParam(InputParam{
			ParamKind: KindStringParam,
			ParamName: decl.Name(),
			TypeName:  "string",
		})
	}

	if kind == KindStringParam {
		return &InputVar{
			VarName:      varName,
			PointerDepth: decl.PointerDepth(),
		}, nil
	}

	for _, p := range *i {
		if p.ParamKind == KindConvertParam &&
			p.ParamName == decl.Name() && p.TypeName == decl.TypeName() {
			varName = p.VarName
			existConvertVar = true
			break
		}
	}

	if !existConvertVar {
		varName = i.appendParam(InputParam{
			ParamKind:    kind,
			ParamName:    decl.Name(),
			TypeName:     decl.TypeName(),
			InputVars:    []InputVar{{VarName: varName}},
			ReturnsError: true,
		})
	}

	return &InputVar{
		VarName:      varName,
		PointerDepth: decl.PointerDepth(),
	}, nil
}

func (i *InputParamSlice) resolvePayloadParam(decl ParamDeclaration) (*InputVar, error) {
	for _, p := range *i {
		if p.ParamKind == KindPayloadParam {
			if p.TypeName == decl.TypeName() && p.TypePackage == decl.TypePackage() {
				return &InputVar{
					VarName:      p.VarName,
					PointerDepth: decl.PointerDepth(),
				}, nil
			}

			return nil, fmt.Errorf("cannot resolve param %s, because %s is already claiming the payload",
				decl.Name(), p.ParamName)
		}
	}

	varName := i.appendParam(InputParam{
		ParamKind:    KindPayloadParam,
		ParamName:    decl.Name(),
		TypeName:     decl.TypeName(),
		TypePackage:  decl.TypePackage(),
		ReturnsError: true,
	})

	return &InputVar{
		VarName:      varName,
		PointerDepth: decl.PointerDepth(),
	}, nil
}

func analyzeService(decl ServiceDeclaration, resolvables ResolvableSlice) (*Service, error) {
	var (
		endpoints []*Endpoint
		service   = Service{
			TypeName: decl.Name(),
			Path:     decl.Path(),
		}
	)

	for _, endpointDeclaration := range decl.Endpoints() {
		endpoint, err := analyzeEndpoint(endpointDeclaration, resolvables)
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

func analyzeEndpoint(decl EndpointDeclaration, resolvables ResolvableSlice) (*Endpoint, error) {
	var (
		inputVars   []InputVar
		inputParams InputParamSlice
		httpMethod  = http.MethodGet
	)

	if decl.Annotations().Exists(aMethod) {
		httpMethod = strings.ToUpper(decl.Annotations().Get(aMethod))
	}

	inputVars, err := inputParams.resolveParams(decl.InputParams(), resolvables)
	if err != nil {
		return nil, err
	}

	inputParams.movePayloadLast()

	returnsValue, returnsError, err := analyzeEndpointOutput(decl.OutputParams())
	if err != nil {
		return nil, err
	}

	return &Endpoint{
		FuncName:     decl.Name(),
		Path:         decl.Path(),
		HttpMethod:   httpMethod,
		InputVars:    inputVars,
		InputParams:  inputParams,
		ReturnsValue: returnsValue,
		ReturnsError: returnsError,
	}, nil
}

func analyzeEndpointOutput(params []ParamDeclaration) (bool, bool, error) {
	switch len(params) {
	case 0:
		return false, false, nil

	case 1:
		isErr := isErrorParam(params[0])
		return !isErr, isErr, nil

	case 2:
		if !isErrorParam(params[1]) {
			return false, false, fmt.Errorf(
				"the second return value must be of type error")
		}

		return true, true, nil

	default:
		return false, false, fmt.Errorf(
			"an endpoint can only return 1 or 2 values: either a value, an error or both")
	}
}

func isErrorParam(param ParamDeclaration) bool {
	return param.IsBuiltIn() && param.TypeName() == "error"
}
