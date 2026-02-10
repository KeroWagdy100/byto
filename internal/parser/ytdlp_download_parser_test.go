package parser_test

import (
	"byto/internal/parser"
	"testing"
)

func TestYTDLPDownloadParser_Parse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult map[string]string
		expectError    bool
	}{
		// Valid inputs - all numeric values
		{
			name:  "valid input with all numeric values",
			input: "[byto] My Video Title [downloaded] 1024 [total] 2048 [frag] 1 [frags] 10",
			expectedResult: map[string]string{
				"title":            "My Video Title",
				"downloaded_bytes": "1024",
				"total_bytes":      "2048",
				"fragment_index":   "1",
				"fragment_count":   "10",
			},
			expectError: false,
		},
		{
			name:  "valid input with large numbers",
			input: "[byto] Large File [downloaded] 999999999999 [total] 1000000000000 [frag] 500 [frags] 1000",
			expectedResult: map[string]string{
				"title":            "Large File",
				"downloaded_bytes": "999999999999",
				"total_bytes":      "1000000000000",
				"fragment_index":   "500",
				"fragment_count":   "1000",
			},
			expectError: false,
		},
		{
			name:  "valid input with zero values",
			input: "[byto] Starting Download [downloaded] 0 [total] 5000 [frag] 0 [frags] 5",
			expectedResult: map[string]string{
				"title":            "Starting Download",
				"downloaded_bytes": "0",
				"total_bytes":      "5000",
				"fragment_index":   "0",
				"fragment_count":   "5",
			},
			expectError: false,
		},

		// Valid inputs - with NA values
		{
			name:  "valid input with all NA values",
			input: "[byto] Unknown Video [downloaded] NA [total] NA [frag] NA [frags] NA",
			expectedResult: map[string]string{
				"title":            "Unknown Video",
				"downloaded_bytes": "NA",
				"total_bytes":      "NA",
				"fragment_index":   "NA",
				"fragment_count":   "NA",
			},
			expectError: false,
		},
		{
			name:  "valid input with NA total bytes",
			input: "[byto] Streaming Content [downloaded] 2048 [total] NA [frag] 2 [frags] NA",
			expectedResult: map[string]string{
				"title":            "Streaming Content",
				"downloaded_bytes": "2048",
				"total_bytes":      "NA",
				"fragment_index":   "2",
				"fragment_count":   "NA",
			},
			expectError: false,
		},
		{
			name:  "valid input with NA downloaded bytes",
			input: "[byto] Starting Content [downloaded] NA [total] 10000 [frag] 1 [frags] 20",
			expectedResult: map[string]string{
				"title":            "Starting Content",
				"downloaded_bytes": "NA",
				"total_bytes":      "10000",
				"fragment_index":   "1",
				"fragment_count":   "20",
			},
			expectError: false,
		},

		// Valid inputs - title edge cases
		{
			name:  "title with special characters",
			input: "[byto] Video: Test (2024) - Part 1 #Official [downloaded] 1000 [total] 2000 [frag] 1 [frags] 5",
			expectedResult: map[string]string{
				"title":            "Video: Test (2024) - Part 1 #Official",
				"downloaded_bytes": "1000",
				"total_bytes":      "2000",
				"fragment_index":   "1",
				"fragment_count":   "5",
			},
			expectError: false,
		},
		{
			name:  "title with unicode characters",
			input: "[byto] 日本語タイトル - Café ñ [downloaded] 500 [total] 1000 [frag] 1 [frags] 2",
			expectedResult: map[string]string{
				"title":            "日本語タイトル - Café ñ",
				"downloaded_bytes": "500",
				"total_bytes":      "1000",
				"fragment_index":   "1",
				"fragment_count":   "2",
			},
			expectError: false,
		},
		{
			name:  "title with numbers",
			input: "[byto] 123 Video 456 [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "123 Video 456",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "title with multiple spaces",
			input: "[byto] Video   With   Extra   Spaces [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "Video   With   Extra   Spaces",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "very long title",
			input: "[byto] This Is A Very Long Video Title That Goes On And On And On And Contains Many Words To Test The Parser With Long Strings [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "This Is A Very Long Video Title That Goes On And On And On And Contains Many Words To Test The Parser With Long Strings",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "single word title",
			input: "[byto] Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "Video",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},

		// Valid inputs - whitespace handling
		{
			name:  "input with leading whitespace",
			input: "   [byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "Test Video",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "input with trailing whitespace",
			input: "[byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1   ",
			expectedResult: map[string]string{
				"title":            "Test Video",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "input with leading and trailing whitespace",
			input: "   [byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1   ",
			expectedResult: map[string]string{
				"title":            "Test Video",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
		{
			name:  "input with extra spaces between markers",
			input: "[byto]   Test Video   [downloaded]   100   [total]   200   [frag]   1   [frags]   1",
			expectedResult: map[string]string{
				"title":            "Test Video",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},

		// Invalid inputs - missing markers
		{
			name:           "missing byto marker",
			input:          "Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "missing downloaded marker",
			input:          "[byto] Test Video 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "missing total marker",
			input:          "[byto] Test Video [downloaded] 100 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "missing frag marker",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "missing frags marker",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 [frag] 1 1",
			expectedResult: nil,
			expectError:    true,
		},

		// Invalid inputs - empty or incomplete
		{
			name:           "empty string",
			input:          "",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "whitespace only",
			input:          "   ",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "only byto marker",
			input:          "[byto] Test Video",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "partial format",
			input:          "[byto] Test Video [downloaded] 100 [total] 200",
			expectedResult: nil,
			expectError:    true,
		},

		// Invalid inputs - wrong values
		{
			name:           "non-numeric downloaded bytes",
			input:          "[byto] Test Video [downloaded] abc [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "non-numeric total bytes",
			input:          "[byto] Test Video [downloaded] 100 [total] xyz [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "non-numeric fragment index",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 [frag] abc [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "non-numeric fragment count",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] xyz",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "lowercase na instead of NA",
			input:          "[byto] Test Video [downloaded] na [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "mixed case Na",
			input:          "[byto] Test Video [downloaded] Na [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "negative number",
			input:          "[byto] Test Video [downloaded] -100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "decimal number",
			input:          "[byto] Test Video [downloaded] 100.5 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},

		// Invalid inputs - wrong order
		{
			name:           "markers in wrong order",
			input:          "[downloaded] 100 [byto] Test Video [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "frag and frags swapped",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 [frags] 1 [frag] 1",
			expectedResult: nil,
			expectError:    true,
		},

		// Invalid inputs - malformed markers
		{
			name:           "missing opening bracket",
			input:          "byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "missing closing bracket",
			input:          "[byto Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "typo in marker",
			input:          "[byto] Test Video [donwloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},

		// Invalid inputs - extra content
		{
			name:           "extra content after frags value",
			input:          "[byto] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1 extra",
			expectedResult: nil,
			expectError:    true,
		},

		// Edge case - empty title should still have content between markers
		{
			name:           "content that looks like another log line",
			input:          "[yt-dlp] Test Video [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: nil,
			expectError:    true,
		},

		// Title containing brackets (but not the marker brackets)
		{
			name:  "title containing square brackets",
			input: "[byto] [HD] My Video [1080p] [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedResult: map[string]string{
				"title":            "[HD] My Video [1080p]",
				"downloaded_bytes": "100",
				"total_bytes":      "200",
				"fragment_index":   "1",
				"fragment_count":   "1",
			},
			expectError: false,
		},
	}

	p := parser.YTDLPDownloadParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none; result: %v", result)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result but got nil")
				return
			}

			// Verify all expected fields
			for key, expectedValue := range tt.expectedResult {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("missing key %q in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("key %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Verify no extra fields
			if len(result) != len(tt.expectedResult) {
				t.Errorf("result has %d fields, expected %d", len(result), len(tt.expectedResult))
			}
		})
	}
}

func TestYTDLPDownloadParser_Parse_ResultMapKeys(t *testing.T) {
	p := parser.YTDLPDownloadParser{}
	result, err := p.Parse("[byto] Test [downloaded] 100 [total] 200 [frag] 1 [frags] 2")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedKeys := []string{"title", "downloaded_bytes", "total_bytes", "fragment_index", "fragment_count"}

	for _, key := range expectedKeys {
		if _, exists := result[key]; !exists {
			t.Errorf("result missing expected key: %q", key)
		}
	}

	if len(result) != len(expectedKeys) {
		t.Errorf("result has %d keys, expected %d", len(result), len(expectedKeys))
	}
}

func TestYTDLPDownloadParser_Parse_TitleTrimming(t *testing.T) {
	p := parser.YTDLPDownloadParser{}

	tests := []struct {
		name          string
		input         string
		expectedTitle string
	}{
		{
			name:          "title with leading space after byto marker",
			input:         "[byto]   Trimmed Title   [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedTitle: "Trimmed Title",
		},
		{
			name:          "title with no extra spaces",
			input:         "[byto] Exact Title [downloaded] 100 [total] 200 [frag] 1 [frags] 1",
			expectedTitle: "Exact Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result["title"] != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, result["title"])
			}
		})
	}
}
