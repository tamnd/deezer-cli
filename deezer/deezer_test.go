package deezer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient returns a Client pointed at srv.URL with no rate limiting.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.baseURL = srv.URL
	c.Rate = 0
	c.Retries = 0
	return c
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func makeTrackJSON(id int, title, artist, album string) map[string]any {
	return map[string]any{
		"id":       id,
		"title":    title,
		"duration": 240,
		"rank":     500000,
		"artist":   map[string]any{"id": 27, "name": artist},
		"album":    map[string]any{"id": 302127, "title": album},
	}
}

func makeAlbumJSON(id int, title, artist string) map[string]any {
	return map[string]any{
		"id":           id,
		"title":        title,
		"nb_tracks":    14,
		"duration":     3600,
		"release_date": "2001-02-26",
		"label":        "Virgin",
		"fans":         500000,
		"artist":       map[string]any{"id": 27, "name": artist},
		"genres": map[string]any{
			"data": []map[string]any{{"id": 129, "name": "Electronic"}},
		},
	}
}

func makeArtistJSON(id int, name string) map[string]any {
	return map[string]any{
		"id":      id,
		"name":    name,
		"nb_album": 31,
		"nb_fan":  7019308,
		"link":    "https://www.deezer.com/artist/27",
	}
}

// TestGet verifies the base HTTP client sends a User-Agent and reads body.
func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL
	c.Rate = 0

	body, err := c.get(context.Background(), "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body = %q, want ok", body)
	}
}

// TestGetRetriesOn503 verifies the client retries on 5xx.
func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	_, err := c.get(context.Background(), "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

// TestSearch verifies Search parses a search response correctly.
func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "daft") {
			http.Error(w, "bad query", 400)
			return
		}
		writeJSON(w, map[string]any{
			"data":  []any{makeTrackJSON(3135556, "Harder, Better, Faster, Stronger", "Daft Punk", "Discovery")},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	tracks, err := c.Search(context.Background(), "daft punk", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	got := tracks[0]
	if got.ID != 3135556 {
		t.Errorf("ID = %d, want 3135556", got.ID)
	}
	if got.Title != "Harder, Better, Faster, Stronger" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", got.Artist)
	}
	if got.Album != "Discovery" {
		t.Errorf("Album = %q, want Discovery", got.Album)
	}
	if got.Duration != 240 {
		t.Errorf("Duration = %d, want 240", got.Duration)
	}
	if got.Rank != 500000 {
		t.Errorf("Rank = %d, want 500000", got.Rank)
	}
}

// TestGetArtist verifies GetArtist fetches artist info.
func TestGetArtist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artist/27" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, makeArtistJSON(27, "Daft Punk"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	a, err := c.GetArtist(context.Background(), 27)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if a.ID != 27 {
		t.Errorf("ID = %d, want 27", a.ID)
	}
	if a.Name != "Daft Punk" {
		t.Errorf("Name = %q, want Daft Punk", a.Name)
	}
	if a.Albums != 31 {
		t.Errorf("Albums = %d, want 31", a.Albums)
	}
	if a.Fans != 7019308 {
		t.Errorf("Fans = %d, want 7019308", a.Fans)
	}
	if a.URL == "" {
		t.Error("URL is empty")
	}
}

// TestSearchArtists verifies SearchArtists parses artist list correctly.
func TestSearchArtists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/artist" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"data":  []any{makeArtistJSON(27, "Daft Punk")},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	artists, err := c.SearchArtists(context.Background(), "daft punk", 10)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	if artists[0].Name != "Daft Punk" {
		t.Errorf("Name = %q, want Daft Punk", artists[0].Name)
	}
	if artists[0].Fans != 7019308 {
		t.Errorf("Fans = %d, want 7019308", artists[0].Fans)
	}
}

// TestGetAlbum verifies GetAlbum parses album with genres.
func TestGetAlbum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/album/302127" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, makeAlbumJSON(302127, "Discovery", "Daft Punk"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	a, err := c.GetAlbum(context.Background(), 302127)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if a.ID != 302127 {
		t.Errorf("ID = %d, want 302127", a.ID)
	}
	if a.Title != "Discovery" {
		t.Errorf("Title = %q, want Discovery", a.Title)
	}
	if a.Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", a.Artist)
	}
	if a.Tracks != 14 {
		t.Errorf("Tracks = %d, want 14", a.Tracks)
	}
	if a.Duration != 3600 {
		t.Errorf("Duration = %d, want 3600", a.Duration)
	}
	if a.Released != "2001-02-26" {
		t.Errorf("Released = %q", a.Released)
	}
	if a.Label != "Virgin" {
		t.Errorf("Label = %q, want Virgin", a.Label)
	}
	if a.Fans != 500000 {
		t.Errorf("Fans = %d, want 500000", a.Fans)
	}
	if a.Genres != "Electronic" {
		t.Errorf("Genres = %q, want Electronic", a.Genres)
	}
}

// TestGetTrack verifies GetTrack fetches a single track.
func TestGetTrack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/3135556" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, makeTrackJSON(3135556, "Harder, Better, Faster, Stronger", "Daft Punk", "Discovery"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	tr, err := c.GetTrack(context.Background(), 3135556)
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if tr.ID != 3135556 {
		t.Errorf("ID = %d, want 3135556", tr.ID)
	}
	if tr.Title != "Harder, Better, Faster, Stronger" {
		t.Errorf("Title = %q", tr.Title)
	}
	if tr.Duration != 240 {
		t.Errorf("Duration = %d, want 240", tr.Duration)
	}
}

// TestHTTP404 verifies the client returns an error on 404.
func TestHTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetTrack(context.Background(), 9999)
	if err == nil {
		t.Error("expected error on 404, got nil")
	}
}
