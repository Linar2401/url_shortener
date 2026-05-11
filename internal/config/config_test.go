package config

import "testing"

func TestNewDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	if c.ServeAddress != "localhost:8080" {
		t.Errorf("ServeAddress: %s", c.ServeAddress)
	}
	if c.ResultAddress != "http://localhost:8080" {
		t.Errorf("ResultAddress: %s", c.ResultAddress)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel: %s", c.LogLevel)
	}
	if c.AuthSecret == "" {
		t.Error("AuthSecret should have a non-empty default")
	}
	if c.FileStoragePath != "" || c.DatabaseDSN != "" || c.AuditFile != "" || c.AuditURL != "" {
		t.Errorf("optional fields should be empty by default: %+v", c)
	}
}
