package cache

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "time"

 "cordialcrm/backend/internal/config"
 "github.com/redis/go-redis/v9"
)
type Cache struct { Client *redis.Client; Prefix string; TTL time.Duration }
type Session struct { UserID string `json:"user_id"`; Version int `json:"version"` }
func New(c config.Config)(*Cache,error){opt,e:=redis.ParseURL(c.RedisURL);if e!=nil{return nil,errors.New("CRM_REDIS_URL 无效")}; opt.DialTimeout=3*time.Second;opt.ReadTimeout=3*time.Second;opt.WriteTimeout=3*time.Second;return &Cache{redis.NewClient(opt),c.RedisPrefix,c.SessionTTL},nil}
func Hash(value string)string{sum:=sha256.Sum256([]byte(value));return hex.EncodeToString(sum[:])}
func(c *Cache) Put(ctx context.Context,token string,s Session)error{b,e:=json.Marshal(s);if e!=nil{return e};return c.Client.Set(ctx,c.Prefix+"session:"+Hash(token),b,c.TTL).Err()}
func(c *Cache) Get(ctx context.Context,token string)(Session,error){var s Session;b,e:=c.Client.Get(ctx,c.Prefix+"session:"+Hash(token)).Bytes();if e!=nil{return s,e};e=json.Unmarshal(b,&s);return s,e}
func(c *Cache) Delete(ctx context.Context,token string)error{return c.Client.Del(ctx,c.Prefix+"session:"+Hash(token)).Err()}
func(c *Cache) LoginAttempt(ctx context.Context,username string)(int64,error){
 const script=`local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],900) end; return n`
 return c.Client.Eval(ctx,script,[]string{c.Prefix+"login:"+Hash(username)}).Int64()
}
func(c *Cache) ResetAttempts(ctx context.Context,username string)error{return c.Client.Del(ctx,c.Prefix+"login:"+Hash(username)).Err()}
