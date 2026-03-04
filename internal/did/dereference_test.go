/*
 * Copyright (c) 2025-2026. TxnLab Inc.
 * All Rights reserved.
 */

package did

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TxnLab/nfd-did/internal/nfd"
)

func TestParseDIDURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantDID    string
		wantFrag   string
		wantParams map[string]string
		wantErr    bool
	}{
		{
			name:       "bare DID",
			input:      "did:nfd:name.algo",
			wantDID:    "did:nfd:name.algo",
			wantFrag:   "",
			wantParams: map[string]string{},
		},
		{
			name:       "with fragment",
			input:      "did:nfd:name.algo#owner",
			wantDID:    "did:nfd:name.algo",
			wantFrag:   "owner",
			wantParams: map[string]string{},
		},
		{
			name:       "with x25519 fragment",
			input:      "did:nfd:name.algo#x25519-owner",
			wantDID:    "did:nfd:name.algo",
			wantFrag:   "x25519-owner",
			wantParams: map[string]string{},
		},
		{
			name:     "with service query",
			input:    "did:nfd:name.algo?service=web",
			wantDID:  "did:nfd:name.algo",
			wantFrag: "",
			wantParams: map[string]string{
				"service": "web",
			},
		},
		{
			name:     "with service and relativeRef",
			input:    "did:nfd:name.algo?service=web&relativeRef=/about",
			wantDID:  "did:nfd:name.algo",
			wantFrag: "",
			wantParams: map[string]string{
				"service":     "web",
				"relativeRef": "/about",
			},
		},
		{
			name:       "segment DID with fragment",
			input:      "did:nfd:mail.name.algo#owner",
			wantDID:    "did:nfd:mail.name.algo",
			wantFrag:   "owner",
			wantParams: map[string]string{},
		},
		{
			name:    "invalid base DID",
			input:   "did:web:example.com#key",
			wantErr: true,
		},
		{
			name:    "invalid NFD name with fragment",
			input:   "did:nfd:INVALID.algo#owner",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDIDURL(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDID, got.DID)
			assert.Equal(t, tt.wantFrag, got.Fragment)
			assert.Equal(t, tt.wantParams, got.Params)
		})
	}
}

// newTestResolver creates a resolver with a standard mock for dereferencing tests.
// Returns a resolver with an NFD that has owner, verified address, services, and profile data.
func newTestResolver() NfdDIDResolver {
	ownerAddr := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY5HFKQ"
	verifiedAddr := "GD64YIY3TWGDMCNPP553DZPPR6LDUSFQOIJVFDPPNRL7NMBER2XNVVTS4I"
	futureExpiry := strconv.FormatUint(uint64(time.Now().Add(365*24*time.Hour).Unix()), 10)

	mock := &mockNfdFetcher{
		didResult: nfd.Properties{
			Internal: map[string]string{
				"name":           "test.algo",
				"owner":          ownerAddr,
				"expirationTime": futureExpiry,
			},
			UserDefined: map[string]string{
				"name":    "Test NFD",
				"bio":     "A test domain",
				"twitter": "testnfd",
			},
			Verified: map[string]string{
				"domain": "https://example.com",
				"caAlgo": verifiedAddr,
			},
		},
		didAppID: 12345,
	}
	return NewNfdDIDResolverWithFetcher(mock, 5*time.Minute)
}

func TestDereference_Fragment_VerificationMethod(t *testing.T) {
	resolver := newTestResolver()

	// Dereference #owner
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#owner", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	vm, ok := result.ContentStream.(VerificationMethod)
	require.True(t, ok, "contentStream should be a VerificationMethod")
	assert.Equal(t, "did:nfd:test.algo#owner", vm.ID)
	assert.Equal(t, KeyTypeEd25519, vm.Type)
	assert.NotEmpty(t, vm.PublicKeyMultibase)
}

