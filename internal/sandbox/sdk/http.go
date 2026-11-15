package sdk

import (
	"encoding/base64"
	"encoding/json"
)

type HTTPRequestOptions struct {
	URL            string            `json:"url"`
	Method         string            `json:"method,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           string            `json:"body,omitempty"`
	BodyBase64     string            `json:"bodyBase64,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds,omitempty"`
	MaxBytes       int64             `json:"maxBytes,omitempty"`
}

type HTTPResponse struct {
	URL       string              `json:"url"`
	Status    int                 `json:"status"`
	Headers   map[string][]string `json:"headers"`
	Body      []byte              `json:"-"`
	Bytes     int                 `json:"bytes"`
	Truncated bool                `json:"truncated"`
}

func HTTPRequest(options HTTPRequestOptions, intent string) (HTTPResponse, error) {
	data, err := json.Marshal(options)
	if err != nil {
		return HTTPResponse{}, err
	}
	var args map[string]any
	if err := json.Unmarshal(data, &args); err != nil {
		return HTTPResponse{}, err
	}
	args["intent"] = intent
	raw, err := callTool("httpRequest", args)
	if err != nil {
		return HTTPResponse{}, err
	}
	var response struct {
		HTTPResponse
		BodyBase64 string `json:"bodyBase64"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return HTTPResponse{}, err
	}
	response.Body, err = base64.StdEncoding.DecodeString(response.BodyBase64)
	return response.HTTPResponse, err
}
