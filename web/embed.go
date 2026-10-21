package web

import (
	_ "embed"
)

//go:embed index.html
var RemoteIndexHTML []byte

//go:embed pairing.html
var PairingDashboardHTML []byte
