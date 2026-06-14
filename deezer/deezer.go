// Package deezer is the library behind the deezer command line:
// the HTTP client, request shaping, and the typed data models for the
// Deezer public music API (https://api.deezer.com/).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to Deezer. A real, honest
// User-Agent is both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "deezer-cli/dev (+https://github.com/tamnd/deezer-cli)"

// Host is the site this client talks to.
const Host = "deezer.com"

// baseURL is the root every request is built from.
const baseURL = "https://api.deezer.com"

// defaultTimeout is the HTTP request timeout.
const defaultTimeout = 15 * time.Second

// defaultRate is the minimum gap between requests (~50 req/s).
const defaultRate = 20 * time.Millisecond

// maxRetries is the number of retries on transient errors.
const maxRetries = 3

// Client talks to the Deezer API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	baseURL string
	mu      sync.Mutex
	last    time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: defaultTimeout},
		UserAgent: DefaultUserAgent,
		Rate:      defaultRate,
		Retries:   maxRetries,
		baseURL:   baseURL,
	}
}

// get fetches the given path with optional query parameters and returns the body bytes.
func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", path, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- Public types ---

// Track is a Deezer track (song).
type Track struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album,omitempty"`
	Duration int    `json:"duration_seconds,omitempty"`
	Preview  string `json:"preview_url,omitempty"`
	Rank     int    `json:"rank,omitempty"`
}

// Artist is a Deezer artist.
type Artist struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Albums     int     `json:"nb_albums,omitempty"`
	Fans       int     `json:"nb_fans,omitempty"`
	PictureURL string  `json:"picture_url,omitempty"`
	TopTracks  []Track `json:"top_tracks,omitempty"`
}

// Album is a Deezer album.
type Album struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Tracks      int      `json:"nb_tracks,omitempty"`
	ReleaseDate string   `json:"release_date,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	TrackList   []Track  `json:"track_list,omitempty"`
	CoverURL    string   `json:"cover_url,omitempty"`
}

// ChartSection holds the current chart data.
type ChartSection struct {
	Tracks  []Track  `json:"tracks,omitempty"`
	Albums  []Album  `json:"albums,omitempty"`
	Artists []Artist `json:"artists,omitempty"`
}

// --- Wire types (unexported) ---

type wireTrack struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		Title string `json:"title"`
	} `json:"album"`
	Duration int    `json:"duration"`
	Preview  string `json:"preview"`
	Rank     int    `json:"rank"`
}

type wireArtist struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	NbAlbum       int    `json:"nb_album"`
	NbFan         int    `json:"nb_fan"`
	PictureMedium string `json:"picture_medium"`
	Tracklist     string `json:"tracklist"`
}

type wireAlbum struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	NbTracks    int    `json:"nb_tracks"`
	ReleaseDate string `json:"release_date"`
	Genres      struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"genres"`
	Tracks struct {
		Data []wireTrack `json:"data"`
	} `json:"tracks"`
	CoverMedium string `json:"cover_medium"`
}

type wireListResp struct {
	Data  []json.RawMessage `json:"data"`
	Total int               `json:"total"`
}

type wireChartResp struct {
	Tracks  struct{ Data []wireTrack  `json:"data"` } `json:"tracks"`
	Albums  struct{ Data []wireAlbum  `json:"data"` } `json:"albums"`
	Artists struct{ Data []wireArtist `json:"data"` } `json:"artists"`
}

// --- Conversion helpers ---

func toTrack(w wireTrack) Track {
	return Track{
		ID:       w.ID,
		Title:    w.Title,
		Artist:   w.Artist.Name,
		Album:    w.Album.Title,
		Duration: w.Duration,
		Preview:  w.Preview,
		Rank:     w.Rank,
	}
}

func toArtist(w wireArtist) Artist {
	return Artist{
		ID:         w.ID,
		Name:       w.Name,
		Albums:     w.NbAlbum,
		Fans:       w.NbFan,
		PictureURL: w.PictureMedium,
	}
}

