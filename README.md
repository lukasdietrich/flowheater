# Flowheater

Flowheater is an annotation based router generator for go with automatic
resolution of parameters as well as marshalling and unmarshalling of requests
and responses.

## Installing

Install `flowheater` by running:

```
$ go get github.com/lukasdietrich/flowheater
```

Ensure that `$GOPATH/bin` is added to your `$PATH`.

## Usage

Flowheater works by scanning a package for annotated structs and methods.
Since go does not support annotations, flowheater uses a special comment syntax:
All trailing lines of the format `Key: Value` are considered an annotation.

### Service

First flowheater looks for struct types with a `Path` annotation.
Each such struct is then traversed for methods with another `Path` annotation.
Every annotated method of an annotated struct is considered a service endpoint
and will be registered in the generated router with a joined path of the struct
and the method.

Endpoints also support an optional `Method` annotation, which indicates the
expected http-method. If no such annotation is present, `GET` will be used as
a default.

### Parameters

A service endpoint does not have to manually extract path-parameters or 
unmarshal a request. Instead you can directly declare function parameters and
flowheater will try its best to resolve them.

#### Native parameter

There are three special types, that you can declare and flowheater injects them
as is. `*net/http.Request` and `net/http.ResponseWriter` are directly passed
forward from the `net/http.Handler` interface. Additionally `context.Context` is
resolved as the requests context.

#### Path parameters

Builtin types (string, int, int..., uint, uint..., bool) are extracted and
converted if necessary using the methods parameter name as the path-parameter
name.

#### Resolver

Flowheater supports a third and very powerful type of parameters to be
automatically resolved.

You can define a struct with a method named `resolveParam` that returns a value
of any type. That type can then be declared as a parameter in service endpoints
and flowheater will resolve the value using the resolver struct type.
The `resolveParam` method in turn can declare any kind of parameters.
This can be used to inject values otherwise implemented via middlewares, for
example a user inferred from a cookie.

#### Payload

Flowheater tries to resolve parameters as either a native, path or resolver
parameter.
If none of these is applicable, it is assumed to be the request payload.
Only one parameter can be the payload. If more than one parameter is assumed
to be the payload, flowheater will complain accordingly.

### Response

Both service endpoints and custom resolvers can or must return values.
A resolver obviously must return the value to be resolved, but can also
optionaly return an error. An endpoint can return nothing, a response, an error
or both. The returned value will be marshalled and writted to the responsewriter
if no error occured.

