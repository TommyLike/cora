package assets

import _ "embed"

// EtherpadSpec is the OpenAPI spec for the Etherpad service,
// embedded at build time from assets/openapi/etherpad/openapi.json.
//
//go:embed openapi/etherpad/openapi.json
var EtherpadSpec []byte

// GitcodeSpec is the OpenAPI spec for the GitCode platform,
// embedded at build time from assets/openapi/gitcode/openapi.json.
//
//go:embed openapi/gitcode/openapi.json
var GitcodeSpec []byte