func TestDereference_Fragment_VerifiedAddress(t *testing.T) {
	resolver := newTestResolver()

	// Dereference #algo-0
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#algo-0", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	vm, ok := result.ContentStream.(VerificationMethod)
	require.True(t, ok, "contentStream should be a VerificationMethod")
	assert.Equal(t, "did:nfd:test.algo#algo-0", vm.ID)
	assert.Equal(t, KeyTypeEd25519, vm.Type)
}

func TestDereference_Fragment_KeyAgreement(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#x25519-owner", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	ka, ok := result.ContentStream.(VerificationMethod)
	require.True(t, ok, "contentStream should be a VerificationMethod")
	assert.Equal(t, "did:nfd:test.algo#x25519-owner", ka.ID)
	assert.Equal(t, KeyTypeX25519, ka.Type)
}

func TestDereference_Fragment_Service(t *testing.T) {
	resolver := newTestResolver()

	// Dereference #web service
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#web", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	svc, ok := result.ContentStream.(Service)
	require.True(t, ok, "contentStream should be a Service")
	assert.Equal(t, "did:nfd:test.algo#web", svc.ID)
	assert.Equal(t, "LinkedDomains", svc.Type)
	assert.Equal(t, "https://example.com", svc.ServiceEndpoint)
}

func TestDereference_Fragment_SocialMedia(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#twitter", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	svc, ok := result.ContentStream.(Service)
	require.True(t, ok, "contentStream should be a Service")
	assert.Equal(t, "did:nfd:test.algo#twitter", svc.ID)
	assert.Equal(t, "SocialMedia", svc.Type)
	assert.Equal(t, "https://x.com/testnfd", svc.ServiceEndpoint)
}

func TestDereference_Fragment_NotFound(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#nonexistent", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorNotFound, result.DereferencingMetadata.Error)
	assert.Nil(t, result.ContentStream)
}

func TestDereference_ServiceQuery(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo?service=web", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)
	assert.Equal(t, "https://example.com", result.ContentStream)
}

func TestDereference_ServiceQuery_RelativeRef(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo?service=web&relativeRef=/about", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)
	assert.Equal(t, "https://example.com/about", result.ContentStream)
}

func TestDereference_ServiceQuery_RelativeRef_WithTrailingSlash(t *testing.T) {
	ownerAddr := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY5HFKQ"
	futureExpiry := strconv.FormatUint(uint64(time.Now().Add(365*24*time.Hour).Unix()), 10)

	mock := &mockNfdFetcher{
		didResult: nfd.Properties{
			Internal: map[string]string{
				"name":           "test2.algo",
				"owner":          ownerAddr,
				"expirationTime": futureExpiry,
			},
			UserDefined: map[string]string{
				"website": "https://example.com/storage/",
			},
			Verified: map[string]string{},
		},
		didAppID: 12345,
	}
	resolver := NewNfdDIDResolverWithFetcher(mock, 5*time.Minute)

	result, err := resolver.Dereference(context.Background(), "did:nfd:test2.algo?service=web&relativeRef=resume.pdf", ContentTypeDIDJSON)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/storage/resume.pdf", result.ContentStream)
}

func TestDereference_ServiceQuery_NotFound(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo?service=nonexistent", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorNotFound, result.DereferencingMetadata.Error)
}

func TestDereference_ServiceQuery_StructuredEndpoint(t *testing.T) {
	resolver := newTestResolver()

	// The #profile service has a structured NFDProfileEndpoint, not a string
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo?service=profile", ContentTypeDIDJSON)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.DereferencingMetadata.Error)

	// Should return the structured endpoint
	endpoint, ok := result.ContentStream.(NFDProfileEndpoint)
	require.True(t, ok, "contentStream should be NFDProfileEndpoint, got %T", result.ContentStream)
	assert.Equal(t, "Test NFD", endpoint.Name)
	assert.Equal(t, "A test domain", endpoint.Bio)
}

