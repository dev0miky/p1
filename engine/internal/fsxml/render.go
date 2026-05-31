package fsxml

import (
	"bytes"
	"strings"
	"text/template" // not html/template — html/template would escape the $$ in FS preprocessor vars
)

type GatewayView struct {
	Name           string
	Proxy          string
	Username       string
	Password       string
	Realm          string
	FromUser       string
	FromDomain     string
	Transport      string
	Register       bool
	CallerIDInFrom bool
	ExpireSeconds  int
	RetrySeconds   int
	Extra          map[string]string
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

var sofiaTemplate = template.Must(template.New("sofia").Funcs(template.FuncMap{
	"x": xmlEscape,
	"boolStr": func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	},
}).Parse(`<document type="freeswitch/xml">
  <section name="configuration">
    <configuration name="sofia.conf" description="sofia Endpoint">
      <global_settings>
        <param name="log-level" value="0"/>
        <param name="auto-restart" value="false"/>
        <param name="debug-presence" value="0"/>
        <param name="capture-server" value="udp:127.0.0.1:9060"/>
      </global_settings>
      <profiles>
        <profile name="internal">
          <aliases/>
          <gateways/>
          <domains>
            <domain name="all" alias="true" parse="false"/>
          </domains>
          <settings>
            <param name="user-agent-string" value="p1"/>
            <param name="debug" value="0"/>
            <param name="sip-trace" value="no"/>
            <param name="sip-port" value="$${internal_sip_port}"/>
            <param name="rtp-ip" value="$${local_ip_v4}"/>
            <param name="sip-ip" value="$${local_ip_v4}"/>
            <param name="ext-rtp-ip" value="$${external_rtp_ip}"/>
            <param name="ext-sip-ip" value="$${external_sip_ip}"/>
            <param name="auth-calls" value="true"/>
            <param name="apply-inbound-acl" value="domains"/>
            <param name="dialplan" value="XML"/>
            <param name="context" value="default"/>
            <param name="dtmf-duration" value="2000"/>
            <param name="dtmf-type" value="rfc2833"/>
            <param name="inbound-codec-prefs" value="$${global_codec_prefs}"/>
            <param name="outbound-codec-prefs" value="$${global_codec_prefs}"/>
            <param name="rtp-timer-name" value="soft"/>
            <param name="enable-100rel" value="true"/>
            <param name="enable-3pcc" value="proxy"/>
            <param name="ws-binding" value=":7080"/>
            <param name="wss-binding" value=":7443"/>
          </settings>
        </profile>
        <profile name="external">
          <gateways>
{{- range .}}
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
{{- if and $isTLS .Register}}
              <param name="register-transport" value="tls"/>
{{- end}}
              <param name="expire-seconds" value="{{.ExpireSeconds}}"/>
              <param name="retry-seconds" value="{{.RetrySeconds}}"/>
              <param name="caller-id-in-from" value="{{boolStr .CallerIDInFrom}}"/>
{{- range $k, $v := .Extra}}
              <param name="{{x $k}}" value="{{x $v}}"/>
{{- end}}
            </gateway>
{{- end}}
          </gateways>
          <aliases/>
          <domains>
            <domain name="all" alias="false" parse="false"/>
          </domains>
          <settings>
            <param name="user-agent-string" value="p1"/>
            <param name="debug" value="0"/>
            <param name="sip-trace" value="yes"/>
            <param name="sip-port" value="$${external_sip_port}"/>
            <param name="rtp-ip" value="$${local_ip_v4}"/>
            <param name="sip-ip" value="$${local_ip_v4}"/>
            <param name="ext-rtp-ip" value="$${external_rtp_ip}"/>
            <param name="ext-sip-ip" value="$${external_sip_ip}"/>
            <param name="context" value="public"/>
            <param name="dialplan" value="XML"/>
            <param name="dtmf-duration" value="2000"/>
            <param name="dtmf-type" value="rfc2833"/>
            <param name="liberal-dtmf" value="true"/>
            <param name="inbound-codec-prefs" value="$${outbound_codec_prefs}"/>
            <param name="outbound-codec-prefs" value="$${outbound_codec_prefs}"/>
            <param name="auth-calls" value="false"/>
            <param name="apply-inbound-acl" value="domains"/>
            <param name="hold-music" value="$${hold_music}"/>
            <param name="tls" value="true"/>
            <param name="tls-only" value="false"/>
            <param name="tls-sip-port" value="5061"/>
            <param name="tls-bind-params" value="transport=tls"/>
            <param name="tls-version" value="tlsv1.2,tlsv1.3"/>
          </settings>
        </profile>
      </profiles>
    </configuration>
  </section>
</document>`))

func RenderSofia(gws []GatewayView) (string, error) {
	var buf bytes.Buffer
	if err := sofiaTemplate.Execute(&buf, gws); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func NotFound() string {
	return `<document type="freeswitch/xml"><section name="result"><result status="not found"/></section></document>`
}
