package handler

import "github.com/aws/aws-sdk-go/aws/request"

// RequestStructuredLogHandler emf header
var RequestStructuredLogHandler = request.NamedHandler{Name: "RequestStructuredLogHandler", Fn: AddStructuredLogHeader}

// AddStructuredLogHeader emf log
func AddStructuredLogHeader(req *request.Request) {
	req.HTTPRequest.Header.Set("x-amzn-logs-format", "json/emf")
}