package fsxml

import (
	"bytes"
	"strings"
	"text/template" // not html/template — html/template would escape the $$ in FS preprocessor vars
)

type GatewayView struct {
	Name            string
	Proxy           string
	Username        string
	Password        string
	Realm           string
	FromUser        string
	FromDomain      string
	Transport       string
	MediaEncryption string
	Register        bool
	CallerIDInFrom  bool
	ExpireSeconds   int
	RetrySeconds    int
	Extra           map[string]string
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

var gatewayTemplate = template.Must(template.New("gateway").Funcs(template.FuncMap{
	"x": xmlEscape,
	"boolStr": func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	},
}).Parse(`<include>
  <gateway name="{{x .Name}}">
{{- if .Username}}
    <param name="username" value="{{x .Username}}"/>
{{- end}}
{{- if .Password}}
    <param name="password" value="{{x .Password}}"/>
{{- end}}
{{- if .Realm}}
    <param name="realm" value="{{x .Realm}}"/>
{{- end}}
{{- if .FromUser}}
    <param name="from-user" value="{{x .FromUser}}"/>
{{- end}}
{{- if .FromDomain}}
    <param name="from-domain" value="{{x .FromDomain}}"/>
{{- end}}
{{- $isTLS := eq .Transport "tls"}}
    <param name="proxy" value="{{x .Proxy}}{{if $isTLS}};transport=tls{{end}}"/>
    <param name="register" value="{{boolStr .Register}}"/>
{{- if .Register}}
    <param name="register-transport" value="{{x .Transport}}"/>
{{- end}}
    <param name="expire-seconds" value="{{.ExpireSeconds}}"/>
    <param name="retry-seconds" value="{{.RetrySeconds}}"/>
    <param name="caller-id-in-from" value="{{boolStr .CallerIDInFrom}}"/>
{{- range $k, $v := .Extra}}
    <param name="{{x $k}}" value="{{x $v}}"/>
{{- end}}
{{- if eq .MediaEncryption "srtp"}}
    <variable name="rtp_secure_media" value="mandatory:AES_CM_128_HMAC_SHA1_80"/>
{{- end}}
  </gateway>
</include>`))

func RenderGatewayFile(g GatewayView) (string, error) {
	var buf bytes.Buffer
	if err := gatewayTemplate.Execute(&buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}
