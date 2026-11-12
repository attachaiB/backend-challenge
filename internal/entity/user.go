package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User is the document stored in the "users" collection.
type User struct {
	ID           bson.ObjectID `bson:"_id"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"password"`
	CreatedAt    time.Time     `bson:"created_at"`
}

// UserUpdate holds the fields that may be changed. A nil field is left as it is.
type UserUpdate struct {
	Name  *string
	Email *string
}
