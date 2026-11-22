package mcp

import "strconv"

// Principal is who is calling, as auth middleware established it.
type Principal struct {
	Subject, Issuer string
	Scopes          []string
	Claims          map[string]any
}

// Owner is the key Handles stores with each value. It is empty for a principal with no issuer and
// no subject, which is what a request without auth gets. The issuer's length comes first, so no
// two principals share a key even when the issuer has spaces in it.
func (p Principal) Owner() string {
	if p.Issuer == "" && p.Subject == "" {
		return ""
	}
	return strconv.Itoa(len(p.Issuer)) + ":" + p.Issuer + " " + p.Subject
}

// SetPrincipal records who is calling. Auth middleware calls it before the method handler runs.
func (r *Request) SetPrincipal(p Principal) {
	r.principal = &p
}

// Principal returns the principal auth middleware set, and false when none did. Scopes and Claims
// are shared with the request, so treat them as read-only.
func (r *Request) Principal() (Principal, bool) {
	if r == nil || r.principal == nil {
		return Principal{}, false
	}
	return *r.principal, true
}
