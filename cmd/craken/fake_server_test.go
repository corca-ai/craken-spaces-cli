package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type fakeOperation struct {
	Status int
	Body   any
	Assert func(t *testing.T, req *http.Request, body []byte)
}

type contractFakeServer struct {
	server *httptest.Server
}

func newContractFakeServer(t *testing.T, operations map[string]fakeOperation) *contractFakeServer {
	t.Helper()
	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: false}
	doc, err := loader.LoadFromFile(filepath.Join("..", "..", "protocol", "public-api-v1.openapi.yaml"))
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	// The checked-in contract snapshot uses the public production base URL,
	// but unit tests exercise handlers through a local httptest server.
	doc.Servers = openapi3.Servers{{URL: "http://127.0.0.1"}}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("OpenAPI validate failed: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleContractRequest(t, router, operations, w, r)
	}))
	t.Cleanup(server.Close)
	return &contractFakeServer{server: server}
}

func handleContractRequest(t *testing.T, router routers.Router, operations map[string]fakeOperation, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("ReadAll request body failed: %v", err)
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	route, pathParams, err := router.FindRoute(r)
	if err != nil {
		t.Fatalf("FindRoute failed for %s %s: %v", r.Method, r.URL.Path, err)
	}
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: contractAuthenticationFunc,
		},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), requestValidationInput); err != nil {
		t.Fatalf("ValidateRequest failed for %s %s: %v", r.Method, r.URL.Path, err)
	}

	operationID := route.Operation.OperationID
	operation, ok := operations[operationID]
	if !ok {
		t.Fatalf("no fake response registered for operation %q", operationID)
	}
	if operation.Assert != nil {
		operation.Assert(t, r, bodyBytes)
	}

	status := operation.Status
	if status == 0 {
		status = http.StatusOK
	}
	var responseBody []byte
	if operation.Body != nil {
		responseBody, err = json.Marshal(operation.Body)
		if err != nil {
			t.Fatalf("json.Marshal response failed for operation %q: %v", operationID, err)
		}
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if len(responseBody) > 0 {
		if _, err := w.Write(responseBody); err != nil {
			t.Fatalf("Write response failed for operation %q: %v", operationID, err)
		}
	}

	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 status,
		Header:                 w.Header(),
	}
	responseValidationInput.SetBodyBytes(responseBody)
	if err := openapi3filter.ValidateResponse(context.Background(), responseValidationInput); err != nil {
		t.Fatalf("ValidateResponse failed for %s %s: %v", r.Method, r.URL.Path, err)
	}
}

func contractAuthenticationFunc(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	if strings.TrimSpace(input.RequestValidationInput.Request.Header.Get("Authorization")) == "" {
		return errors.New("missing Authorization header")
	}
	return nil
}
