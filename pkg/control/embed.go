package control

import _ "embed"

//go:embed web/index.html
var indexHTML []byte

//go:embed web/generic.html
var genericHTML []byte
