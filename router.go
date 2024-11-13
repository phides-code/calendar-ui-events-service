package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/go-playground/validator"
)

type ResponseStructure struct {
	Data         interface{} `json:"data"`
	ErrorMessage *string     `json:"errorMessage"` // can be string or nil
}

var validate *validator.Validate = validator.New()

var headers = map[string]string{
	"Access-Control-Allow-Origin":  OriginURL,
	"Access-Control-Allow-Headers": "Content-Type",
}

func router(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("router() received " + req.HTTPMethod + " request")

	providedApiKey := req.Headers["x-api-key"]

	// x-api-key shows up in Camel-Case when run in SAM for some reason
	if providedApiKey == "" {
		providedApiKey = req.Headers["X-Api-Key"]
	}

	apiKey, err := getApiKey()

	if err != nil {
		log.Printf("router() error running getApiKey(): %v", err)
		return serverError(err)
	}

	if apiKey == nil {
		errorMessage := "router() error: apiKey not found"
		log.Println(errorMessage)
		return serverError(errors.New(errorMessage))
	}

	if providedApiKey != *apiKey {
		log.Println("router() error: apiKey mismatch")
		return clientError(http.StatusUnauthorized)
	}

	switch req.HTTPMethod {
	case "GET":
		return processGet(ctx, req)
	case "POST":
		return processPost(ctx, req)
	case "DELETE":
		return processDelete(ctx, req)
	case "PUT":
		return processPut(ctx, req)
	case "OPTIONS":
		return processOptions()
	default:
		log.Println("router() error parsing HTTP method")
		return clientError(http.StatusMethodNotAllowed)
	}
}

func processOptions() (events.APIGatewayProxyResponse, error) {
	additionalHeaders := map[string]string{
		"Access-Control-Allow-Methods": "OPTIONS, POST, GET, PUT, DELETE",
		"Access-Control-Max-Age":       "3600",
	}
	mergedHeaders := mergeHeaders(headers, additionalHeaders)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    mergedHeaders,
	}, nil
}

func processGet(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, idPresent := req.PathParameters["id"]
	if idPresent {
		return processGetEntityById(ctx, id)
	} else {
		return processGetAll(ctx)
	}
}

func processGetEntityById(ctx context.Context, id string) (events.APIGatewayProxyResponse, error) {
	log.Println("running processGetEntityById: " + id)

	entity, err := getEntity(ctx, id)
	if err != nil {
		return serverError(err)
	}

	if entity == nil {
		return clientError(http.StatusNotFound)
	}

	response := ResponseStructure{
		Data:         entity,
		ErrorMessage: nil,
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println("processGetEntityById() error running json.Marshal")
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(responseJson),
		Headers:    headers,
	}, nil
}

func processGetAll(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	log.Println("running processGetAll")

	entities, err := listEntities(ctx)
	if err != nil {
		return serverError(err)
	}

	response := ResponseStructure{
		Data:         entities,
		ErrorMessage: nil,
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println("processGetAll() error running json.Marshal")
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(responseJson),
		Headers:    headers,
	}, nil
}

func processPost(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("running processPost()")

	var createdEntity NewEntity
	err := json.Unmarshal([]byte(req.Body), &createdEntity)
	if err != nil {
		log.Printf("Can't unmarshal body: %v", err)
		return clientError(http.StatusUnprocessableEntity)
	}

	err = validate.Struct(&createdEntity)
	if err != nil {
		log.Printf("Invalid body: %v", err)
		return clientError(http.StatusBadRequest)
	}

	entity, err := insertEntity(ctx, createdEntity)
	if err != nil {
		return serverError(err)
	}

	response := ResponseStructure{
		Data:         entity,
		ErrorMessage: nil,
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println("processPost() error running json.Marshal")
		return serverError(err)
	}

	additionalHeaders := map[string]string{
		"Location": fmt.Sprintf("/%s/%s", ApiPath, entity.Id),
	}
	mergedHeaders := mergeHeaders(headers, additionalHeaders)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(responseJson),
		Headers:    mergedHeaders,
	}, nil
}

func processPut(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, idPresent := req.PathParameters["id"]
	if !idPresent {
		log.Println("processPut() error reading req.PathParameters[\"id\"]")
		return clientError(http.StatusBadRequest)
	}

	log.Println("running processPut with id: " + id)

	var updatedEntity UpdatedEntity
	err := json.Unmarshal([]byte(req.Body), &updatedEntity)
	if err != nil {
		log.Printf("Error unmarshalling body: %v", err)
		return clientError(http.StatusUnprocessableEntity)
	}

	err = validate.Struct(&updatedEntity)
	if err != nil {
		log.Printf("Error validating body: %v", err)
		return clientError(http.StatusBadRequest)
	}

	entity, err := updateEntity(ctx, id, updatedEntity)
	if err != nil {
		return serverError(err)
	}

	if entity == nil {
		return clientError(http.StatusNotFound)
	}

	response := ResponseStructure{
		Data:         entity,
		ErrorMessage: nil,
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println("processPut() error running json.Marshal")
		return serverError(err)
	}

	additionalHeaders := map[string]string{
		"Location": fmt.Sprintf("/%s/%s", ApiPath, entity.Id),
	}
	mergedHeaders := mergeHeaders(headers, additionalHeaders)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(responseJson),
		Headers:    mergedHeaders,
	}, nil
}

func processDelete(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id, idPresent := req.PathParameters["id"]
	if !idPresent {
		log.Println("processDelete() error reading req.PathParameters[\"id\"]")
		return clientError(http.StatusBadRequest)
	}
	log.Println("running processDelete on id: " + id)

	entity, err := deleteEntity(ctx, id)
	if err != nil {
		return serverError(err)
	}

	if entity == nil {
		return clientError(http.StatusNotFound)
	}

	response := ResponseStructure{
		Data:         entity,
		ErrorMessage: nil,
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println("processDelete() error running json.Marshal")
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(responseJson),
		Headers:    headers,
	}, nil
}