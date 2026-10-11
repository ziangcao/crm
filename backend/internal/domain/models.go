package domain

import (
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "time"
)

type Data map[string]any
type Error struct { Status int; Message string }
func (e *Error) Error() string { return e.Message }
func Fail(status int, message string) error { return &Error{status, message} }
func ID() string { b := make([]byte,12); if _, err := rand.Read(b); err != nil { panic(err) }; return hex.EncodeToString(b) }
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func Text(v any) string { if v == nil { return "" }; if s,ok:=v.(string); ok{return s}; return fmt.Sprint(v) }
func JSON(v any) string { b,err:=json.Marshal(v); if err!=nil { panic(err) }; return string(b) }

type User struct {
 ID string `json:"id" gorm:"primaryKey;size:24"`
 Username string `json:"username" gorm:"size:80;uniqueIndex;not null"`
 Name string `json:"name" gorm:"size:80;not null"`
 Password string `json:"-" gorm:"size:200;not null"`
 Role string `json:"role" gorm:"size:16;not null"`
 Department string `json:"department" gorm:"size:80;not null"`
 Active bool `json:"active" gorm:"not null"`
 AuthVersion int `json:"-" gorm:"not null;default:1"`
 CreatedAt string `json:"created_at" gorm:"size:40;autoCreateTime:false"`
}
type Record struct {
 ID string `json:"id" gorm:"primaryKey;size:24"`
 Module string `json:"module" gorm:"size:40;index:idx_module_owner"`
 Data Data `json:"data" gorm:"serializer:json;type:longtext"`
 OwnerID *string `json:"owner_id" gorm:"size:24;index:idx_module_owner"`
 Version int `json:"version" gorm:"not null"`
 CreatedAt string `json:"created_at" gorm:"size:40;autoCreateTime:false"`
 UpdatedAt string `json:"updated_at" gorm:"size:40;autoUpdateTime:false;index"`
 OwnerName string `json:"owner_name" gorm:"-"`
 References map[string]string `json:"references" gorm:"-"`
}
type Followup struct {
 ID string `json:"id" gorm:"primaryKey;size:24"`
 RecordID string `json:"record_id" gorm:"size:24;index"`
 UserID string `json:"user_id" gorm:"size:24"`
 Content string `json:"content" gorm:"type:text"`
 Method string `json:"method" gorm:"size:20"`
 NextDate *string `json:"next_date" gorm:"size:10"`
 CreatedAt string `json:"created_at" gorm:"size:40;autoCreateTime:false"`
 UserName string `json:"user_name" gorm:"-"`
}
type Approval struct {
 ID string `json:"id" gorm:"primaryKey;size:24"`
 RecordID string `json:"record_id" gorm:"size:24;index"`
 RequestedBy string `json:"requested_by" gorm:"size:24"`
 Status string `json:"status" gorm:"size:16;index"`
 ReviewedBy *string `json:"reviewed_by" gorm:"size:24"`
 Comment string `json:"comment" gorm:"type:text"`
 CreatedAt string `json:"created_at" gorm:"size:40;autoCreateTime:false"`
 ReviewedAt *string `json:"reviewed_at" gorm:"size:40"`
 Module string `json:"module" gorm:"-"`
 Name string `json:"name" gorm:"-"`
 OwnerID *string `json:"owner_id" gorm:"-"`
 RequesterName string `json:"requester_name" gorm:"-"`
 ReviewerName string `json:"reviewer_name" gorm:"-"`
}
type Audit struct {
 ID string `json:"id" gorm:"primaryKey;size:24"`
 UserID string `json:"user_id" gorm:"size:24"`
 Action string `json:"action" gorm:"size:80"`
 Module string `json:"module" gorm:"size:40"`
 RecordID *string `json:"record_id" gorm:"size:24"`
 Detail string `json:"detail" gorm:"type:text"`
 CreatedAt string `json:"created_at" gorm:"size:40;autoCreateTime:false;index"`
 UserName string `json:"user_name" gorm:"-"`
}
func (Audit) TableName() string { return "audit" }
type CustomField struct {
 Module string `gorm:"primaryKey;size:40" json:"module"`
 Key string `gorm:"primaryKey;size:48" json:"key"`
 Definition string `gorm:"type:text" json:"definition"`
}
type MutationLock struct { ID int `gorm:"primaryKey;autoIncrement:false"` }
type Migration struct { Version int `gorm:"primaryKey;autoIncrement:false"`; AppliedAt string `gorm:"size:40"` }
