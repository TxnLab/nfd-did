/*
 * Copyright (c) 2025-2026. TxnLab Inc.
 * All Rights reserved.
 */

package did

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ParseDIDURL parses a DID URL string into its components: base DID, fragment, and query parameters.
func ParseDIDURL(didURL string) (*DIDURL, error) {
	result := &DIDURL{Params: make(map[string]string)}

	rest := didURL

	// Split off query string
	if idx := strings.IndexByte(rest, '?'); idx != -1 {
		queryStr := rest[idx+1:]
		rest = rest[:idx]
		params, err := url.ParseQuery(queryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid query string: %w", err)
		}
		for k, v := range params {
			if len(v) > 0 {
				result.Params[k] = v[0]
			}
		}
	}

	// Split off fragment
	if idx := strings.IndexByte(rest, '#'); idx != -1 {
		result.Fragment = rest[idx+1:]
		rest = rest[:idx]
	}

	// Validate the base DID
	if _, err := parseDID(rest); err != nil {
		return nil, err
	}
	result.DID = rest

	return result, nil
}

// Dereference resolves a DID URL to a specific resource within the DID Document.
// It supports fragment dereferencing (#owner, #web, etc.) and the ?service= query parameter.
func (r *nfdDIDResolver) Dereference(ctx context.Context, didURL string, contentType string) (*DereferencingResult, error) {
	parsed, err := ParseDIDURL(didURL)
	if err != nil {
		return DereferencingErrorResult(ErrorInvalidDIDURL, contentType), err
	}

	// Resolve the base DID (uses cache)
	resolution, err := r.Resolve(ctx, parsed.DID)
	if err != nil {
		errCode := ErrorInternalError
		if resolution != nil && resolution.ResolutionMetadata.Error != "" {
			errCode = resolution.ResolutionMetadata.Error
		}
		return DereferencingErrorResult(errCode, contentType), err
	}

	if resolution.DocumentMetadata.Deactivated {
		return DereferencingErrorResult(ErrorNotFound, contentType),
			fmt.Errorf("DID is deactivated")
	}

	doc := resolution.DIDDocument

	// Reject ambiguous DID URLs with both fragment and query parameters
	if parsed.Fragment != "" && len(parsed.Params) > 0 {
		return DereferencingErrorResult(ErrorInvalidDIDURL, contentType),
			fmt.Errorf("DID URL must not contain both a fragment and query parameters")
	}

	// Handle ?service= query parameter
	if svcName, ok := parsed.Params["service"]; ok {
		return dereferenceService(doc, parsed.DID, svcName, parsed.Params["relativeRef"], contentType)
	}

	// Handle #fragment
	if parsed.Fragment != "" {
		return dereferenceFragment(doc, parsed.DID, parsed.Fragment, contentType)
	}

	return DereferencingErrorResult(ErrorInvalidDIDURL, contentType),
		fmt.Errorf("DID URL has no fragment or service parameter")
}

// dereferenceFragment searches the DID Document for a resource matching the given fragment.
func dereferenceFragment(doc *DIDDocument, baseDID, fragment, contentType string) (*DereferencingResult, error) {
	targetID := baseDID + "#" + fragment

	// Search verification methods
	for _, vm := range doc.VerificationMethod {
		if vm.ID == targetID {
			return &DereferencingResult{
				DereferencingMetadata: DereferencingMetadata{ContentType: contentType},
				ContentStream:         vm,
			}, nil
		}
	}

	// Search key agreements
	for _, ka := range doc.KeyAgreement {
		if ka.ID == targetID {
			return &DereferencingResult{
				DereferencingMetadata: DereferencingMetadata{ContentType: contentType},
				ContentStream:         ka,
			}, nil
		}
	}

	// Search services
	for _, svc := range doc.Service {
		if svc.ID == targetID {
			return &DereferencingResult{
				DereferencingMetadata: DereferencingMetadata{ContentType: contentType},
				ContentStream:         svc,
			}, nil
		}
	}

	return DereferencingErrorResult(ErrorNotFound, contentType),
		fmt.Errorf("fragment %q not found in DID document", fragment)
}

// dereferenceService finds a service by name and returns its endpoint URL,
// optionally resolving a relative reference against it.
func dereferenceService(doc *DIDDocument, baseDID, serviceName, relativeRef, contentType string) (*DereferencingResult, error) {
	targetID := baseDID + "#" + serviceName

	for _, svc := range doc.Service {
		if svc.ID != targetID {
			continue
		}

		// Extract the service endpoint URL
		endpointURL, ok := svc.ServiceEndpoint.(string)
		if !ok {
			// Non-string endpoint (e.g., NFDProfile structured object) — return as-is
			return &DereferencingResult{
				DereferencingMetadata: DereferencingMetadata{ContentType: contentType},
				ContentStream:         svc.ServiceEndpoint,
			}, nil
		}

		// Apply relativeRef if present
		if relativeRef != "" {
			base, err := url.Parse(endpointURL)
			if err != nil {
				return DereferencingErrorResult(ErrorInternalError, contentType),
					fmt.Errorf("invalid service endpoint URL: %w", err)
			}
			ref, err := url.Parse(relativeRef)
			if err != nil {
				return DereferencingErrorResult(ErrorInvalidDIDURL, contentType),
					fmt.Errorf("invalid relativeRef: %w", err)
			}
			endpointURL = base.ResolveReference(ref).String()
		}

		return &DereferencingResult{
			DereferencingMetadata: DereferencingMetadata{ContentType: contentType},
			ContentStream:         endpointURL,
		}, nil
	}

	return DereferencingErrorResult(ErrorNotFound, contentType),
		fmt.Errorf("service %q not found in DID document", serviceName)
}