func toAlbum(w wireAlbum) Album {
	genres := make([]string, 0, len(w.Genres.Data))
	for _, g := range w.Genres.Data {
		genres = append(genres, g.Name)
	}
	tracks := make([]Track, 0, len(w.Tracks.Data))
	for _, t := range w.Tracks.Data {
		tracks = append(tracks, toTrack(t))
	}
	return Album{
		ID:          w.ID,
		Title:       w.Title,
		Artist:      w.Artist.Name,
		Tracks:      w.NbTracks,
		ReleaseDate: w.ReleaseDate,
		Genres:      genres,
		TrackList:   tracks,
		CoverURL:    w.CoverMedium,
	}
}

// --- API methods ---

// SearchTracks searches for tracks matching query.
func (c *Client) SearchTracks(ctx context.Context, query string, limit int) ([]Track, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.get(ctx, "/search", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []wireTrack `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	out := make([]Track, len(resp.Data))
	for i, w := range resp.Data {
		out[i] = toTrack(w)
	}
	return out, nil
}

// GetArtist fetches artist info and their top tracks.
func (c *Client) GetArtist(ctx context.Context, id int64) (*Artist, error) {
	body, err := c.get(ctx, fmt.Sprintf("/artist/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wa wireArtist
	if err := json.Unmarshal(body, &wa); err != nil {
		return nil, fmt.Errorf("decode artist: %w", err)
	}
	a := toArtist(wa)

	// Fetch top tracks.
	topBody, err := c.get(ctx, fmt.Sprintf("/artist/%d/top", id),
		url.Values{"limit": {"10"}})
	if err != nil {
		return nil, err
	}
	var topResp struct {
		Data []wireTrack `json:"data"`
	}
	if err := json.Unmarshal(topBody, &topResp); err != nil {
		return nil, fmt.Errorf("decode artist top: %w", err)
	}
	a.TopTracks = make([]Track, len(topResp.Data))
	for i, w := range topResp.Data {
		a.TopTracks[i] = toTrack(w)
	}
	return &a, nil
}

// GetAlbum fetches album details with track list.
func (c *Client) GetAlbum(ctx context.Context, id int64) (*Album, error) {
	body, err := c.get(ctx, fmt.Sprintf("/album/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wa wireAlbum
	if err := json.Unmarshal(body, &wa); err != nil {
		return nil, fmt.Errorf("decode album: %w", err)
	}
	a := toAlbum(wa)
	return &a, nil
}

// GetTrack fetches a single track by ID.
func (c *Client) GetTrack(ctx context.Context, id int64) (*Track, error) {
	body, err := c.get(ctx, fmt.Sprintf("/track/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var wt wireTrack
	if err := json.Unmarshal(body, &wt); err != nil {
		return nil, fmt.Errorf("decode track: %w", err)
	}
	t := toTrack(wt)
	return &t, nil
}

// GetChart fetches the current global chart.
func (c *Client) GetChart(ctx context.Context) (*ChartSection, error) {
	body, err := c.get(ctx, "/chart", nil)
	if err != nil {
		return nil, err
	}
	var wc wireChartResp
	if err := json.Unmarshal(body, &wc); err != nil {
		return nil, fmt.Errorf("decode chart: %w", err)
	}
	cs := &ChartSection{}
	cs.Tracks = make([]Track, len(wc.Tracks.Data))
	for i, w := range wc.Tracks.Data {
		cs.Tracks[i] = toTrack(w)
	}
	cs.Albums = make([]Album, len(wc.Albums.Data))
	for i, w := range wc.Albums.Data {
		cs.Albums[i] = toAlbum(w)
	}
	cs.Artists = make([]Artist, len(wc.Artists.Data))
	for i, w := range wc.Artists.Data {
		cs.Artists[i] = toArtist(w)
	}
	return cs, nil
}

// SearchAlbums searches for albums matching query.
func (c *Client) SearchAlbums(ctx context.Context, query string, limit int) ([]Album, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.get(ctx, "/search/album", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []wireAlbum `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search/album: %w", err)
	}
	out := make([]Album, len(resp.Data))
	for i, w := range resp.Data {
		out[i] = toAlbum(w)
	}
	return out, nil
}

// SearchArtists searches for artists matching query.
func (c *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.get(ctx, "/search/artist", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []wireArtist `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search/artist: %w", err)
	}
	out := make([]Artist, len(resp.Data))
	for i, w := range resp.Data {
		out[i] = toArtist(w)
	}
	return out, nil
}
