/*
 * Copyright (c) 2025. TxnLab Inc.
 * All Rights reserved.
 */

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"

	"github.com/TxnLab/nfd-did/internal/did"
)

// Config holds the server configuration.
type Config struct {
	Listen     string
	AlgodURL   string
	AlgodToken string
	RegistryID uint64
	CacheTTL   time.Duration
}

func main() {
	cfg := loadConfig()

	// Create algod client
	client, err := algod.MakeClient(cfg.AlgodURL, cfg.AlgodToken)
	if err != nil {
		log.Fatalf("Failed to create algod client: %v", err)
	}

	// Create resolver
	resolver := did.NewNfdDIDResolver(client, cfg.RegistryID, cfg.CacheTTL)

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /1.0/identifiers/{did...}", handleResolve(resolver))
	mux.HandleFunc("GET /1.0/properties", handleProperties)
	mux.HandleFunc("GET /health", handleHealth)

	log.Printf("Starting DID (%s) resolver on %s (algod: %s, registry: %d, cache: %s)",
		getVersionInfo(), cfg.Listen, cfg.AlgodURL, cfg.RegistryID, cfg.CacheTTL)

	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadConfig() Config {
	cfg := Config{
		Listen:     envOrDefault("LISTEN", ":8080"),
		AlgodURL:   envOrDefault("ALGOD_URL", "https://mainnet-api.4160.nodely.dev"),
		AlgodToken: envOrDefault("ALGOD_TOKEN", ""),
		RegistryID: 760937186,
		CacheTTL:   5 * time.Minute,
	}

	if v := os.Getenv("REGISTRY_ID"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			cfg.RegistryID = id
		}
	}

	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CacheTTL = d
		}
	}

	return cfg
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// setContentTypeUTF8 writes Content-Type with an explicit UTF-8 charset.
// All response bodies are JSON-encoded UTF-8; the charset parameter prevents
// consumers from guessing (e.g. Latin-1) and mojibake on non-ASCII property values.
func setContentTypeUTF8(w http.ResponseWriter, base string) {
	w.Header().Set("Content-Type", base+"; charset=utf-8")
}

func handleResolve(resolver did.NfdDIDResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		didStr := r.PathValue("did")
		if didStr == "" {
			writeError(w, http.StatusBadRequest, "missing DID parameter")
			return
		}

		// URL-decode the path value to handle %23 -> # (Universal Resolver encodes fragments in path)
		decodedDID, err := url.PathUnescape(didStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid URL encoding")
			return
		}

		// Determine content type from Accept header
		contentType := did.ContentTypeDIDJSON
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, did.ContentTypeDIDLDJSON) {
			contentType = did.ContentTypeDIDLDJSON
		}

		// Check if this is a dereferencing request (fragment in path or ?service= query)
		hasFragment := strings.Contains(decodedDID, "#")
		serviceParam := r.URL.Query().Get("service")

		if hasFragment || serviceParam != "" {
			// Build the full DID URL string for the dereferencer
			didURL := decodedDID
			if serviceParam != "" {
				// Strip any fragment from the path — ?service= takes precedence
				if idx := strings.IndexByte(didURL, '#'); idx != -1 {
					didURL = didURL[:idx]
				}
				didURL += "?service=" + url.QueryEscape(serviceParam)
				if relRef := r.URL.Query().Get("relativeRef"); relRef != "" {
					didURL += "&relativeRef=" + url.QueryEscape(relRef)
				}
			}
			handleDereference(w, resolver, r, didURL, contentType, accept)
			return
		}

		// Standard DID resolution
		result, err := resolver.Resolve(r.Context(), decodedDID)
		if err != nil {
			if result != nil && result.ResolutionMetadata.Error != "" {
				switch result.ResolutionMetadata.Error {
				case did.ErrorNotFound:
					setContentTypeUTF8(w, contentType)
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(result)
					return
				case did.ErrorInvalidDID:
					setContentTypeUTF8(w, contentType)
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(result)
					return
				}
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Check for deactivated
		statusCode := http.StatusOK
		if result.DocumentMetadata.Deactivated {
			statusCode = http.StatusGone
		}

		setContentTypeUTF8(w, contentType)
		w.WriteHeader(statusCode)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	}
}

func handleDereference(w http.ResponseWriter, resolver did.NfdDIDResolver, r *http.Request, didURL, contentType, accept string) {
	result, err := resolver.Dereference(r.Context(), didURL, contentType)
	if err != nil {
		if result != nil && result.DereferencingMetadata.Error != "" {
			switch result.DereferencingMetadata.Error {
			case did.ErrorNotFound:
				setContentTypeUTF8(w, contentType)
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(result)
				return
			case did.ErrorInvalidDID, did.ErrorInvalidDIDURL:
				setContentTypeUTF8(w, contentType)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(result)
				return
			}
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// For ?service= with Accept: text/uri-list, return HTTP 303 redirect
	if r.URL.Query().Get("service") != "" {
		if endpointURL, ok := result.ContentStream.(string); ok {
			if strings.Contains(accept, did.ContentTypeURIList) {
				// Only redirect to http/https URLs to prevent open redirect abuse
				parsed, err := url.Parse(endpointURL)
				if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
					w.Header().Set("Location", endpointURL)
					w.WriteHeader(http.StatusSeeOther)
					return
				}
			}
		}
	}

	setContentTypeUTF8(w, result.DereferencingMetadata.ContentType)
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	setContentTypeUTF8(w, "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func handleProperties(w http.ResponseWriter, _ *http.Request) {
	setContentTypeUTF8(w, "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]interface{}{
		"method":  "nfd",
		"network": "algorand",
		"properties": map[string]interface{}{
			"identifierFormat":          "did:nfd:<name>.algo",
			"identifierRegex":           "^did:nfd:([a-z0-9]{1,27}\\.){1,2}algo$",
			"blockchain":                "algorand",
			"keyType":                   "Ed25519",
			"supportsDeactivation":      true,
			"supportsExpiration":        true,
			"supportsKeyAgreement":      true,
			"supportsServices":          true,
			"supportsReverseResolution": true,
		},
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	setContentTypeUTF8(w, "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// Version is replaced at build time during docker builds w/ 'release' version
// If not defined, we just return the git rev.
var Version string

func getVersionInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "The version information could not be determined"
	}
	var vcsRev = "(unknown)"
	if fnd := slices.IndexFunc(info.Settings, func(v debug.BuildSetting) bool { return v.Key == "vcs.revision" }); fnd != -1 {
		rev := info.Settings[fnd].Value
		if len(rev) > 7 {
			rev = rev[:7]
		}
		vcsRev = rev
	}
	if Version != "" {
		return fmt.Sprintf("%s [%s]", Version, vcsRev)
	}
	return vcsRev
}
