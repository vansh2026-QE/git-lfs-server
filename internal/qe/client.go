package qe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client talks to lfsd's reveal surface: GET /{repo}/names (the authorized real
// paths) and GET /{repo}/content (dereference a pointer's bytes). It is built
// from the workspace lfs.url, whose userinfo carries the LFS credentials. Go's
// http client does not send userinfo automatically, so NewClient strips the
// userinfo from the request URL and replays it as HTTP Basic auth on every call.
// See docs/name-hiding-design.md §9 and internal/pep/transfer.go (getContent).
type Client struct {
	base string // lfs.url without userinfo, e.g. http://localhost:8080/demo
	user string
	pass string
	http *http.Client
}

// NewClient parses an lfs.url of the form scheme://user:pass@host/repo and
// returns a Client that targets {scheme}://{host}/{repo} with Basic auth.
func NewClient(lfsURL string) (*Client, error) {
	u, err := url.Parse(lfsURL)
	if err != nil {
		return nil, fmt.Errorf("qe: parse lfs.url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("qe: lfs.url %q missing scheme or host", lfsURL)
	}
	c := &Client{http: http.DefaultClient}
	if u.User != nil {
		c.user = u.User.Username()
		c.pass, _ = u.User.Password()
	}
	bare := *u
	bare.User = nil
	c.base = bare.String()
	return c, nil
}

// namesResponse mirrors the {"paths":[...]} body GET /{repo}/names returns.
type namesResponse struct {
	Paths []string `json:"paths"`
}

// Names fetches the real paths the authenticated user may download in this repo.
// The server has already filtered by policy, so every returned path is one the
// caller may reveal (docs/name-hiding-design.md §6).
func (c *Client) Names(ctx context.Context) ([]string, error) {
	req, err := c.newRequest(ctx, c.base+"/names")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qe: GET names: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qe: GET names: unexpected status %s", resp.Status)
	}
	var nr namesResponse
	if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
		return nil, fmt.Errorf("qe: decode names: %w", err)
	}
	return nr.Paths, nil
}

// Content streams the bytes of the object identified by oid into w. The claimed
// name is left empty: the server resolves it from PathIndex (firstRecordedPath)
// and the caller is already known to be authorized via Names, so this only ever
// fetches content the user may read. A non-200 (e.g. 403/404) is returned as an
// error and nothing is written. See internal/pep/transfer.go getContent.
func (c *Client) Content(ctx context.Context, oid string, w io.Writer) error {
	u := c.base + "/content?" + url.Values{"oid": {oid}}.Encode()
	req, err := c.newRequest(ctx, u)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("qe: GET content %s: %w", oid, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qe: GET content %s: unexpected status %s", oid, resp.Status)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("qe: stream content %s: %w", oid, err)
	}
	return nil
}

// newRequest builds a GET carrying Basic auth when the lfs.url supplied creds.
func (c *Client) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("qe: build request: %w", err)
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	return req, nil
}
