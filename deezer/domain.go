package deezer

import (
	"context"
	"fmt"

	"github.com/tamnd/any-cli/kit"
)

// domain.go exposes deezer as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/deezer-cli/deezer"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then routes
// deezer:// URIs to the operations Register installs. The same Domain also
// builds the standalone deezer binary (see cmd/deezer/main.go).
func init() { kit.Register(Domain{}) }

// Domain is the deezer driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "deezer",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "deezer",
			Short:  "A command line for the Deezer music API.",
			Long: `A command line for the Deezer public music API.

deezer reads public Deezer data over plain HTTPS with no API key required,
shapes it into clean records, and prints output that pipes into the rest of
your tools.`,
			Site: Host,
			Repo: "https://github.com/tamnd/deezer-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "tracks",
		Summary: "Search for tracks by query",
	}, searchTracks)

	kit.Handle(app, kit.OpMeta{
		Name:    "artist",
		Group:   "library",
		Summary: "Get artist info and top tracks",
	}, getArtist)

	kit.Handle(app, kit.OpMeta{
		Name:    "album",
		Group:   "library",
		Summary: "Get album details with track list",
	}, getAlbum)

	kit.Handle(app, kit.OpMeta{
		Name:    "track",
		Group:   "library",
		Summary: "Get a single track by ID",
	}, getTrack)

	kit.Handle(app, kit.OpMeta{
		Name:    "chart",
		Group:   "charts",
		Summary: "Get current top tracks, albums, and artists",
	}, getChart)

	kit.Handle(app, kit.OpMeta{
		Name:    "search-albums",
		Group:   "search",
		Summary: "Search for albums by query",
	}, searchAlbums)

	kit.Handle(app, kit.OpMeta{
		Name:    "search-artists",
		Group:   "search",
		Summary: "Search for artists by query",
	}, searchArtists)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type searchIn struct {
	Query  string  `kit:"flag" short:"q" help:"search query (required)"`
	Limit  int     `kit:"flag,inherit" help:"max results (default 10)"`
	Client *Client `kit:"inject"`
}

type artistIn struct {
	ID     int64   `kit:"flag" help:"artist ID"`
	Client *Client `kit:"inject"`
}

type albumIn struct {
	ID     int64   `kit:"flag" help:"album ID"`
	Client *Client `kit:"inject"`
}

type trackIn struct {
	ID     int64   `kit:"flag" help:"track ID"`
	Client *Client `kit:"inject"`
}

type chartIn struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchTracks(ctx context.Context, in searchIn, emit func(*Track) error) error {
	if in.Query == "" {
		return fmt.Errorf("--query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	tracks, err := in.Client.SearchTracks(ctx, in.Query, limit)
	if err != nil {
		return err
	}
	for i := range tracks {
		if err := emit(&tracks[i]); err != nil {
			return err
		}
	}
	return nil
}

func getArtist(ctx context.Context, in artistIn, emit func(*Artist) error) error {
	if in.ID == 0 {
		return fmt.Errorf("--id is required")
	}
	a, err := in.Client.GetArtist(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(a)
}

func getAlbum(ctx context.Context, in albumIn, emit func(*Album) error) error {
	if in.ID == 0 {
		return fmt.Errorf("--id is required")
	}
	a, err := in.Client.GetAlbum(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(a)
}

func getTrack(ctx context.Context, in trackIn, emit func(*Track) error) error {
	if in.ID == 0 {
		return fmt.Errorf("--id is required")
	}
	t, err := in.Client.GetTrack(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(t)
}

func getChart(ctx context.Context, in chartIn, emit func(*ChartSection) error) error {
	cs, err := in.Client.GetChart(ctx)
	if err != nil {
		return err
	}
	return emit(cs)
}

func searchAlbums(ctx context.Context, in searchIn, emit func(*Album) error) error {
	if in.Query == "" {
		return fmt.Errorf("--query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	albums, err := in.Client.SearchAlbums(ctx, in.Query, limit)
	if err != nil {
		return err
	}
	for i := range albums {
		if err := emit(&albums[i]); err != nil {
			return err
		}
	}
	return nil
}

func searchArtists(ctx context.Context, in searchIn, emit func(*Artist) error) error {
	if in.Query == "" {
		return fmt.Errorf("--query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	artists, err := in.Client.SearchArtists(ctx, in.Query, limit)
	if err != nil {
		return err
	}
	for i := range artists {
		if err := emit(&artists[i]); err != nil {
			return err
		}
	}
	return nil
}

// Classify and Locate satisfy the kit.Domain interface for URI routing.
func (Domain) Classify(input string) (uriType, id string, err error) {
	return "track", input, nil
}

func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "track":
		return baseURL + "/track/" + id, nil
	case "artist":
		return baseURL + "/artist/" + id, nil
	case "album":
		return baseURL + "/album/" + id, nil
	}
	return "", fmt.Errorf("deezer: unknown resource type %q", uriType)
}
