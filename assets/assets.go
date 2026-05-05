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

// GithubSpec is the OpenAPI spec for the GitHub v3 REST API,
// embedded at build time from assets/openapi/github/api.github.com.json.
//
//go:embed openapi/github/api.github.com.json
var GithubSpec []byte

// JenkinsSpec is the OpenAPI spec for the Jenkins automation server,
// embedded at build time from assets/openapi/jenkins/openapi.json.
//
//go:embed openapi/jenkins/openapi.json
var JenkinsSpec []byte
