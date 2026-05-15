package server

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Conversation struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title" gorm:"not null"`
	Archived  bool      `json:"archived" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

type ChatMessage struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	ConversationID  string    `json:"conversation_id" gorm:"not null;index"`
	ParentMessageID string    `json:"parent_message_id" gorm:"index"`
	Role            string    `json:"role" gorm:"not null"`
	Content         string    `json:"content" gorm:"type:text;not null"`
	Rounds          string    `json:"rounds" gorm:"type:text;not null"`
	CreatedAt       time.Time `json:"created_at" gorm:"not null"`

	Conversation Conversation `gorm:"foreignKey:ConversationID"`
}

type OffloadEntry struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text;not null"`
}

type DB struct {
	db *gorm.DB
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

func NewDB(config DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&Conversation{}, &ChatMessage{}, &OffloadEntry{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &DB{db: db}, nil
}

func (d *DB) CreateConversation(ctx context.Context, conv *Conversation) error {
	return d.db.WithContext(ctx).Create(conv).Error
}

func (d *DB) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	var conv Conversation
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (d *DB) ListConversations(ctx context.Context) ([]*Conversation, error) {
	var convs []*Conversation
	err := d.db.WithContext(ctx).Order("updated_at DESC").Find(&convs).Error
	return convs, err
}

func (d *DB) UpdateConversation(ctx context.Context, id string, updatedAt time.Time) error {
	return d.db.WithContext(ctx).Model(&Conversation{}).Where("id = ?", id).Update("updated_at", updatedAt).Error
}

func (d *DB) UpdateConversationTitle(ctx context.Context, id string, title string) error {
	return d.db.WithContext(ctx).Model(&Conversation{}).Where("id = ?", id).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()}).Error
}

func (d *DB) ArchiveConversation(ctx context.Context, id string, archived bool) error {
	return d.db.WithContext(ctx).Model(&Conversation{}).Where("id = ?", id).
		Updates(map[string]interface{}{"archived": archived, "updated_at": time.Now()}).Error
}

func (d *DB) DeleteConversation(ctx context.Context, id string) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", id).Delete(&ChatMessage{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&Conversation{}).Error
	})
}

func (d *DB) ListMessages(ctx context.Context, conversationID string) ([]*ChatMessage, error) {
	var messages []*ChatMessage
	err := d.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

func (d *DB) CreateMessage(ctx context.Context, msg *ChatMessage) error {
	return d.db.WithContext(ctx).Create(msg).Error
}

func (d *DB) GetMessageChain(ctx context.Context, conversationID, messageID string) ([]*ChatMessage, error) {
	var messages []*ChatMessage
	currentID := messageID

	for currentID != "" {
		var msg ChatMessage
		err := d.db.WithContext(ctx).Where("id = ?", currentID).First(&msg).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return nil, err
		}
		messages = append([]*ChatMessage{&msg}, messages...)
		currentID = msg.ParentMessageID
	}

	return messages, nil
}

func (d *DB) DeleteMessagesByConversation(ctx context.Context, conversationID string) error {
	return d.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Delete(&ChatMessage{}).Error
}

func (d *DB) Raw() *gorm.DB {
	return d.db
}

func (d *DB) StoreOffload(ctx context.Context, key string, value string) error {
	return d.db.WithContext(ctx).Save(&OffloadEntry{Key: key, Value: value}).Error
}

func (d *DB) LoadOffload(ctx context.Context, key string) (string, error) {
	var entry OffloadEntry
	err := d.db.WithContext(ctx).Where("key = ?", key).First(&entry).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("offload key not found: %s", key)
		}
		return "", err
	}
	return entry.Value, nil
}

func (d *DB) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type DBStorage struct {
	db *DB
}

func NewDBStorage(db *DB) *DBStorage {
	return &DBStorage{db: db}
}

func (s *DBStorage) Store(ctx context.Context, key string, value string) error {
	return s.db.StoreOffload(ctx, key, value)
}

func (s *DBStorage) Load(ctx context.Context, key string) (string, error) {
	return s.db.LoadOffload(ctx, key)
}
