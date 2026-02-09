package queue_test

import (
	"byto/internal/domain"
	"byto/internal/queue"
	"testing"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name          string
		media         []*domain.Media
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "single item",
			media:         []*domain.Media{{ID: "1", Title: "Test Media"}},
			expectedCount: 1,
			expectedIDs:   []string{"1"},
		},
		{
			name: "multiple items",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			expectedCount: 3,
			expectedIDs:   []string{"1", "2", "3"},
		},
		{
			name:          "empty queue",
			media:         []*domain.Media{},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name: "duplicate IDs",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "1", Title: "Media 1 Duplicate"},
			},
			expectedCount: 2,
			expectedIDs:   []string{"1", "1"},
		},
		{
			name:          "item with empty ID",
			media:         []*domain.Media{{ID: "", Title: "No ID"}},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewQueue()
			for _, m := range tt.media {
				q.Add(m)
			}

			items := q.GetAll()
			if len(items) != tt.expectedCount {
				t.Errorf("got %d items, want %d", len(items), tt.expectedCount)
			}

			for i, expectedID := range tt.expectedIDs {
				if items[i].ID != expectedID {
					t.Errorf("item[%d].ID = %s, want %s", i, items[i].ID, expectedID)
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name          string
		media         []*domain.Media
		removeID      string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "remove first item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			removeID:      "1",
			expectedCount: 2,
			expectedIDs:   []string{"2", "3"},
		},
		{
			name: "remove middle item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			removeID:      "2",
			expectedCount: 2,
			expectedIDs:   []string{"1", "3"},
		},
		{
			name: "remove last item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			removeID:      "3",
			expectedCount: 2,
			expectedIDs:   []string{"1", "2"},
		},
		{
			name: "remove non-existent item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			removeID:      "999",
			expectedCount: 3,
			expectedIDs:   []string{"1", "2", "3"},
		},
		{
			name:          "remove from empty queue",
			media:         []*domain.Media{},
			removeID:      "1",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name: "remove only item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
			},
			removeID:      "1",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewQueue()
			for _, m := range tt.media {
				q.Add(m)
			}

			q.Remove(tt.removeID)

			items := q.GetAll()

			if len(items) != tt.expectedCount {
				t.Errorf("got %d items, want %d", len(items), tt.expectedCount)
			}
			for i, item := range items {
				if item.ID != tt.expectedIDs[i] {
					t.Errorf("item[%d].ID = %s, want %s", i, item.ID, tt.expectedIDs[i])
				}
			}
		})
	}
}

func TestGetAll(t *testing.T) {
	tests := []struct {
		name        string
		media       []*domain.Media
		expectedIDs []string
	}{
		{
			name:        "empty queue",
			media:       []*domain.Media{},
			expectedIDs: []string{},
		},
		{
			name: "single item",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
			},
			expectedIDs: []string{"1"},
		},
		{
			name: "multiple items preserves order",
			media: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			expectedIDs: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewQueue()
			for _, m := range tt.media {
				q.Add(m)
			}

			items := q.GetAll()

			if len(items) != len(tt.expectedIDs) {
				t.Errorf("got %d items, want %d", len(items), len(tt.expectedIDs))
			}

			for i, expectedID := range tt.expectedIDs {
				if items[i].ID != expectedID {
					t.Errorf("item[%d].ID = %s, want %s", i, items[i].ID, expectedID)
				}
			}
		})
	}
}

func TestGetAll_ReturnsCopy(t *testing.T) {
	q := queue.NewQueue()
	q.Add(&domain.Media{ID: "1", Title: "Media 1"})

	items := q.GetAll()
	items[0] = &domain.Media{ID: "modified", Title: "Modified"}

	originalItems := q.GetAll()
	if originalItems[0].ID != "1" {
		t.Error("GetAll should return a copy, not the original slice")
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name         string
		initialMedia []*domain.Media
		getID        string
		expectError  bool
		expectedID   string
	}{
		{
			name: "get existing item",
			initialMedia: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
				{ID: "3", Title: "Media 3"},
			},
			getID:       "2",
			expectError: false,
			expectedID:  "2",
		},
		{
			name: "get first item",
			initialMedia: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
			},
			getID:       "1",
			expectError: false,
			expectedID:  "1",
		},
		{
			name: "get last item",
			initialMedia: []*domain.Media{
				{ID: "1", Title: "Media 1"},
				{ID: "2", Title: "Media 2"},
			},
			getID:       "2",
			expectError: false,
			expectedID:  "2",
		},
		{
			name: "get non-existent item",
			initialMedia: []*domain.Media{
				{ID: "1", Title: "Media 1"},
			},
			getID:       "999",
			expectError: true,
			expectedID:  "",
		},
		{
			name:         "get from empty queue",
			initialMedia: []*domain.Media{},
			getID:        "1",
			expectError:  true,
			expectedID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queue.NewQueue()
			for _, m := range tt.initialMedia {
				q.Add(m)
			}

			item, err := q.Get(tt.getID)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && item.ID != tt.expectedID {
				t.Errorf("got ID %s, want %s", item.ID, tt.expectedID)
			}
		})
	}
}
