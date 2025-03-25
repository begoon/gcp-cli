package main

import (
	"reflect"
	"testing"
)

func TestHistoryPriority(t *testing.T) {
	for _, tt := range []struct {
		history  []string
		lines    []string
		expected []string
	}{
		{
			history:  []string{"apple", "banana", "cherry"},
			lines:    []string{"date", "banana", "grape", "apple", "kiwi", "cherry"},
			expected: []string{"apple", "banana", "cherry", "date", "grape", "kiwi"},
		},
		{
			history:  []string{"one", "two", "three", "four"},
			lines:    []string{"five", "one", "six", "three", "seven"},
			expected: []string{"one", "three", "five", "six", "seven"},
		},
		{
			history:  []string{"a", "b", "c"},
			lines:    []string{"x", "y", "z"},
			expected: []string{"x", "y", "z"},
		},
		{
			history:  []string{"a", "b", "c"},
			lines:    []string{"c", "b", "a"},
			expected: []string{"a", "b", "c"},
		},
	} {
		t.Run("", func(t *testing.T) {
			actual := historyPriority(tt.history, tt.lines)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("got %v, want %v", actual, tt.expected)
			}
		})
	}
}
