package usecase

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

const maxAttachmentSize = 20 << 20

type AttachmentUseCase struct {
	repo    Repository
	storage AttachmentStorage
}

func NewAttachment(repo Repository, storage AttachmentStorage) *AttachmentUseCase {
	return &AttachmentUseCase{repo: repo, storage: storage}
}

type UploadAttachmentInput struct {
	Actor                               UserOutput
	Module, RecordID, Name, ContentType string
	Content                             []byte
}
type ListAttachmentsInput struct {
	Actor            UserOutput
	Module, RecordID string
}
type DownloadAttachmentInput struct {
	Actor UserOutput
	ID    string
}
type AttachmentOutput struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}
type DownloadAttachmentOutput struct {
	Attachment entity.Attachment
	Content    []byte
}

func (u *AttachmentUseCase) Upload(ctx context.Context, input UploadAttachmentInput) (AttachmentOutput, error) {
	record, err := u.authorize(ctx, input.Actor, input.Module, input.RecordID, true)
	if err != nil {
		return AttachmentOutput{}, err
	}
	name := strings.TrimSpace(filepath.Base(input.Name))
	if name == "." || name == "" || len([]rune(name)) > 255 || len(input.Content) == 0 || len(input.Content) > maxAttachmentSize {
		return AttachmentOutput{}, ErrValidation
	}
	item := entity.Attachment{ID: entity.NewID(), RecordID: record.ID, Module: record.Module, UserID: input.Actor.ID, Name: name, ContentType: strings.TrimSpace(input.ContentType), Version: 1, Size: int64(len(input.Content)), CreatedAt: time.Now().UTC()}
	items, err := u.repo.ListAttachments(ctx, record.ID)
	if err != nil {
		return AttachmentOutput{}, err
	}
	for _, existing := range items {
		if existing.Name == name && existing.Version >= item.Version {
			item.Version = existing.Version + 1
			item.GroupID = existing.GroupID
			if item.GroupID == "" {
				item.GroupID = existing.ID
			}
		}
	}
	if item.GroupID == "" {
		item.GroupID = item.ID
	}
	if err := u.storage.Save(ctx, item.ID, input.Content); err != nil {
		return AttachmentOutput{}, err
	}
	if err := u.repo.CreateAttachment(ctx, item); err != nil {
		_ = u.storage.Delete(ctx, item.ID)
		return AttachmentOutput{}, err
	}
	return attachmentOutput(item), nil
}

func (u *AttachmentUseCase) List(ctx context.Context, input ListAttachmentsInput) ([]AttachmentOutput, error) {
	if _, err := u.authorize(ctx, input.Actor, input.Module, input.RecordID, false); err != nil {
		return nil, err
	}
	items, err := u.repo.ListAttachments(ctx, input.RecordID)
	if err != nil {
		return nil, err
	}
	output := make([]AttachmentOutput, len(items))
	for index, item := range items {
		output[index] = attachmentOutput(item)
	}
	return output, nil
}

func (u *AttachmentUseCase) Download(ctx context.Context, input DownloadAttachmentInput) (DownloadAttachmentOutput, error) {
	item, err := u.repo.FindAttachment(ctx, input.ID)
	if err != nil {
		return DownloadAttachmentOutput{}, ErrNotFound
	}
	if _, err = u.authorize(ctx, input.Actor, item.Module, item.RecordID, false); err != nil {
		return DownloadAttachmentOutput{}, err
	}
	if (item.Module == "quotes" || item.Module == "orders") && !HasModulePermission(input.Actor, item.Module, "DOWNLOAD", false) {
		return DownloadAttachmentOutput{}, ErrForbidden
	}
	content, err := u.storage.Read(ctx, item.ID)
	if err != nil {
		return DownloadAttachmentOutput{}, ErrNotFound
	}
	return DownloadAttachmentOutput{Attachment: item, Content: content}, nil
}

func (u *AttachmentUseCase) Delete(ctx context.Context, input DownloadAttachmentInput) error {
	item, err := u.repo.FindAttachment(ctx, input.ID)
	if err != nil {
		return ErrNotFound
	}
	if _, err = u.authorize(ctx, input.Actor, item.Module, item.RecordID, true); err != nil {
		return err
	}
	if input.Actor.Role != entity.RoleAdmin && item.UserID != input.Actor.ID {
		return ErrForbidden
	}
	if err = u.repo.DeleteAttachment(ctx, item.ID); err != nil {
		return err
	}
	return u.storage.Delete(ctx, item.ID)
}

func (u *AttachmentUseCase) authorize(ctx context.Context, actor UserOutput, moduleKey, recordID string, write bool) (entity.Record, error) {
	m, err := module(moduleKey)
	if err != nil {
		return entity.Record{}, err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, recordID)
	if err != nil {
		return entity.Record{}, ErrNotFound
	}
	if !visible(actor, m, record, write) {
		return entity.Record{}, ErrForbidden
	}
	return record, nil
}

func attachmentOutput(item entity.Attachment) AttachmentOutput {
	return AttachmentOutput{ID: item.ID, Name: item.Name, ContentType: item.ContentType, Size: item.Size, Version: item.Version, CreatedAt: item.CreatedAt}
}
