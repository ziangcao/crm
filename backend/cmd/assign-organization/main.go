package main

import (
	"context"
	"flag"
	"log"
	"time"

	"cordialcrm/backend/internal/config"
	"cordialcrm/backend/internal/repo"
	"cordialcrm/backend/internal/usecase"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	organizationID := flag.String("organization-id", "", "target organization ID")
	apply := flag.Bool("apply", false, "persist changes; default is preview only")
	flag.Parse()
	if *organizationID == "" {
		log.Fatal("请提供 --organization-id")
	}
	configuration, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	var dialector gorm.Dialector
	if configuration.Driver == "postgres" {
		dialector = postgres.Open(configuration.DSN)
	} else {
		dialector = mysql.Open(configuration.DSN)
	}
	database, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repository := repo.New(database)
	result, err := usecase.NewOrganizationMigration(repository, repository).Assign(ctx, usecase.AssignOrganizationInput{OrganizationID: *organizationID, Apply: *apply})
	if err != nil {
		log.Fatal(err)
	}
	if result.Applied {
		log.Printf("已归属组织 %s：成员 %d，记录 %d", result.OrganizationID, result.Users, result.Records)
		return
	}
	log.Printf("预览：将归属组织 %s 的成员 %d、记录 %d。确认备份后加 --apply 执行。", result.OrganizationID, result.Users, result.Records)
}
