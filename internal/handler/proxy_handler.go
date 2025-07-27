package handler

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"

	"vertigo/internal/proxy"

	"github.com/sirupsen/logrus"
)

// NewProxyHandler creates a new reverse proxy handler.
func NewProxyHandler(keyRotator *proxy.KeyRotator, log *logrus.Logger) http.HandlerFunc {
	target, _ := url.Parse("https://generativelanguage.googleapis.com")

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	reverseProxy.Director = func(req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorf("Failed to read request body: %v", err)
			return
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		// Select the model and get the modified body
		_, modifiedBody, err := proxy.SelectModel(body)
		if err != nil {
			log.Errorf("Failed to select model: %v", err)
			return
		}

		// Use the modified body for the outgoing request
		req.Body = ioutil.NopCloser(bytes.NewBuffer(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))

		// Set the API key
		apiKey := keyRotator.GetNextKey()
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// Set the target URL
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = "/v1beta/openai/chat/completions"
		req.Host = target.Host
	}

	return func(w http.ResponseWriter, r *http.Request) {
		reverseProxy.ServeHTTP(w, r)
	}
}
