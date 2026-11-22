package mcp

import "testing"

func TestPrincipalOwner(t *testing.T) {
	tests := []struct {
		name string
		p    Principal
		want string
	}{
		{"zero", Principal{}, ""},
		{"scopes only", Principal{Scopes: []string{"read"}}, ""},
		{"issuer and subject", Principal{Issuer: "https://idp.example", Subject: "alice"}, "19:https://idp.example alice"},
		{"subject only", Principal{Subject: "alice"}, "0: alice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Owner(); got != tt.want {
				t.Fatalf("Owner() = %q, want %q", got, tt.want)
			}
		})
	}
	a := Principal{Issuer: "a b", Subject: "c"}.Owner()
	b := Principal{Issuer: "a", Subject: "b c"}.Owner()
	if a == b {
		t.Fatalf("different principals share the owner key %q", a)
	}
}

func TestRequestPrincipal(t *testing.T) {
	req := newTestRequest("x.example/echo")
	if _, ok := req.Principal(); ok {
		t.Fatal("a new request has a principal")
	}
	p := Principal{Issuer: "https://idp.example", Subject: "alice", Scopes: []string{"read"}}
	req.SetPrincipal(p)
	got, ok := req.Principal()
	if !ok || got.Owner() != p.Owner() || len(got.Scopes) != 1 {
		t.Fatalf("Principal() = %+v, %v", got, ok)
	}
	var nilReq *Request
	if _, ok := nilReq.Principal(); ok {
		t.Fatal("a nil request has a principal")
	}
}
