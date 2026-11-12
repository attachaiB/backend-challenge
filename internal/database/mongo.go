package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const UserCollection = "users"

func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())

		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	return client, nil
}

// EnsureIndexes creates the unique index on email. MongoDB has no schema,
// so this index is what actually guarantees that an email is used only once.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(UserCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetName("email_unique").SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create email index: %w", err)
	}

	return nil
}
