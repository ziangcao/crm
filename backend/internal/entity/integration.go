package entity

import "time"

type IntegrationSetting struct {
	Key, Endpoint, EncryptedToken string
	UpdatedAt                     time.Time
}
