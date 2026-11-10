package main

import (
	"testing"
)

func TestConfigValidateMissing(t *testing.T) {
	c := &Config{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error for empty config")
	}
}

func TestConfigValidateExampleCreds(t *testing.T) {
	c := &Config{
		Server: ServerConfig{Listen: "127.0.0.1:8443", TLSEnabled: true, TLSCert: "cert", TLSKey: "key"},
		API:    APIConfig{Username: "u", Password: "CHANGE_THIS"},
		AMI:    AMIConfig{Host: "127.0.0.1", Port: 5038, Username: "ami", Secret: "CHANGE_THIS"},
		Asterisk: AsteriskConfig{ChannelDriver: "pjsip", TrunkName: "YOUR_TRUNK", DialFormat: "CONFIGURE_FOR_YOUR_TRUNK", CallerID: "CONFIGURED_CALLER_ID"},
		Security: SecurityConfig{AllowedClientCIDRs: []string{"127.0.0.1/32"}},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error for example credentials")
	}
}

func TestConfigDialStringNoCallerInput(t *testing.T) {
	c := &Config{
		Asterisk: AsteriskConfig{ChannelDriver: "pjsip", TrunkName: "mytrunk", DialFormat: "PJSIP/mytrunk/%s"},
	}
	got := c.DialString("+989121234567")
	want := "PJSIP/mytrunk/+989121234567"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConfigValidateAPINotAMISecret(t *testing.T) {
	c := &Config{
		Server: ServerConfig{Listen: "127.0.0.1:8443", TLSEnabled: true, TLSCert: "cert", TLSKey: "key"},
		API:    APIConfig{Username: "u", Password: "same-secret"},
		AMI:    AMIConfig{Host: "127.0.0.1", Port: 5038, Username: "ami", Secret: "same-secret"},
		Asterisk: AsteriskConfig{ChannelDriver: "pjsip", TrunkName: "t", DialFormat: "PJSIP/t/%s", CallerID: "c"},
		Security: SecurityConfig{AllowedClientCIDRs: []string{"127.0.0.1/32"}},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error when api.password equals ami.secret")
	}
}
