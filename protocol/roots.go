package protocol

// Roots are deprecated in 2026-07-28 and still supported.

type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
	Meta Object `json:"_meta,omitempty"`
}

type ListRootsParams struct {
	Meta Object `json:"_meta,omitempty"`
}

type ListRootsResult struct {
	Roots []Root `json:"roots"`
}
