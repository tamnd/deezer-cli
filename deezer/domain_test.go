package deezer

import (
	"testing"
)

// These tests exercise the conversion helpers and the URI driver's pure string
// functions, which need no network. HTTP behaviour is covered in deezer_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "deezer" {
		t.Errorf("Scheme = %q, want deezer", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "deezer" {
		t.Errorf("Identity.Binary = %q, want deezer", info.Identity.Binary)
	}
}

func TestToTrack(t *testing.T) {
	w := wireTrack{
		ID:       3135556,
		Title:    "Harder, Better, Faster, Stronger",
		Duration: 224,
		Rank:     999901,
	}
	w.Artist.Name = "Daft Punk"
	w.Album.Title = "Discovery"

	got := toTrack(w)
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
	if got.Duration != 224 {
		t.Errorf("Duration = %d, want 224", got.Duration)
	}
	if got.Rank != 999901 {
		t.Errorf("Rank = %d, want 999901", got.Rank)
	}
}

func TestToArtist(t *testing.T) {
	w := wireArtist{
		ID:      27,
		Name:    "Daft Punk",
		NbAlbum: 31,
		NbFan:   7019308,
		Link:    "https://www.deezer.com/artist/27",
	}

	got := toArtist(w)
	if got.ID != 27 {
		t.Errorf("ID = %d, want 27", got.ID)
	}
	if got.Name != "Daft Punk" {
		t.Errorf("Name = %q, want Daft Punk", got.Name)
	}
	if got.Albums != 31 {
		t.Errorf("Albums = %d, want 31", got.Albums)
	}
	if got.Fans != 7019308 {
		t.Errorf("Fans = %d, want 7019308", got.Fans)
	}
	if got.URL != "https://www.deezer.com/artist/27" {
		t.Errorf("URL = %q", got.URL)
	}
}

func TestToAlbum(t *testing.T) {
	w := wireAlbum{
		ID:          302127,
		Title:       "Discovery",
		NbTracks:    14,
		Duration:    3600,
		ReleaseDate: "2001-02-26",
		Label:       "Virgin",
		Fans:        500000,
	}
	w.Artist.Name = "Daft Punk"
	w.Genres.Data = []struct {
		Name string `json:"name"`
	}{
		{Name: "Electronic"},
		{Name: "Dance"},
	}

	got := toAlbum(w)
	if got.ID != 302127 {
		t.Errorf("ID = %d, want 302127", got.ID)
	}
	if got.Title != "Discovery" {
		t.Errorf("Title = %q, want Discovery", got.Title)
	}
	if got.Artist != "Daft Punk" {
		t.Errorf("Artist = %q, want Daft Punk", got.Artist)
	}
	if got.Tracks != 14 {
		t.Errorf("Tracks = %d, want 14", got.Tracks)
	}
	if got.Duration != 3600 {
		t.Errorf("Duration = %d, want 3600", got.Duration)
	}
	if got.Released != "2001-02-26" {
		t.Errorf("Released = %q", got.Released)
	}
	if got.Label != "Virgin" {
		t.Errorf("Label = %q, want Virgin", got.Label)
	}
	if got.Fans != 500000 {
		t.Errorf("Fans = %d, want 500000", got.Fans)
	}
	if got.Genres != "Electronic, Dance" {
		t.Errorf("Genres = %q, want \"Electronic, Dance\"", got.Genres)
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType string
		id      string
		want    string
	}{
		{"track", "3135556", baseURL + "/track/3135556"},
		{"artist", "27", baseURL + "/artist/27"},
		{"album", "302127", baseURL + "/album/302127"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("playlist", "123")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
