package cli

import (
	"testing"
)

func TestFlagSet(t *testing.T) {
	s := FlagSet{"verbose": true, "force": false, "quiet": true}
	
	t.Run("Has", func(t *testing.T) {
		if !s.Has("verbose") { t.Error("expected true") }
		if !s.Has("force") { t.Error("expected true") }
		if s.Has("missing") { t.Error("expected false") }
	})

	t.Run("Get", func(t *testing.T) {
		if !s.Get("verbose") { t.Error("expected true") }
		if s.Get("force") { t.Error("expected false") }
		if s.Get("missing") { t.Error("expected false") }
	})

	t.Run("String", func(t *testing.T) {
		got := s.String()
		want := "--quiet --verbose"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestOptionSet(t *testing.T) {
	s := OptionSet{"host": {"localhost"}, "user": {"admin"}}

	t.Run("Has", func(t *testing.T) {
		if !s.Has("host") { t.Error("expected true") }
		if s.Has("missing") { t.Error("expected false") }
	})

	t.Run("Get", func(t *testing.T) {
		if s.Get("host") != "localhost" { t.Errorf("got %q", s.Get("host")) }
		if s.Get("missing") != "" { t.Errorf("got %q", s.Get("missing")) }
	})

	t.Run("GetReturnsLast", func(t *testing.T) {
		multi := OptionSet{"tag": {"a", "b", "c"}}
		if got := multi.Get("tag"); got != "c" {
			t.Errorf("got %q, want last value", got)
		}
	})

	t.Run("Values", func(t *testing.T) {
		multi := OptionSet{"tag": {"a", "b"}}
		got := multi.Values("tag")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v", got)
		}
		if multi.Values("missing") != nil {
			t.Error("expected nil for missing key")
		}
	})

	t.Run("Add", func(t *testing.T) {
		s := OptionSet{}
		s.Add("tag", "a")
		s.Add("tag", "b")
		got := s.Values("tag")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("String", func(t *testing.T) {
		got := s.String()
		want := "--host localhost --user admin"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestArgSet(t *testing.T) {
	s := ArgSet{"path": "/tmp", "name": "test"}

	t.Run("Has", func(t *testing.T) {
		if !s.Has("path") { t.Error("expected true") }
		if s.Has("missing") { t.Error("expected false") }
	})

	t.Run("Get", func(t *testing.T) {
		if s.Get("path") != "/tmp" { t.Errorf("got %q", s.Get("path")) }
		if s.Get("missing") != "" { t.Errorf("got %q", s.Get("missing")) }
	})

	t.Run("String", func(t *testing.T) {
		got := s.String()
		// Alphabetical order because it's a map
		want := "<name> test <path> /tmp"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
