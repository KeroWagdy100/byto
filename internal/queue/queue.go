package queue

import (
	"byto/internal/domain"
	"errors"
	"sync"
)

type Queue struct {
	items []*domain.Media
	mu    sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		items: make([]*domain.Media, 0),
	}
}

func (q *Queue) Add(media *domain.Media) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if media.ID == "" {
		return
	}
	q.items = append(q.items, media)
}

func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, media := range q.items {
		if media.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return nil
		}
	}
	return errors.New("media with given ID not found")
}

func (q *Queue) GetAll() []*domain.Media {
	q.mu.Lock()
	defer q.mu.Unlock()
	itemsCopy := make([]*domain.Media, len(q.items))
	copy(itemsCopy, q.items)
	return itemsCopy
}

func (q *Queue) Get(id string) (*domain.Media, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, media := range q.items {
		if media.ID == id {
			return media, nil
		}
	}
	return nil, errors.New("media not found")
}
