package service

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/attachaiB/backend-challenge/internal/entity"
	"github.com/attachaiB/backend-challenge/internal/repository"
)

// fakeUserRepository is an in-memory UserRepository used instead of MongoDB.
// It keeps the same rules as the real one: unique email and ErrNotFound.
// Setting err makes every method fail with that error.
type fakeUserRepository struct {
	users      map[string]entity.User
	order      []string
	err        error
	lastUpdate entity.UserUpdate
}

func newFakeUserRepository(users ...entity.User) *fakeUserRepository {
	repo := &fakeUserRepository{users: map[string]entity.User{}}
	for _, user := range users {
		repo.users[user.ID.Hex()] = user
		repo.order = append(repo.order, user.ID.Hex())
	}

	return repo
}

func (r *fakeUserRepository) Create(_ context.Context, user entity.User) (entity.User, error) {
	if r.err != nil {
		return entity.User{}, r.err
	}

	if r.emailUsedBy(user.Email, "") {
		return entity.User{}, repository.ErrDuplicateEmail
	}

	user.ID = bson.NewObjectID()
	r.users[user.ID.Hex()] = user
	r.order = append(r.order, user.ID.Hex())

	return user, nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, id string) (entity.User, error) {
	if r.err != nil {
		return entity.User{}, r.err
	}

	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}

	return user, nil
}

func (r *fakeUserRepository) FindByEmail(_ context.Context, email string) (entity.User, error) {
	if r.err != nil {
		return entity.User{}, r.err
	}

	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}

	return entity.User{}, repository.ErrNotFound
}

func (r *fakeUserRepository) FindAll(context.Context) ([]entity.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	users := []entity.User{}
	for _, id := range r.order {
		if user, ok := r.users[id]; ok {
			users = append(users, user)
		}
	}

	return users, nil
}

func (r *fakeUserRepository) Update(
	_ context.Context,
	id string,
	update entity.UserUpdate,
) (entity.User, error) {
	r.lastUpdate = update

	if r.err != nil {
		return entity.User{}, r.err
	}

	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}

	if update.Email != nil && r.emailUsedBy(*update.Email, id) {
		return entity.User{}, repository.ErrDuplicateEmail
	}

	if update.Name != nil {
		user.Name = *update.Name
	}

	if update.Email != nil {
		user.Email = *update.Email
	}

	r.users[id] = user

	return user, nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id string) error {
	if r.err != nil {
		return r.err
	}

	if _, ok := r.users[id]; !ok {
		return repository.ErrNotFound
	}

	delete(r.users, id)

	return nil
}

func (r *fakeUserRepository) Count(context.Context) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return int64(len(r.users)), nil
}

func (r *fakeUserRepository) emailUsedBy(email, exceptID string) bool {
	for id, user := range r.users {
		if user.Email == email && id != exceptID {
			return true
		}
	}

	return false
}
