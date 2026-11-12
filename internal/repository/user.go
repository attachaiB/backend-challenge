package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/attachaiB/backend-challenge/internal/database"
	"github.com/attachaiB/backend-challenge/internal/entity"
)

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) UserRepository {
	return &userRepository{collection: db.Collection(database.UserCollection)}
}

func (r *userRepository) Create(ctx context.Context, user entity.User) (entity.User, error) {
	user.ID = bson.NewObjectID()

	if _, err := r.collection.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return entity.User{}, ErrDuplicateEmail
		}

		return entity.User{}, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id string) (entity.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		// A malformed ID can never match a document.
		return entity.User{}, ErrNotFound
	}

	return r.findOne(ctx, bson.D{{Key: "_id", Value: objectID}})
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (entity.User, error) {
	return r.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *userRepository) FindAll(ctx context.Context) ([]entity.User, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}

	users := []entity.User{}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	return users, nil
}

func (r *userRepository) Update(
	ctx context.Context,
	id string,
	update entity.UserUpdate,
) (entity.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return entity.User{}, ErrNotFound
	}

	set := bson.D{}
	if update.Name != nil {
		set = append(set, bson.E{Key: "name", Value: *update.Name})
	}

	if update.Email != nil {
		set = append(set, bson.E{Key: "email", Value: *update.Email})
	}

	// ReturnDocument(After) gives back the document as it is after the update,
	// so we do not need a second query.
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var user entity.User

	err = r.collection.FindOneAndUpdate(
		ctx,
		bson.D{{Key: "_id", Value: objectID}},
		bson.D{{Key: "$set", Value: set}},
		opts,
	).Decode(&user)

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return entity.User{}, ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return entity.User{}, ErrDuplicateEmail
	case err != nil:
		return entity.User{}, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return ErrNotFound
	}

	result, err := r.collection.DeleteOne(ctx, bson.D{{Key: "_id", Value: objectID}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *userRepository) Count(ctx context.Context) (int64, error) {
	total, err := r.collection.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return total, nil
}

func (r *userRepository) findOne(ctx context.Context, filter bson.D) (entity.User, error) {
	var user entity.User

	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return entity.User{}, ErrNotFound
	}

	if err != nil {
		return entity.User{}, fmt.Errorf("find user: %w", err)
	}

	return user, nil
}
