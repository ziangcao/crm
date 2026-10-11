package repository

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "time"

 "cordialcrm/backend/internal/config"
 "cordialcrm/backend/internal/domain"
 "gorm.io/driver/mysql"
 "gorm.io/driver/postgres"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
 "gorm.io/gorm/logger"
)

type Store struct { DB *gorm.DB }
type Tx struct { db *gorm.DB }

func Open(c config.Config) (*Store,error) {
 var driver gorm.Dialector
 switch c.Driver { case "postgres": driver=postgres.Open(c.DSN); case "mysql": driver=mysql.Open(c.DSN); default:return nil,fmt.Errorf("unsupported database") }
 db,err:=gorm.Open(driver,&gorm.Config{Logger:logger.Default.LogMode(logger.Silent),TranslateError:true})
 if err!=nil{return nil,fmt.Errorf("数据库连接失败，请检查配置及服务状态")}
 pool,err:=db.DB();if err!=nil{return nil,err};pool.SetMaxOpenConns(20);pool.SetMaxIdleConns(5);pool.SetConnMaxLifetime(30*time.Minute)
 return &Store{DB:db},nil
}
func (s *Store) Close() error { db,e:=s.DB.DB();if e!=nil{return e};return db.Close() }
func (s *Store) Ping(ctx context.Context) error {db,e:=s.DB.DB();if e!=nil{return e};return db.PingContext(ctx)}

// Explicit migration command, rather than changing schema on every server boot.
func (s *Store) Migrate(ctx context.Context) error {
 db:=s.DB.WithContext(ctx)
 if err:=db.AutoMigrate(&domain.Migration{});err!=nil{return err}
 var applied int64
 if err:=db.Model(&domain.Migration{}).Where("version = ?",1).Count(&applied).Error;err!=nil{return err}
 if applied>0{return nil}
 // JSON uses TEXT on both dialects to retain identical decimal strings and API values.
 if err:=db.AutoMigrate(&domain.User{},&domain.Record{},&domain.Followup{},&domain.Approval{},&domain.Audit{},&domain.CustomField{},&domain.MutationLock{});err!=nil{return err}
 if err:=db.Clauses(clause.OnConflict{DoNothing:true}).Create(&domain.MutationLock{ID:1}).Error;err!=nil{return err}
 return db.Create(&domain.Migration{Version:1,AppliedAt:domain.Now()}).Error
}
func (s *Store) Read(ctx context.Context, fn func(*Tx) error) error {
 return s.DB.WithContext(ctx).Transaction(func(db *gorm.DB) error{return fn(&Tx{db})},&sql.TxOptions{Isolation:sql.LevelRepeatableRead,ReadOnly:true})
}
func (s *Store) Write(ctx context.Context, fn func(*Tx) error) error {
 return s.DB.WithContext(ctx).Transaction(func(db *gorm.DB) error {
  var lock domain.MutationLock
  if err:=db.Clauses(clause.Locking{Strength:"UPDATE"}).First(&lock,1).Error;err!=nil{return err}
  return fn(&Tx{db})
 },&sql.TxOptions{Isolation:sql.LevelReadCommitted})
}
func All[T any](tx *Tx, order string, query string,args ...any)([]T,error){
 result:=make([]T,0);db:=tx.db;if query!=""{db=db.Where(query,args...)};if order!=""{db=db.Order(order)};err:=db.Find(&result).Error;return result,err
}
func One[T any](tx *Tx,query string,args ...any)(T,error){var value T;err:=tx.db.Where(query,args...).First(&value).Error;return value,err}
func Create[T any](tx *Tx,v *T)error{return tx.db.Create(v).Error}
func Save[T any](tx *Tx,v *T)error{return tx.db.Save(v).Error}
func Delete[T any](tx *Tx,query string,args ...any)error{var v T;return tx.db.Where(query,args...).Delete(&v).Error}
func Missing(err error)bool{return errors.Is(err,gorm.ErrRecordNotFound)}
func Conflict(err error)bool{return errors.Is(err,gorm.ErrDuplicatedKey)}