func TestDereference_DeactivatedDID(t *testing.T) {
	ownerAddr := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY5HFKQ"
	// Expired NFD
	pastExpiry := strconv.FormatUint(uint64(time.Now().Add(-24*time.Hour).Unix()), 10)

	mock := &mockNfdFetcher{
		didResult: nfd.Properties{
			Internal: map[string]string{
				"name":           "expired.algo",
				"owner":          ownerAddr,
				"expirationTime": pastExpiry,
			},
			UserDefined: map[string]string{},
			Verified:    map[string]string{},
		},
		didAppID: 12345,
	}
	resolver := NewNfdDIDResolverWithFetcher(mock, 5*time.Minute)

	result, err := resolver.Dereference(context.Background(), "did:nfd:expired.algo#owner", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorNotFound, result.DereferencingMetadata.Error)
}

func TestDereference_InvalidDIDURL(t *testing.T) {
	resolver := newTestResolver()

	result, err := resolver.Dereference(context.Background(), "did:web:example.com#key", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorInvalidDIDURL, result.DereferencingMetadata.Error)
}

func TestDereference_NotFoundDID(t *testing.T) {
	mock := &mockNfdFetcher{
		didErr: nfd.ErrNfdNotFound,
	}
	resolver := NewNfdDIDResolverWithFetcher(mock, 5*time.Minute)

	result, err := resolver.Dereference(context.Background(), "did:nfd:missing.algo#owner", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorNotFound, result.DereferencingMetadata.Error)
}

func TestDereference_NoFragmentOrService(t *testing.T) {
	resolver := newTestResolver()

	// Bare DID passed to Dereference should error
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorInvalidDIDURL, result.DereferencingMetadata.Error)
}

func TestDereference_FragmentAndQueryRejectsAmbiguous(t *testing.T) {
	resolver := newTestResolver()

	// DID URL with both fragment and query should be rejected
	result, err := resolver.Dereference(context.Background(), "did:nfd:test.algo#owner?service=web", ContentTypeDIDJSON)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ErrorInvalidDIDURL, result.DereferencingMetadata.Error)
	assert.Contains(t, err.Error(), "must not contain both")
}

func TestDereference_CacheReuse(t *testing.T) {
	callCount := 0
	ownerAddr := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY5HFKQ"
	futureExpiry := strconv.FormatUint(uint64(time.Now().Add(365*24*time.Hour).Unix()), 10)

	mock := &mockNfdFetcher{
		didResult: nfd.Properties{
			Internal: map[string]string{
				"name":           "cached.algo",
				"owner":          ownerAddr,
				"expirationTime": futureExpiry,
			},
			UserDefined: map[string]string{},
			Verified:    map[string]string{"domain": "https://example.com"},
		},
		didAppID: 12345,
	}

	// Wrap mock to count calls
	counting := &countingFetcher{inner: mock, count: &callCount}
	resolver := NewNfdDIDResolverWithFetcher(counting, 5*time.Minute)

	// First dereference triggers a resolve
	_, err := resolver.Dereference(context.Background(), "did:nfd:cached.algo#owner", ContentTypeDIDJSON)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second dereference of same DID (different fragment) should use cache
	_, err = resolver.Dereference(context.Background(), "did:nfd:cached.algo#web", ContentTypeDIDJSON)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "should reuse cached resolution")
}

// countingFetcher wraps a fetcher and counts FetchNfdDidVals calls.
type countingFetcher struct {
	inner nfd.NfdFetcher
	count *int
}

func (c *countingFetcher) FetchNfdDnsVals(ctx context.Context, names []string) (map[string]nfd.Properties, error) {
	return c.inner.FetchNfdDnsVals(ctx, names)
}

func (c *countingFetcher) FetchNfdDidVals(ctx context.Context, name string) (nfd.Properties, uint64, error) {
	*c.count++
	return c.inner.FetchNfdDidVals(ctx, name)
}

func (c *countingFetcher) FindNFDsByAddress(ctx context.Context, address string) ([]string, error) {
	return c.inner.FindNFDsByAddress(ctx, address)
}
