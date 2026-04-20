/*
 * Copyright (c) 2025-2026. TxnLab Inc.
 * All Rights reserved.
 */

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TxnLab/nfd-did/internal/did"
)

type mockResolver struct {
	result   *did.ResolutionResult
	err      error
	derefRes *did.DereferencingResult
	derefErr error
}

func (m *mockResolver) Resolve(_ context.Context, _ string) (*did.ResolutionResult, error) {
	return m.result, m.err
}

func (m *mockResolver) Dereference(_ context.Context, _ string, _ string) (*did.DereferencingResult, error) {
	return m.derefRes, m.derefErr
}

func newResolveRequest(didStr, accept string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/1.0/identifiers/"+didStr, nil)
	req.SetPathValue("did", didStr)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return req
}

func TestHandleHealth_ContentTypeUTF8(t *testing.T) {
	rec := httptest.NewRecorder()
	handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleProperties_ContentTypeUTF8(t *testing.T) {
	rec := httptest.NewRecorder()
	handleProperties(rec, httptest.NewRequest(http.MethodGet, "/1.0/properties", nil))

	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWriteError_ContentTypeUTF8(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "boom")

	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleResolve_ContentTypeUTF8(t *testing.T) {
	canned := &did.ResolutionResult{
		DIDDocument: &did.DIDDocument{ID: "did:nfd:test.algo"},
		ResolutionMetadata: did.ResolutionMetadata{
			ContentType: did.ContentTypeDIDJSON, // bare, per W3C
		},
	}

	tests := []struct {
		name          string
		accept        string
		wantHeader    string
		wantBodyCtype string
	}{
		{
			name:          "default Accept → did+json with charset",
			accept:        "",
			wantHeader:    "application/did+json; charset=utf-8",
			wantBodyCtype: did.ContentTypeDIDJSON,
		},
		{
			name:          "Accept did+ld+json → ld+json with charset",
			accept:        did.ContentTypeDIDLDJSON,
			wantHeader:    "application/did+ld+json; charset=utf-8",
			wantBodyCtype: did.ContentTypeDIDJSON, // body metadata mirrors resolver; bare either way
		},
		{
			name:          "Accept did+ld+json with existing charset still negotiates",
			accept:        "application/did+ld+json; charset=utf-8",
			wantHeader:    "application/did+ld+json; charset=utf-8",
			wantBodyCtype: did.ContentTypeDIDJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockResolver{result: canned}
			rec := httptest.NewRecorder()

			handleResolve(m)(rec, newResolveRequest("did:nfd:test.algo", tt.accept))

			assert.Equal(t, tt.wantHeader, rec.Header().Get("Content-Type"))
			assert.Equal(t, http.StatusOK, rec.Code)

			var parsed struct {
				DIDResolutionMetadata struct {
					ContentType string `json:"contentType"`
				} `json:"didResolutionMetadata"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
			assert.Equal(t, tt.wantBodyCtype, parsed.DIDResolutionMetadata.ContentType,
				"body contentType must stay bare (no charset) per W3C DID Resolution §7")
		})
	}
}

func TestHandleResolve_ErrorResponse_ContentTypeUTF8(t *testing.T) {
	errResult := did.ErrorResult(did.ErrorNotFound, did.ContentTypeDIDJSON)
	m := &mockResolver{result: errResult, err: errors.New("not found")}

	rec := httptest.NewRecorder()
	handleResolve(m)(rec, newResolveRequest("did:nfd:missing.algo", ""))

	assert.Equal(t, "application/did+json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
