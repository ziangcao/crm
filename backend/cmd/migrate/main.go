package main

import (
	"context"
	"cordialcrm/backend/internal/config"
	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/repo"
	"cordialcrm/backend/internal/usecase"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"time"
)

func main() {
	c, e := config.Load()
	if e != nil {
		log.Fatal(e)
	}
	var d gorm.Dialector
	if c.Driver == "postgres" {
		d = postgres.Open(c.DSN)
	} else {
		d = mysql.Open(c.DSN)
	}
	db, e := gorm.Open(d, &gorm.Config{})
	if e != nil {
		log.Fatal("数据库连接失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repository := repo.New(db)
	if e = repository.Migrate(ctx); e != nil {
		log.Fatal("数据库迁移失败，请检查权限与数据库版本")
	}
	modules := make([]entity.ModuleDefinition, 0, len(usecase.Modules))
	for key, module := range usecase.Modules {
		modules = append(modules, entity.ModuleDefinition{Key: key, Name: module.Label})
	}
	if e = repository.BootstrapModules(ctx, modules); e != nil {
		log.Fatal("业务模块初始化失败")
	}
	if result, unambiguous, migrationErr := usecase.NewOrganizationMigration(repository, repository).AssignUnambiguous(ctx, true); migrationErr != nil {
		log.Fatal("历史组织归属迁移失败")
	} else if unambiguous && (result.Users > 0 || result.Records > 0) {
		log.Printf("历史数据已归属组织 %s：成员 %d，记录 %d", result.OrganizationID, result.Users, result.Records)
	}
	log.Print("database migration complete")
}
