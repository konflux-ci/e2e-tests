package gohooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
)

const (
	DefaultSignatureHeader = "X-GoHooks-Verification"
)

// GoHook represents the definition of a GoHook.
type GoHook struct {
	// Data to be sent in the GoHook
	Payload GoHookPayload
	// The encrypted SHA resulting with the used salt
	ResultingSha string
	// Prepared JSON marshaled data
	PreparedData []byte
	// Choice of signature header to use on sending a GoHook
	SignatureHeader string
	// Should validate SSL certificate
	IsSecure bool
	// Preferred HTTP method to send the GoHook
	// Please choose only POST, DELETE, PATCH or PUT
	// Any other value will make the send use POST as fallback
	PreferredMethod string
	// Additional HTTP headers to be added to the hook
	AdditionalHeaders map[string]string
	// Span for distributed tracing
	Span *opentracing.Span
}

// GoHookPayload represents the data that will be sent in the GoHook.
type GoHookPayload struct {
	Resource string      `json:"resource"`
	Data     interface{} `json:"data"`
}

// Create creates a webhook to be sent to another system,
// with a SHA 256 signature based on its contents.
func (hook *GoHook) Create(data interface{}, resource, secret string) {
	hook.Payload.Resource = resource
	hook.Payload.Data = data

	preparedHookData, err := json.Marshal(hook.Payload)
	if err != nil {
		log.Println(err.Error())
	}

	hook.PreparedData = preparedHookData

	h := hmac.New(sha256.New, []byte(secret))

	_, err = h.Write(preparedHookData)
	if err != nil {
		log.Println(err.Error())
	}

	// Get result and encode as hexadecimal string
	hook.ResultingSha = hex.EncodeToString(h.Sum(nil))
}

// CreateCreateWithoutWrapper creates a webhook to be sent to another system,
// without wrapping it in a resource - data struct, with a SHA 256 signature based on its contents.
func (hook *GoHook) CreateWithoutWrapper(data interface{}, secret string) {
	preparedHookData, err := json.Marshal(data)
	if err != nil {
		log.Println(err.Error())
	}

	hook.PreparedData = preparedHookData

	h := hmac.New(sha256.New, []byte(secret))

	_, err = h.Write(preparedHookData)
	if err != nil {
		log.Println(err.Error())
	}

	// Get result and encode as hexadecimal string
	hook.ResultingSha = hex.EncodeToString(h.Sum(nil))
}

// Send sends a GoHook to the specified URL, as a UTF-8 JSON payload.
func (hook *GoHook) Send(receiverURL string) (*http.Response, error) {
	if hook.SignatureHeader == "" {
		// Use the DefaultSignatureHeader as default if no custom header is specified
		hook.SignatureHeader = DefaultSignatureHeader
	}

	if !hook.IsSecure {
		// By default do not verify SSL certificate validity
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		}
	}

	switch hook.PreferredMethod {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		// Valid Methods, do nothing
	default:
		// By default send GoHook using a POST method
		hook.PreferredMethod = http.MethodPost
	}

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(
		hook.PreferredMethod,
		receiverURL,
		bytes.NewBuffer(hook.PreparedData),
	)
	if err != nil {
		return nil, err
	}

	if hook.Span != nil {
		ctx := opentracing.ContextWithSpan(context.Background(), *hook.Span)

		req = req.WithContext(ctx)

		_ = InjectRequestContext(*hook.Span, req)
	} else {
		req = req.WithContext(context.Background())
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Charset", "utf-8")
	req.Header.Add(DefaultSignatureHeader, hook.ResultingSha)

	// Add user's additional headers
	for i := range hook.AdditionalHeaders {
		req.Header.Add(i, hook.AdditionalHeaders[i])
	}

	req.Close = true

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func InjectRequestContext(span opentracing.Span, request *http.Request) error {
	return span.Tracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(request.Header))
}
