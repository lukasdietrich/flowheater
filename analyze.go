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
	Service     *Service
	FuncName    string
	Path        string
	HttpMethod  string
	InputVars   []InputVar
	InputParams InputParamSlice
}

func (e *Endpoint) WrapperFunc() string {
	return fmt.Sprintf("_handle_%s_%s", e.Service.TypeName, e.FuncName)
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
	ParamKind   int
	ParamName   string
	VarName     string
	TypeName    string
	TypePackage string
	InputVars   []InputVar
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

func (i *InputParamSlice) resolveParams(decls []ParamDeclaration) ([]InputVar, error) {
	var inputVars []InputVar

	for _, decl := range decls {
		inputVar, err := i.resolveParam(decl)
		if err != nil {
			return nil, err
		}

		inputVars = append(inputVars, *inputVar)
	}

	return inputVars, nil
}

func (i *InputParamSlice) resolveParam(decl ParamDeclaration) (*InputVar, error) {
	if decl.PointerDepth() > 1 {
		return nil, fmt.Errorf("%s: pointers of pointers are not supported", decl.Name())
	}

	if decl.TypePackage() == "net/http" {
		if decl.TypeName() == "Request" && decl.PointerDepth() == 1 {
			return &InputVar{VarName: "r"}, nil
		}

		if decl.TypeName() == "ResponseWriter" {
			return &InputVar{VarName: "w"}, nil
		}
	}

	if decl.IsBuiltIn() {
		return i.resolveBuiltinParam(decl)
	} else {
		return i.resolvePayloadParam(decl)
	}

	return nil, fmt.Errorf("could not resolve param '%s %s'", decl.Name(), decl.TypeName())
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
			ParamKind: kind,
			ParamName: decl.Name(),
			TypeName:  decl.TypeName(),
			InputVars: []InputVar{{VarName: varName}},
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
		ParamKind:   KindPayloadParam,
		ParamName:   decl.Name(),
		TypeName:    decl.TypeName(),
		TypePackage: decl.TypePackage(),
	})

	return &InputVar{
		VarName:      varName,
		PointerDepth: decl.PointerDepth(),
	}, nil
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
		inputVars   []InputVar
		inputParams InputParamSlice
		httpMethod  = http.MethodGet
	)

	if declaration.Annotations().Exists(aMethod) {
		httpMethod = strings.ToUpper(declaration.Annotations().Get(aMethod))
	}

	inputVars, err := inputParams.resolveParams(declaration.InputParams())
	if err != nil {
		return nil, err
	}

	inputParams.movePayloadLast()

	return &Endpoint{
		FuncName:    declaration.Name(),
		Path:        declaration.Path(),
		HttpMethod:  httpMethod,
		InputVars:   inputVars,
		InputParams: inputParams,
	}, nil
}
