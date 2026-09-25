// Copyright 2026 Bahonio
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestProbeHealth(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://127.0.0.1/status" {
			t.Fatalf("request URL = %q", request.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}

	if err := probeHealth(client, "http://127.0.0.1/status"); err != nil {
		t.Fatalf("probeHealth() error = %v", err)
	}
}

func TestProbeHealthRejectsNonOK(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       io.NopCloser(strings.NewReader("not ready")),
		}, nil
	})}

	err := probeHealth(client, "http://127.0.0.1/status")
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("probeHealth() error = %v, want HTTP 503", err)
	}
}

func TestRunHealthcheckRejectsInvalidPort(t *testing.T) {
	t.Setenv("WEB_UI_PORT", "not-a-port")
	if err := runHealthcheck(); err == nil {
		t.Fatal("runHealthcheck() error = nil, want invalid port error")
	}
}
