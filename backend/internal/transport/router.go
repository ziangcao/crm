package transport

import (
 "context"
 "errors"
 "log/slog"
 "net/http"
 "path/filepath"
 "slices"
 "strings"
 "time"

 "cordialcrm/backend/internal/cache"
 "cordialcrm/backend/internal/config"
 "cordialcrm/backend/internal/domain"
 "cordialcrm/backend/internal/repository"
 "github.com/gin-gonic/gin"
)

func Foundation(c config.Config, store *repository.Store, redis *cache.Cache, log *slog.Logger)*gin.Engine{
 gin.SetMode(gin.ReleaseMode)
 r:=gin.New();_ = r.SetTrustedProxies(nil)
 r.Use(func(ctx *gin.Context){
  started:=time.Now();requestID:=domain.ID();ctx.Header("X-Request-ID",requestID)
  ctx.Header("X-Content-Type-Options","nosniff");ctx.Header("X-Frame-Options","DENY");ctx.Header("Referrer-Policy","same-origin")
  if strings.HasPrefix(ctx.Request.URL.Path,"/api"){ctx.Header("Cache-Control","no-store")}
  defer func(){if recover()!=nil{log.Error("request panic","request_id",requestID);ctx.AbortWithStatusJSON(500,gin.H{"detail":"服务器内部错误"})};log.Info("http request","request_id",requestID,"method",ctx.Request.Method,"route",ctx.FullPath(),"status",ctx.Writer.Status(),"duration_ms",time.Since(started).Milliseconds())}()
  ctx.Next()
 })
 r.Use(func(ctx *gin.Context){
  if strings.HasPrefix(ctx.Request.URL.Path,"/api") && !slices.Contains([]string{"GET","HEAD","OPTIONS"},ctx.Request.Method){
   scheme:="http";if ctx.Request.TLS!=nil{scheme="https"}
   origin:=ctx.GetHeader("Origin")
   if (origin!="" && origin!=scheme+"://"+ctx.Request.Host && !slices.Contains(c.AllowedOrigins,origin)) || ctx.GetHeader("Sec-Fetch-Site")=="cross-site" {ctx.AbortWithStatusJSON(403,gin.H{"detail":"不允许跨站请求"});return}
   if ctx.Request.ContentLength>2_000_000{ctx.AbortWithStatusJSON(413,gin.H{"detail":"请求不能超过 2 MB"});return}
   ctx.Request.Body=http.MaxBytesReader(ctx.Writer,ctx.Request.Body,2_000_000)
  };ctx.Next()
 })
 r.GET("/api/health",func(ctx *gin.Context){ctx.JSON(200,gin.H{"status":"ok"})})
 r.GET("/api/ready",func(ctx *gin.Context){
  deadline,cancel:=context.WithTimeout(ctx.Request.Context(),3*time.Second);defer cancel()
  if store.Ping(deadline)!=nil || redis.Client.Ping(deadline).Err()!=nil{ctx.JSON(503,gin.H{"detail":"数据库或 Redis 不可用"});return};ctx.JSON(200,gin.H{"status":"ready"})
 })
 r.Static("/assets",filepath.Join(c.Frontend,"assets"));r.GET("/",func(ctx *gin.Context){ctx.File(filepath.Join(c.Frontend,"index.html"))})
 r.NoRoute(func(ctx *gin.Context){ctx.JSON(404,gin.H{"detail":"接口不存在"})})
 return r
}
func respond(ctx *gin.Context,status int,value any,err error){
 if err==nil{ctx.JSON(status,value);return};var business *domain.Error
 if errors.As(err,&business){ctx.JSON(business.Status,gin.H{"detail":business.Message});return}
 if repository.Conflict(err){ctx.JSON(409,gin.H{"detail":"记录已存在"});return}
 slog.Error("request failed","request_id",ctx.Writer.Header().Get("X-Request-ID"),"error_type",strings.SplitN(strings.ReplaceAll(http.StatusText(500)," ","_")," ",1)[0])
 ctx.JSON(500,gin.H{"detail":"服务器内部错误"})
}
