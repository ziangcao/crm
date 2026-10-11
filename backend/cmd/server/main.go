package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cordialcrm/backend/internal/config"
	"cordialcrm/backend/internal/controller"
	"cordialcrm/backend/internal/integration"
	"cordialcrm/backend/internal/repo"
	"cordialcrm/backend/internal/usecase"
	"cordialcrm/backend/internal/wecom"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("server stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	var dialector gorm.Dialector
	if c.Driver == "postgres" {
		dialector = postgres.Open(c.DSN)
	} else {
		dialector = mysql.Open(c.DSN)
	}
	database, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return errors.New("数据库连接失败，请检查配置及服务状态")
	}
	repository := repo.New(database)
	attachments, err := repo.NewFileStorage(c.UploadDir)
	if err != nil {
		return err
	}
	passwords := controller.Passwords{}
	auth := usecase.NewAuth(repository, passwords, controller.Tokens{}, c.SessionTTL)
	users := usecase.NewUser(repository, passwords)
	records := usecase.NewRecord(repository, repository, repository)
	weComClient := wecom.New(c.WeComCorpID, c.WeComAgentID, c.WeComSecret)
	weComClient.Configure(c.WeComCorpID, c.WeComAgentID, c.WeComSecret, c.WeComOAuthRedirect)
	notificationSenders := []usecase.NotificationSender{weComClient}
	notifications := usecase.NewNotification(repository, notificationSenders...)
	integrationNames := map[string]string{"feishu": "飞书", "dingtalk": "钉钉", "sso": "SSO", "tax": "税务开票", "bank": "银行对账", "tender": "自动标讯"}
	connectors := make([]integration.Connector, 0, len(integrationNames))
	for key, name := range integrationNames {
		connectors = append(connectors, integration.Connector{Key: key, Name: name, Endpoint: c.IntegrationEndpoints[key], Token: c.IntegrationTokens[key], Mock: c.IntegrationMock})
	}
	integrations := integration.New(connectors...)
	encryptionMaterial := c.ConfigEncryptionKey
	if encryptionMaterial == "" {
		encryptionMaterial = "cordialcrm/integrations:" + c.DSN
	}
	encryptionKey := sha256.Sum256([]byte(encryptionMaterial))
	weComDefault := usecase.WeComConfig{CorpID: c.WeComCorpID, AgentID: c.WeComAgentID, Secret: c.WeComSecret, RedirectURL: c.WeComOAuthRedirect}
	integrationSettings, err := usecase.NewIntegrationSetting(repository, integrations, weComClient, weComDefault, encryptionKey[:])
	if err != nil {
		return err
	}
	if err := integrationSettings.Load(context.Background()); err != nil {
		return err
	}
	reminders := usecase.NewFollowupReminder(repository)
	recycler := usecase.NewCustomerRecycle(repository, records)
	router := controller.New(auth, users, usecase.NewAPIKey(repository, passwords, controller.Tokens{}), usecase.NewAgent(repository), usecase.NewDashboardConfig(repository), integrationSettings, records, usecase.NewApproval(repository, notifications), usecase.NewAudit(repository), usecase.NewSystem(repository), usecase.NewAttachment(repository, attachments), notifications, usecase.NewFollowupPlan(repository), usecase.NewApprovalFlow(repository), weComClient, integrations, c.CookieSecure, c.Frontend).Router()
	server := &http.Server{Addr: c.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runFollowupReminders(signals, log, reminders)
	go runCustomerRecycle(signals, log, recycler, c.CustomerRecycleDays)
	stopped := make(chan error, 1)
	go func() { stopped <- server.ListenAndServe() }()
	log.Info("server starting", "address", c.Address, "database_driver", c.Driver)
	select {
	case err = <-stopped:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-signals.Done():
		deadline, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		return server.Shutdown(deadline)
	}
	return nil
}

func runCustomerRecycle(ctx context.Context, log *slog.Logger, recycler *usecase.CustomerRecycleUseCase, idleDays int) {
	if idleDays <= 0 {
		return
	}
	run := func() {
		output, err := recycler.Run(ctx, usecase.RunCustomerRecycleInput{IdleDays: idleDays})
		if err != nil {
			log.Error("customer recycle run failed", "reason", err.Error())
			return
		}
		if output.Recycled > 0 {
			log.Info("customers recycled", "count", output.Recycled, "candidates", output.Candidates)
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func runFollowupReminders(ctx context.Context, log *slog.Logger, reminders *usecase.FollowupReminderUseCase) {
	run := func() {
		output, err := reminders.Run(ctx, usecase.RunFollowupRemindersInput{})
		if err != nil {
			log.Error("followup reminder run failed", "reason", err.Error())
			return
		}
		if output.Delivered > 0 {
			log.Info("followup reminders delivered", "count", output.Delivered, "candidates", output.Candidates)
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
