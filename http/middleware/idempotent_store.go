package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/henrywhitaker3/windowframe/uuid"
	"github.com/redis/rueidis"
)

type IdempotentRueidisStore struct {
	client rueidis.Client
}

func NewIdempotentRueidisStore(client rueidis.Client) *IdempotentRueidisStore {
	return &IdempotentRueidisStore{
		client: client,
	}
}

func (i *IdempotentRueidisStore) Lock(ctx context.Context, key uuid.UUID, ttl time.Duration) error {
	cmd := i.client.B().
		Set().
		Key(fmt.Sprintf("%s:lock", key.String())).
		Value("1").
		Nx().
		Ex(ttl).
		Build()
	err := i.client.Do(ctx, cmd).Error()
	if rueidis.IsRedisNil(err) {
		return ErrIdempotentLocked
	}
	if err != nil {
		return fmt.Errorf("setnx key: %w", err)
	}
	return nil
}

func (i *IdempotentRueidisStore) Unlock(ctx context.Context, key uuid.UUID) error {
	cmd := i.client.B().Del().Key(fmt.Sprintf("%s:lock", key.String())).Build()
	if err := i.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("del key: %w", err)
	}
	return nil
}

func (i *IdempotentRueidisStore) Get(ctx context.Context, key uuid.UUID) ([]byte, error) {
	cmd := i.client.B().Get().Key(key.String()).Build()
	res := i.client.Do(ctx, cmd)
	if err := res.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, ErrIdempotentMissing
		}
		return nil, fmt.Errorf("get key: %w", err)
	}

	val, err := res.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("get result as bytes: %w", err)
	}

	return val, nil
}

func (i *IdempotentRueidisStore) Store(
	ctx context.Context,
	key uuid.UUID,
	ttl time.Duration,
	resp []byte,
) error {
	cmd := i.client.B().Set().Key(key.String()).Value(string(resp)).Nx().Ex(ttl).Build()
	if err := i.client.Do(ctx, cmd).Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil
		}
		return fmt.Errorf("store result: %w", err)
	}
	return nil
}

var _ IdempotencyStore = &IdempotentRueidisStore{}
