package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
)

// RouterParameters is used to retrieve router request context values
type RouterParameters int

// ParametersKey is the key used in request context to retrieve route parameters
const ParametersKey RouterParameters = iota

const pathDelimiter string = "/"

var (
	// ErrNoHandlerFound happens when no suitable handlers is found to match given request
	ErrNoHandlerFound = errors.New("no http handler found")
	// ErrPathAlreadyRegistered happens when the path given is already registered
	ErrPathAlreadyRegistered = errors.New("path already registered")

	errRuleDoesNotMatch        = errors.New("routing rule does not match given request")
	errMethodMismatch          = errors.New("method mismatch")
	errPathMismatch            = errors.New("path mismatch")
	errPathMatchMethodMismatch = errors.New("path match, method mismatch")
)

// NewRouter returns a new router instance
func NewRouter() Router {
	return Router{
		routes: []routingRule{},
	}
}

type routingRule struct {
	Pattern string
	Method  string
	Handler http.HandlerFunc
}

// Router is the main routing component
type Router struct {
	routes []routingRule
	logger *log.Logger
}

func (r Router) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	handler, matches, err := r.determineHandler(request)
	if err == nil {
		ctx := context.WithValue(request.Context(), ParametersKey, matches)
		request = request.WithContext(ctx)

		handler(w, request)
		return
	}

	if err == ErrNoHandlerFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err == errPathMatchMethodMismatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}

func cleanPath(path string) string {
	return strings.TrimRight(deduplicateDelimiter(path), "/")
}

func deduplicateDelimiter(path string) string {
	path = strings.ReplaceAll(path, "//", "/")
	if strings.Count(path, "//") > 0 {
		return deduplicateDelimiter(path)
	}
	return path
}

func (r *Router) determineHandler(request *http.Request) (http.HandlerFunc, map[string]string, error) {
	var pathMatched bool
	for _, rule := range r.routes {
		err, parameters := r.match(*request, rule)

		if err == nil {
			return rule.Handler, parameters, nil
		}

		if err == errMethodMismatch {
			pathMatched = true
		}

	}

	if pathMatched {
		return nil, nil, errPathMatchMethodMismatch
	}

	return nil, nil, ErrNoHandlerFound
}

func (r *Router) match(request http.Request, rule routingRule) (error, map[string]string) {
	requestPath := cleanPath(request.URL.Path)
	splitRulePattern := strings.Split(rule.Pattern, pathDelimiter)[1:]
	splitRequestPath := strings.Split(requestPath, pathDelimiter)[1:]

	parameters := make(map[string]string, strings.Count(requestPath, ":"))

	for index, value := range splitRequestPath {
		if len(splitRulePattern) < index+1 {
			return errPathMismatch, nil
		}

		pattern := splitRulePattern[index]

		if pattern[0] == ':' {
			parameters[pattern[1:]] = value
			continue
		}

		if value != pattern {
			return errPathMismatch, nil
		}
	}

	if request.Method != rule.Method {
		return errMethodMismatch, nil
	}

	return nil, parameters
}

func (r *Router) Register(method string, path string, handler http.HandlerFunc) error {
	if r.isPathAlreadyRegistered(path) {
		return ErrPathAlreadyRegistered
	}

	newRoute := routingRule{
		Pattern: path,
		Method:  method,
		Handler: handler,
	}

	r.routes = append(r.routes, newRoute)
	return nil
}

func (r *Router) isPathAlreadyRegistered(path string) bool {
	for _, route := range r.routes {
		if route.Pattern == path {
			return true
		}
	}
	return false
}
