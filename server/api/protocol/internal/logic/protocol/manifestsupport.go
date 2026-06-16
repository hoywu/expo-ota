package protocol

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// Accepted media types for the manifest endpoint (§5.1).
const (
	mediaMultipart = "multipart/mixed"
	mediaExpoJSON  = "application/expo+json"
	mediaJSON      = "application/json"
)

// acceptMode is the negotiated response representation.
type acceptMode int

const (
	acceptNone      acceptMode = iota // not acceptable -> 406
	acceptMultipart                   // multipart/mixed
	acceptJSON                        // application/expo+json or application/json
)

// negotiate resolves the response representation from the Accept header.
// multipart/mixed wins when present (§5.1). An empty Accept defaults to
// multipart, matching how the expo-updates client behaves.
func negotiate(accept string) acceptMode {
	a := strings.ToLower(accept)
	if a == "" {
		return acceptMultipart
	}
	if strings.Contains(a, mediaMultipart) {
		return acceptMultipart
	}
	if strings.Contains(a, mediaExpoJSON) || strings.Contains(a, mediaJSON) || strings.Contains(a, "*/*") {
		return acceptJSON
	}
	return acceptNone
}

// jsonContentType selects the single-body content type via proactive
// negotiation (§5.1 common response headers): application/expo+json is the
// preferred representation, but a client that accepts only application/json
// MUST receive application/json rather than a type outside its Accept set.
func jsonContentType(accept string) string {
	a := strings.ToLower(accept)
	if strings.Contains(a, mediaExpoJSON) || strings.Contains(a, "*/*") || a == "" {
		return mediaExpoJSON
	}
	return mediaJSON
}

// ManifestResponse is the fully-rendered manifest endpoint response. The
// handler writes Header, then StatusCode, then Body verbatim.
type ManifestResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// commonManifestHeader returns the headers carried by every manifest
// response (§5.1).
func commonManifestHeader() http.Header {
	h := make(http.Header)
	h.Set("expo-protocol-version", "1")
	h.Set("expo-sfv-version", "0")
	h.Set("expo-manifest-filters", `branch="default"`)
	h.Set("expo-server-defined-headers", "{}")
	h.Set("cache-control", "private, max-age=0")
	return h
}

// buildMultipart assembles a multipart/mixed body with an optional manifest
// part and an optional directive part, returning the body and the boundary.
// signatureValue, when non-empty, is attached to the manifest part as the
// expo-signature header.
func buildMultipart(manifestBody []byte, signatureValue string, directiveBody []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if manifestBody != nil {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="manifest"`)
		header.Set("Content-Type", "application/json; charset=utf-8")
		if signatureValue != "" {
			header.Set("expo-signature", signatureValue)
		}
		part, err := w.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(manifestBody); err != nil {
			return nil, "", err
		}
	}

	if directiveBody != nil {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="directive"`)
		header.Set("Content-Type", "application/json; charset=utf-8")
		part, err := w.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(directiveBody); err != nil {
			return nil, "", err
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.Boundary(), nil
}
