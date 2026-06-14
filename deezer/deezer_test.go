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

func makeTrackJSON(id int64, title, artist, album string) map[string]any {
	return map[string]any{
		"id":       id,
		"title":    title,
		"duration": 240,
		"rank":     500000,
		"preview":  "https://cdn.deezer.com/preview/test.mp3",
		"artist":   map[string]any{"id": 27, "name": artist},
		"album":    map[string]any{"id": 302127, "title": album},
	}
}

func makeAlbumJSON(id int64, title, artist string) map[string]any {
	return map[string]any{
		"id":           id,
		"title":        title,
		"nb_tracks":    14,
		"release_date": "2001-02-26",
		"cover_medium": "https://e-cdns-images.dzcdn.net/cover/disc.jpg",
		"artist":       map[string]any{"id": 27, "name": artist},
		"genres": map[string]any{
			"data": []map[string]any{{"id": 129, "name": "Electronic"}},
		},
		"tracks": map[string]any{
			"data": []any{makeTrackJSON(1, "One More Time", artist, title)},
		},
	}
}

func makeArtistJSON(id int64, name string) map[string]any {
	return map[string]any{
		"id":             id,
		"name":           name,
		"nb_album":       31,
		"nb_fan":         7019308,
		"picture_medium": "https://e-cdns-images.dzcdn.net/artist/pic.jpg",
		"tracklist":      "https://api.deezer.com/artist/27/top?limit=50",
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

// TestSearch verifies SearchTracks parses a search response correctly.
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
	tracks, err := c.SearchTracks(context.Background(), "daft punk", 10)
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
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
}

// TestArtist verifies GetArtist fetches both artist info and top tracks.
func TestArtist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/artist/27":
			writeJSON(w, makeArtistJSON(27, "Daft Punk"))
		case r.URL.Path == "/artist/27/top":
			writeJSON(w, map[string]any{
				"data": []any{
					makeTrackJSON(3135556, "Harder, Better, Faster, Stronger", "Daft Punk", "Discovery"),
					makeTrackJSON(64868958, "Get Lucky", "Daft Punk", "Random Access Memories"),
				},
			})
		default:
			http.NotFound(w, r)
		}
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
	if len(a.TopTracks) != 2 {
		t.Fatalf("len(TopTracks) = %d, want 2", len(a.TopTracks))
	}
	if a.TopTracks[0].Title != "Harder, Better, Faster, Stronger" {
		t.Errorf("TopTracks[0].Title = %q", a.TopTracks[0].Title)
	}
	if a.TopTracks[1].Title != "Get Lucky" {
		t.Errorf("TopTracks[1].Title = %q", a.TopTracks[1].Title)
	}
}

// TestAlbum verifies GetAlbum parses album with genres and track list.
func TestAlbum(t *testing.T) {
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
	if a.ReleaseDate != "2001-02-26" {
		t.Errorf("ReleaseDate = %q", a.ReleaseDate)
	}
	if len(a.Genres) != 1 || a.Genres[0] != "Electronic" {
		t.Errorf("Genres = %v, want [Electronic]", a.Genres)
	}
	if len(a.TrackList) != 1 || a.TrackList[0].Title != "One More Time" {
		t.Errorf("TrackList = %v", a.TrackList)
	}
	if a.CoverURL == "" {
		t.Error("CoverURL is empty")
	}
}

// TestTrack verifies GetTrack fetches a single track.
func TestTrack(t *testing.T) {
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
	if tr.Preview == "" {
		t.Error("Preview is empty")
	}
}

// TestChart verifies GetChart parses all three chart sections.
func TestChart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chart" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"tracks": map[string]any{
				"data": []any{makeTrackJSON(1, "Top Track", "Artist A", "Album A")},
			},
			"albums": map[string]any{
				"data": []any{makeAlbumJSON(100, "Top Album", "Artist B")},
			},
			"artists": map[string]any{
				"data": []any{makeArtistJSON(200, "Top Artist")},
			},
			"playlists": map[string]any{
				"data": []any{},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	cs, err := c.GetChart(context.Background())
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	if len(cs.Tracks) != 1 {
		t.Errorf("len(Tracks) = %d, want 1", len(cs.Tracks))
	}
	if len(cs.Albums) != 1 {
		t.Errorf("len(Albums) = %d, want 1", len(cs.Albums))
	}
	if len(cs.Artists) != 1 {
		t.Errorf("len(Artists) = %d, want 1", len(cs.Artists))
	}
	if cs.Tracks[0].Title != "Top Track" {
		t.Errorf("Tracks[0].Title = %q", cs.Tracks[0].Title)
	}
	if cs.Albums[0].Title != "Top Album" {
		t.Errorf("Albums[0].Title = %q", cs.Albums[0].Title)
	}
	if cs.Artists[0].Name != "Top Artist" {
		t.Errorf("Artists[0].Name = %q", cs.Artists[0].Name)
	}
}

// TestSearchAlbums verifies SearchAlbums parses album list correctly.
func TestSearchAlbums(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/album" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"data":  []any{makeAlbumJSON(302127, "Discovery", "Daft Punk")},
			"total": 1,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	albums, err := c.SearchAlbums(context.Background(), "discovery", 10)
	if err != nil {
		t.Fatalf("SearchAlbums: %v", err)
	}
	if len(albums) != 1 {
		t.Fatalf("len(albums) = %d, want 1", len(albums))
	}
	if albums[0].Title != "Discovery" {
		t.Errorf("Title = %q, want Discovery", albums[0].Title)
	}
	if albums[0].Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", albums[0].Artist)
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
