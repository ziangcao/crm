package config

import "testing"

func TestLoadRejectsPartialWeComConfiguration(t *testing.T) {
	t.Setenv("CRM_DB_DRIVER", "postgres")
	t.Setenv("CRM_DB_DSN", "postgres://crm:password@127.0.0.1:5432/crm?sslmode=disable")
	t.Setenv("CRM_WECOM_CORP_ID", "ww-test")
	t.Setenv("CRM_WECOM_AGENT_ID", "")
	t.Setenv("CRM_WECOM_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected partial WeCom configuration to be rejected")
	}
}

func TestLoadAcceptsCompleteWeComConfiguration(t *testing.T) {
	t.Setenv("CRM_DB_DRIVER", "postgres")
	t.Setenv("CRM_DB_DSN", "postgres://crm:password@127.0.0.1:5432/crm?sslmode=disable")
	t.Setenv("CRM_WECOM_CORP_ID", "ww-test")
	t.Setenv("CRM_WECOM_AGENT_ID", "1000002")
	t.Setenv("CRM_WECOM_SECRET", "secret")
	if _, err := Load(); err != nil {
		t.Fatalf("expected complete WeCom configuration to load: %v", err)
	}
}
