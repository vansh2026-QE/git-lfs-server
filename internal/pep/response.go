package pep

// BatchResponse is the LFS batch response body. Mirrors the fields the fork's
// client reads. See client-implementation/git-lfs/tq/{api,transfer}.go.
type BatchResponse struct {
	Objects       []*Transfer `json:"objects"`
	Transfer      string      `json:"transfer,omitempty"`
	HashAlgorithm string      `json:"hash_algo,omitempty"`
}

// Transfer is one object's result: either Actions (permitted) or Error
// (denied / not found), never both. Authenticated tells the client the action
// hrefs are self-authorizing capability URLs, so it sends no extra credentials
// to them (the demo's open-transfer-endpoint decision). See plan.
type Transfer struct {
	OID           string             `json:"oid"`
	Size          int64              `json:"size"`
	Authenticated bool               `json:"authenticated,omitempty"`
	Actions       map[string]*Action `json:"actions,omitempty"`
	Error         *ObjectError       `json:"error,omitempty"`
}

// Action is a transfer action under the "download" or "upload" key.
type Action struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresIn int               `json:"expires_in,omitempty"`
}

// ObjectError is a per-object error inside a 200 batch (download denials,
// not-found). Code is an HTTP-style status. See docs/auth-design.md §8.4.
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
