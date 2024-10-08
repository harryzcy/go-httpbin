package cmd

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mccutchen/go-httpbin/v2/httpbin"
	"github.com/mccutchen/go-httpbin/v2/internal/testing/assert"
)

// To update, run:
// OSX:
// make && ./dist/go-httpbin -h 2>&1 | pbcopy
// Linux (paste with middle mouse):
// make && ./dist/go-httpbin -h 2>&1 | xclip
const usage = `Usage of go-httpbin:
  -allowed-redirect-domains string
    	Comma-separated list of domains the /redirect-to endpoint will allow
  -exclude-headers string
    	Drop platform-specific headers. Comma-separated list of headers key to drop, supporting wildcard matching.
  -host string
    	Host to listen on (default "0.0.0.0")
  -https-cert-file string
    	HTTPS Server certificate file
  -https-key-file string
    	HTTPS Server private key file
  -log-format string
    	Log format (text or json) (default "text")
  -max-body-size int
    	Maximum size of request or response, in bytes (default 1048576)
  -max-duration duration
    	Maximum duration a response may take (default 10s)
  -port int
    	Port to listen on (default 8080)
  -prefix string
    	Path prefix (empty or start with slash and does not end with slash)
  -use-real-hostname
    	Expose value of os.Hostname() in the /hostname endpoint instead of dummy value
`

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	testDefaultRealHostname := "real-hostname.test"
	getHostnameDefault := func() (string, error) {
		return testDefaultRealHostname, nil
	}

	testCases := map[string]struct {
		args        []string
		env         map[string]string
		getHostname func() (string, error)
		wantCfg     *config
		wantErr     error
		wantOut     string
	}{
		"defaults": {
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"-h": {
			args:    []string{"-h"},
			wantErr: flag.ErrHelp,
		},
		"-help": {
			args:    []string{"-help"},
			wantErr: flag.ErrHelp,
		},

		// env
		"ok env with empty variables": {
			env: map[string]string{},
			wantCfg: &config{
				Env:         nil,
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok env with recognized variables": {
			env: map[string]string{
				fmt.Sprintf("%sFOO", defaultEnvPrefix):                     "foo",
				fmt.Sprintf("%s%sBAR", defaultEnvPrefix, defaultEnvPrefix): "bar",
				fmt.Sprintf("%s123", defaultEnvPrefix):                     "123",
			},
			wantCfg: &config{
				Env: map[string]string{
					fmt.Sprintf("%sFOO", defaultEnvPrefix):                     "foo",
					fmt.Sprintf("%s%sBAR", defaultEnvPrefix, defaultEnvPrefix): "bar",
					fmt.Sprintf("%s123", defaultEnvPrefix):                     "123",
				},
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok env with unrecognized variables": {
			env: map[string]string{"HTTPBIN_FOO": "foo", "BAR": "bar"},
			wantCfg: &config{
				Env:         nil,
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},

		// max body size
		"invalid -max-body-size": {
			args:    []string{"-max-body-size", "foo"},
			wantErr: errors.New("invalid value \"foo\" for flag -max-body-size: parse error"),
		},
		"invalid MAX_BODY_SIZE": {
			env:     map[string]string{"MAX_BODY_SIZE": "foo"},
			wantErr: errors.New("invalid value \"foo\" for env var MAX_BODY_SIZE: parse error"),
		},
		"ok -max-body-size": {
			args: []string{"-max-body-size", "99"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: 99,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok MAX_BODY_SIZE": {
			env: map[string]string{"MAX_BODY_SIZE": "9999"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: 9999,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok max body size CLI takes precedence over env": {
			args: []string{"-max-body-size", "1234"},
			env:  map[string]string{"MAX_BODY_SIZE": "5678"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: 1234,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},

		// max duration
		"invalid -max-duration": {
			args:    []string{"-max-duration", "foo"},
			wantErr: errors.New("invalid value \"foo\" for flag -max-duration: parse error"),
		},
		"invalid MAX_DURATION": {
			env:     map[string]string{"MAX_DURATION": "foo"},
			wantErr: errors.New("invalid value \"foo\" for env var MAX_DURATION: parse error"),
		},
		"ok -max-duration": {
			args: []string{"-max-duration", "99s"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: 99 * time.Second,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok MAX_DURATION": {
			env: map[string]string{"MAX_DURATION": "9999s"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: 9999 * time.Second,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok max duration size CLI takes precedence over env": {
			args: []string{"-max-duration", "1234s"},
			env:  map[string]string{"MAX_DURATION": "5678s"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: 1234 * time.Second,
				LogFormat:   defaultLogFormat,
			},
		},

		// host
		"ok -host": {
			args: []string{"-host", "192.0.0.1"},
			wantCfg: &config{
				ListenHost:  "192.0.0.1",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok HOST": {
			env: map[string]string{"HOST": "192.0.0.2"},
			wantCfg: &config{
				ListenHost:  "192.0.0.2",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok host cli takes precedence over end": {
			args: []string{"-host", "99.99.99.99"},
			env:  map[string]string{"HOST": "11.11.11.11"},
			wantCfg: &config{
				ListenHost:  "99.99.99.99",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},

		// port
		"invalid -port": {
			args:    []string{"-port", "foo"},
			wantErr: errors.New("invalid value \"foo\" for flag -port: parse error"),
		},
		"invalid PORT": {
			env:     map[string]string{"PORT": "foo"},
			wantErr: errors.New("invalid value \"foo\" for env var PORT: parse error"),
		},
		"ok -port": {
			args: []string{"-port", "99"},
			wantCfg: &config{
				ListenHost:  defaultListenHost,
				ListenPort:  99,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok PORT": {
			env: map[string]string{"PORT": "9999"},
			wantCfg: &config{
				ListenHost:  defaultListenHost,
				ListenPort:  9999,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok port CLI takes precedence over env": {
			args: []string{"-port", "1234"},
			env:  map[string]string{"PORT": "5678"},
			wantCfg: &config{
				ListenHost:  defaultListenHost,
				ListenPort:  1234,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},

		// prefix
		"invalid -prefix (does not start with slash)": {
			args:    []string{"-prefix", "invalidprefix1"},
			wantErr: errors.New("Prefix \"invalidprefix1\" must start with a slash"),
		},
		"invalid -prefix (ends with with slash)": {
			args:    []string{"-prefix", "/invalidprefix2/"},
			wantErr: errors.New("Prefix \"/invalidprefix2/\" must not end with a slash"),
		},
		"ok -prefix takes precedence over env": {
			args: []string{"-prefix", "/prefix1"},
			env:  map[string]string{"PREFIX": "/prefix2"},
			wantCfg: &config{
				ListenHost:  defaultListenHost,
				ListenPort:  defaultListenPort,
				Prefix:      "/prefix1",
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok PREFIX": {
			env: map[string]string{"PREFIX": "/prefix2"},
			wantCfg: &config{
				ListenHost:  defaultListenHost,
				ListenPort:  defaultListenPort,
				Prefix:      "/prefix2",
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},

		// https cert file
		"https cert and key must both be provided, cert only": {
			args:    []string{"-https-cert-file", "/tmp/test.crt"},
			wantErr: errors.New("https cert and key must both be provided"),
		},
		"https cert and key must both be provided, key only": {
			args:    []string{"-https-key-file", "/tmp/test.crt"},
			wantErr: errors.New("https cert and key must both be provided"),
		},
		"ok https CLI": {
			args: []string{
				"-https-cert-file", "/tmp/test.crt",
				"-https-key-file", "/tmp/test.key",
			},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				TLSCertFile: "/tmp/test.crt",
				TLSKeyFile:  "/tmp/test.key",
				LogFormat:   defaultLogFormat,
			},
		},
		"ok https env": {
			env: map[string]string{
				"HTTPS_CERT_FILE": "/tmp/test.crt",
				"HTTPS_KEY_FILE":  "/tmp/test.key",
			},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				TLSCertFile: "/tmp/test.crt",
				TLSKeyFile:  "/tmp/test.key",
				LogFormat:   defaultLogFormat,
			},
		},
		"ok https CLI takes precedence over env": {
			args: []string{
				"-https-cert-file", "/tmp/cli.crt",
				"-https-key-file", "/tmp/cli.key",
			},
			env: map[string]string{
				"HTTPS_CERT_FILE": "/tmp/env.crt",
				"HTTPS_KEY_FILE":  "/tmp/env.key",
			},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				TLSCertFile: "/tmp/cli.crt",
				TLSKeyFile:  "/tmp/cli.key",
				LogFormat:   defaultLogFormat,
			},
		},

		// use-real-hostname
		"ok -use-real-hostname": {
			args: []string{"-use-real-hostname"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		"ok -use-real-hostname=1": {
			args: []string{"-use-real-hostname", "1"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		"ok -use-real-hostname=true": {
			args: []string{"-use-real-hostname", "true"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		// any value for the argument is interpreted as true
		"ok -use-real-hostname=0": {
			args: []string{"-use-real-hostname", "0"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		"ok USE_REAL_HOSTNAME=1": {
			env: map[string]string{"USE_REAL_HOSTNAME": "1"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		"ok USE_REAL_HOSTNAME=true": {
			env: map[string]string{"USE_REAL_HOSTNAME": "true"},
			wantCfg: &config{
				ListenHost:   "0.0.0.0",
				ListenPort:   8080,
				MaxBodySize:  httpbin.DefaultMaxBodySize,
				MaxDuration:  httpbin.DefaultMaxDuration,
				RealHostname: testDefaultRealHostname,
				LogFormat:    defaultLogFormat,
			},
		},
		// case sensitive
		"ok USE_REAL_HOSTNAME=TRUE": {
			env: map[string]string{"USE_REAL_HOSTNAME": "TRUE"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok USE_REAL_HOSTNAME=false": {
			env: map[string]string{"USE_REAL_HOSTNAME": "false"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"err real hostname error": {
			env:         map[string]string{"USE_REAL_HOSTNAME": "true"},
			getHostname: func() (string, error) { return "", errors.New("hostname error") },
			wantErr:     errors.New("could not look up real hostname: hostname error"),
		},

		// allowed-redirect-domains
		"ok -allowed-redirect-domains": {
			args: []string{"-allowed-redirect-domains", "foo,bar"},
			wantCfg: &config{
				ListenHost:             "0.0.0.0",
				ListenPort:             8080,
				MaxBodySize:            httpbin.DefaultMaxBodySize,
				MaxDuration:            httpbin.DefaultMaxDuration,
				AllowedRedirectDomains: []string{"foo", "bar"},
				LogFormat:              defaultLogFormat,
			},
		},
		"ok ALLOWED_REDIRECT_DOMAINS": {
			env: map[string]string{"ALLOWED_REDIRECT_DOMAINS": "foo,bar"},
			wantCfg: &config{
				ListenHost:             "0.0.0.0",
				ListenPort:             8080,
				MaxBodySize:            httpbin.DefaultMaxBodySize,
				MaxDuration:            httpbin.DefaultMaxDuration,
				AllowedRedirectDomains: []string{"foo", "bar"},
				LogFormat:              defaultLogFormat,
			},
		},
		"ok allowed redirect domains CLI takes precedence over env": {
			args: []string{"-allowed-redirect-domains", "foo.cli,bar.cli"},
			env:  map[string]string{"ALLOWED_REDIRECT_DOMAINS": "foo.env,bar.env"},
			wantCfg: &config{
				ListenHost:             "0.0.0.0",
				ListenPort:             8080,
				MaxBodySize:            httpbin.DefaultMaxBodySize,
				MaxDuration:            httpbin.DefaultMaxDuration,
				AllowedRedirectDomains: []string{"foo.cli", "bar.cli"},
				LogFormat:              defaultLogFormat,
			},
		},
		"ok allowed redirect domains are normalized": {
			args: []string{"-allowed-redirect-domains", "foo, bar  ,, baz   "},
			wantCfg: &config{
				ListenHost:             "0.0.0.0",
				ListenPort:             8080,
				MaxBodySize:            httpbin.DefaultMaxBodySize,
				MaxDuration:            httpbin.DefaultMaxDuration,
				AllowedRedirectDomains: []string{"foo", "bar", "baz"},
				LogFormat:              defaultLogFormat,
			},
		},
		"ok use json log format": {
			args: []string{"-log-format", "json"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   "json",
			},
		},
		"ok use text log format": {
			args: []string{"-log-format", "text"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   "text",
			},
		},
		"ok use default log format": {
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   defaultLogFormat,
			},
		},
		"ok use json log format using LOG_FORMAT env": {
			env: map[string]string{"LOG_FORMAT": "json"},
			wantCfg: &config{
				ListenHost:  "0.0.0.0",
				ListenPort:  8080,
				MaxBodySize: httpbin.DefaultMaxBodySize,
				MaxDuration: httpbin.DefaultMaxDuration,
				LogFormat:   "json",
			},
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.getHostname == nil {
				tc.getHostname = getHostnameDefault
			}
			cfg, err := loadConfig(tc.args, func(key string) string { return tc.env[key] }, func() []string { return environSlice(tc.env) }, tc.getHostname)

			switch {
			case tc.wantErr != nil && err != nil:
				if tc.wantErr.Error() != err.Error() {
					t.Fatalf("incorrect error\nwant: %q\ngot:  %q", tc.wantErr, err)
				}
			case tc.wantErr != nil:
				t.Fatalf("want error %q, got nil", tc.wantErr)
			case err != nil:
				t.Fatalf("got unexpected error: %q", err)
			}

			if !reflect.DeepEqual(tc.wantCfg, cfg) {
				t.Fatalf("bad config\nwant: %#v\ngot:  %#v", tc.wantCfg, cfg)
			}
		})
	}
}

func TestMainImpl(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		args        []string
		env         map[string]string
		getHostname func() (string, error)
		wantCode    int
		wantOut     string
		wantOutFn   func(t *testing.T, out string)
	}{
		"help": {
			args:     []string{"-h"},
			wantCode: 0,
			wantOut:  usage,
		},
		"cli error": {
			args:     []string{"-max-body-size", "foo"},
			wantCode: 2,
			wantOut:  "error: invalid value \"foo\" for flag -max-body-size: parse error\n\n" + usage,
		},
		"unknown argument": {
			args:     []string{"-zzz"},
			wantCode: 2,
			wantOut:  "error: flag provided but not defined: -zzz\n\n" + usage,
		},
		"real hostname error": {
			args:        []string{"-use-real-hostname"},
			getHostname: func() (string, error) { return "", errors.New("hostname failure") },
			wantCode:    1,
			wantOut:     "error: could not look up real hostname: hostname failure",
		},
		"server error": {
			args: []string{
				"-port", "-256",
				"-host", "127.0.0.1", // default of 0.0.0.0 causes annoying permission popup on macOS
			},
			wantCode: 1,
			wantOutFn: func(t *testing.T, out string) {
				assert.Contains(t, out, `msg="error: listen tcp: address -256: invalid port"`, "server error does not contain expected message")
			},
		},
		"tls cert error": {
			args: []string{
				"-host", "127.0.0.1", // default of 0.0.0.0 causes annoying permission popup on macOS
				"-port", "0",
				"-https-cert-file", "./https-cert-does-not-exist",
				"-https-key-file", "./https-key-does-not-exist",
			},
			wantCode: 1,
			wantOutFn: func(t *testing.T, out string) {
				assert.Contains(t, out, `msg="error: open ./https-cert-does-not-exist: no such file or directory"`, "tls cert error does not contain expected message")
			},
		},
		"log format error": {
			args:     []string{"-log-format", "invalid"},
			wantCode: 2,
			wantOut:  "error: invalid log format \"invalid\", must be \"text\" or \"json\"\n\n" + usage,
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.getHostname == nil {
				tc.getHostname = os.Hostname
			}

			buf := &bytes.Buffer{}
			gotCode := mainImpl(tc.args, func(key string) string { return tc.env[key] }, func() []string { return environSlice(tc.env) }, tc.getHostname, buf)
			out := buf.String()

			if gotCode != tc.wantCode {
				t.Logf("unexpected error: output:\n%s", out)
				t.Fatalf("expected return code %d, got %d", tc.wantCode, gotCode)
			}

			if tc.wantOutFn != nil {
				tc.wantOutFn(t, out)
				return
			}

			if out != tc.wantOut {
				t.Fatalf("output mismatch error:\nwant: %q\ngot:  %q", tc.wantOut, out)
			}
		})
	}
}

func environSlice(env map[string]string) []string {
	envStrings := make([]string, 0, len(env))
	for name, value := range env {
		envStrings = append(envStrings, fmt.Sprintf("%s=%s", name, value))
	}
	return envStrings
}
