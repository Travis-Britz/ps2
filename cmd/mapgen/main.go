package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
	"github.com/Travis-Britz/ps2/psmap"
	"github.com/anthonynsimon/bild/transform"
	"github.com/google/uuid"
)

var config = struct {
	Bind         string
	ServiceID    string
	VerboseLog   bool
	World        ps2.WorldID
	Zone         ps2.ContinentID
	Region       ps2.RegionID
	Env          ps2.Environment
	Loc          psmap.Loc
	DataFile     string
	Output       string
	OutputDir    string
	OutputFormat string
	Mode         mode
}{}

type renderable struct {
	fn        renderingFn
	extension string
	mimetype  string
}

var formats = map[string]renderable{
	"image": {
		RenderMapImageDefaultPNG,
		".png",
		"image/png",
	},
	"transparent": {
		RenderMapImageNoBackgroundPNG,
		".png",
		"image/png",
	},
	"thumbnail": {
		RenderMapImageDiscordThumbnailPNG,
		".png",
		"image/png",
	},
	"json": {
		RenderMapStateJSON,
		".json",
		"application/json",
	},
}

type mode uint8

func (m mode) String() string {
	switch m {
	case SingleFile:
		return "SingleFile"
	case MultiFile:
		return "MultiFile"
	case MapDataFile:
		return "MapDataFile"
	case HTTPServer:
		return "HTTPServer"
	case SingleRegion:
		return "SingleRegion"
	case ZoneLoc:
		return "ZoneLoc"
	case AllRegions:
		return "AllRegions"
	default:
		return fmt.Sprintf("%d", m)
	}
}

const (
	SingleFile mode = iota
	MultiFile
	MapDataFile
	HTTPServer
	SingleRegion
	ZoneLoc
	AllRegions
)

func main() {
	var environment, world, zone, location string
	var datamode bool
	var cropregionmode bool
	flag.StringVar(&config.Bind, "serve", config.Bind, "Serve will start the process as a small HTTP server bound to the given network interface such as \"localhost:8080\".")
	flag.StringVar(&config.ServiceID, "s", config.ServiceID, "Service ID: https://census.daybreakgames.com/#service-id")
	flag.BoolVar(&config.VerboseLog, "v", config.VerboseLog, "Enable writing verbose logging information to stderr.")
	flag.BoolVar(&datamode, "data", false, "Output map data as json")
	flag.StringVar(&environment, "env", "pc", "PlanetSide environment. Note: env is only required for generating json data files. (pc, ps4us, ps4eu)")
	flag.StringVar(&world, "world", "", "The world to check (emerald, soltech, etc.)")
	flag.StringVar(&zone, "zone", "", "The zone to check (indar, hossin, esamir, amerish, oshur)")
	flag.StringVar(&config.OutputDir, "outputdir", ".", "File paths will be appended to this directory")
	flag.StringVar(&config.OutputFormat, "format", "image", "The output format for a map (image, thumbnail, json).")
	flag.IntVar((*int)(&config.Region), "region", 0, "Draw a map region PNG.")
	flag.BoolVar(&cropregionmode, "regions", false, "Generate cropped region and facility images.")
	flag.StringVar(&location, "loc", "", "Location as reported by the /loc command in-game, e.g. -loc \"3211.266 470.785 3136.692\". A fourth value, heading, is optional.")
	// flag.StringVar(&config.DataFile, "datafile", "", "Use a provided map data file to override the embedded map data.")
	flag.Parse()

	config.Output = flag.Arg(0)

	switch environment {
	case "ps4us":
		config.Env = ps2.PS4US
	case "ps4eu":
		config.Env = ps2.PS4EU
	}

	if config.ServiceID != "" {
		census.DefaultClient.ServiceID = config.ServiceID
	}

	config.World = parseWorld(world)
	config.Zone = parseZone(zone)

	locParams := strings.Split(location, " ")
	if len(locParams) >= 3 {
		config.Loc.X, _ = strconv.ParseFloat(locParams[0], 64)
		config.Loc.Y, _ = strconv.ParseFloat(locParams[1], 64)
		config.Loc.Z, _ = strconv.ParseFloat(locParams[2], 64)
	}
	if len(locParams) == 4 {
		config.Loc.Heading, _ = strconv.ParseFloat(locParams[3], 64)
	}

	var logLevel = slog.LevelInfo
	if config.VerboseLog {
		logLevel = slog.LevelDebug
	}
	slog.SetLogLoggerLevel(logLevel)
	baseLogger := slog.New(&contextHandler{
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}),
	})
	slog.SetDefault(baseLogger)
	census.DefaultClient.SetLog(slog.DebugContext)
	switch {
	case config.Bind != "":
		config.Mode = HTTPServer
	case location != "":
		config.Mode = ZoneLoc
	case cropregionmode:
		config.Mode = AllRegions
	case datamode:
		config.Mode = MapDataFile
	case config.Output == "":
		config.Mode = MultiFile
	case config.Region != 0:
		config.Mode = SingleRegion
	default:
		config.Mode = SingleFile
	}

	ctx, shutdown := context.WithCancelCause(context.Background())
	go func() {
		defer slog.Debug("received interrupt")
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		shutdown(errGracefulShutdown)
	}()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			err = context.Cause(ctx)
		}
		if errors.Is(err, errGracefulShutdown) {
			return
		}
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	zones := []ps2.ContinentID{
		ps2.Amerish,
		ps2.Indar,
		ps2.Hossin,
		ps2.Esamir,
		ps2.Oshur,
	}
	for _, zone := range zones {
		f, _ := imagedir.Open(fmt.Sprintf("images/zone-%d.webp", zone))
		defer f.Close()
		img, _, _ := image.Decode(f)
		imagecache[zone] = img
	}

	f, _ := imagedir.Open("images/220.png")
	locMapIcon, _, _ = image.Decode(f)
	f.Close()

	json.Unmarshal(mapdata, &maps)

	// renderFn is the function that takes map state and renders it to a byte stream,
	// like a PNG or json file.
	var renderFn renderingFn
	if format, found := formats[config.OutputFormat]; found {
		renderFn = format.fn
	} else {
		return fmt.Errorf("invalid output format %q: valid options for -format are \"image\", \"thumbnail\", \"json\"", config.OutputFormat)
	}

	switch config.Mode {
	case HTTPServer:
		census.RateLimit(2, 1)
		sd := filepath.Join(config.OutputDir, "maps-public") // explicitly set a public dir because we're serving static files and don't want to accidentally serve anything but the ones we generate
		slog.Info("starting", "mode", config.Mode, "service_id", config.ServiceID, "bind", config.Bind, "serve_directory", sd)
		return runHTTPServerMode(ctx, config.Bind, sd)
	case MultiFile:
		census.RateLimit(6, 1)
		slog.Info("starting", "mode", config.Mode, "service_id", config.ServiceID, "outputdir", config.OutputDir, "world", config.World, "zone", config.Zone, "renderer", config.OutputFormat)
		return runMultiFileMode(ctx, config.OutputDir, renderFn, config.World, config.Zone)
	case MapDataFile:
		slog.Info("starting", "mode", config.Mode, "service_id", config.ServiceID, "output", config.Output, "environment", config.Env)
		rc := NewAllMapDataJSONReader(ctx, config.Env)
		defer rc.Close()
		return writeToOutput(rc, config.Output)
	case AllRegions:
		slog.Info("starting", "mode", config.Mode, "outputdir", config.OutputDir)
		return runCropAllRegionsMode(ctx, config.OutputDir)
	case SingleFile:
		slog.Info("starting", "mode", config.Mode, "service_id", config.ServiceID, "world", config.World, "zone", config.Zone, "renderer", config.OutputFormat)
		rc := NewRenderZoneReader(ctx, config.World, config.Zone, renderFn)
		defer rc.Close()
		return writeToOutput(rc, config.Output)
	case SingleRegion:
		slog.Info("starting", "mode", config.Mode, "output", config.Output, "region", config.Region)
		rc := RenderMapRegionPNG(config.Region)
		defer rc.Close()
		return writeToOutput(rc, config.Output)
	case ZoneLoc:
		slog.Info("starting", "mode", config.Mode, "output", config.Output, "zone", config.Zone, "loc", config.Loc)
		rc := RenderMapLoc(config.Zone, config.Loc)
		defer rc.Close()
		return writeToOutput(rc, config.Output)
	}
	return nil
}

func writeToOutput(r io.Reader, output string) error {
	if output == "" {
		return fmt.Errorf("no output destination given")
	}

	var w io.Writer
	if output == "-" {
		slog.Debug("writing to stdout")
		w = os.Stdout
	} else {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		slog.Debug("writing to file", "filename", f.Name())
		w = f
	}
	n, err := io.Copy(w, r)
	if err != nil {
		return fmt.Errorf("failed to write output: %w (%d bytes written)", err, n)
	}
	slog.Debug(fmt.Sprintf("finished with %d bytes written", n))
	return nil
}

// crop function? masked vs unmasked
func runCropAllRegionsMode(_ context.Context, dir string) error {

	if err := os.MkdirAll(filepath.Join(dir, "regions"), 0750); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "facilities"), 0750); err != nil {
		return err
	}

	var terrainLOD image.Image

	for _, mapdata := range maps {
		continent, err := mapdata.ZoneID.ContinentID()
		if err != nil {
			slog.Debug("skipping zone", "zone", mapdata.ZoneID, "error", err)
			continue
		}
		// img might be very large, so re-use the same variable to hopefully nuke references for garbage collection
		terrainLOD = getFullsizeMapTerrainImage(continent)

		for _, region := range mapdata.Regions {
			if len(region.Hexes) == 0 {
				slog.Debug("skipping region", "region", region.RegionID, "error", "empty hex list")
				continue
			}

			var w io.Writer = nil

			var regionfile, facilityfile *os.File

			regionfile, err = os.Create(filepath.Join(dir, "regions", fmt.Sprintf("%d.png", region.RegionID)))
			if err != nil {
				slog.Info("failed to create region file", "region", region.RegionID, "error", err)
				continue
			}
			if region.FacilityID == 0 {
				facilityfile = nil
				w = regionfile
			} else {
				facilityfile, err = os.Create(filepath.Join(dir, "facilities", fmt.Sprintf("%d.png", region.FacilityID)))
				if err != nil {
					slog.Info("failed to create facility file", "region", region.RegionID, "error", err)
					regionfile.Close()
					continue
				}
				w = io.MultiWriter(regionfile, facilityfile)
			}
			imgrc := RenderCroppedMapRegionPNG(terrainLOD, mapdata, region, true)
			_, err := io.Copy(w, imgrc)
			if err != nil {
				slog.Info("error white writing map region", "region", region.RegionID, "error", err)
			}
			imgrc.Close()
			regionfile.Close()
			facilityfile.Close()
		}
		// let GC clear references?
		terrainLOD = nil

	}
	return nil
}

func runMultiFileMode(ctx context.Context, dir string, renderFn renderingFn, world ps2.WorldID, zone ps2.ContinentID) error {
	zones := []ps2.ContinentID{ps2.Indar, ps2.Hossin, ps2.Amerish, ps2.Esamir, ps2.Oshur}
	if zone != 0 {
		zones = []ps2.ContinentID{zone}
	}

	worlds := []ps2.WorldID{ps2.Connery, ps2.Miller, ps2.Emerald, ps2.Jaeger, ps2.SolTech, ps2.Genudine, ps2.Ceres}
	if world != 0 {
		worlds = []ps2.WorldID{config.World}
	}

	zids := []ps2.ZoneInstanceID{}
	for _, zone := range zones {
		zids = append(zids, ps2.ZoneInstanceID(zone))
	}

	// errors for missing data can be skipped and let the process complete with exit code 0
	// while printing log messages to stderr.
	// errors for creating directories, files, or writing data should result in a nonzero exit code
	// because something is going wrong that needs to be fixed

	var retryable interface{ Retryable() bool }

	for _, world := range worlds {
		subdir := filepath.Join(dir, worldName(world))
		if err := os.MkdirAll(subdir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", subdir, err)
		}
		mapstates, err := psmap.GetMapState(ctx, world, zids...)
		if errors.As(err, &retryable) && !retryable.Retryable() {
			return err
		}
		if err != nil {
			slog.Info("failed to get map state", "world", worldName(world), "error", err)
			continue
		}
		for _, state := range mapstates {
			continent := state.ZoneID.ZoneID()
			mapdata, err := getMapData(continent)
			if err != nil {
				slog.Info("failed to get map data", "zone", zoneName(continent), "error", err)
				continue
			}

			fileName := filepath.Join(dir, worldName(world), zoneName(continent)+formats[config.OutputFormat].extension)

			renderer := renderFn(mapdata, state)
			defer renderer.Close()

			// encode to a buffer first so that if there were an encoding error for some reason,
			// we don't truncate an existing map file
			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, renderer)
			if err != nil {
				// a rendering error could be caused by missing map data,
				// but rendering the working maps is better than returning with a failure here
				slog.Info("error rendering map", "zone", zoneName(continent), "format", config.OutputFormat, "error", err)
				continue
			}
			f, err := os.Create(fileName)
			if err != nil {
				// if we can't create a file here then something is wrong in a way that will prevent this program from executing as the user expected,
				// so we want to report back a failure.
				// this includes file create permissions.
				return fmt.Errorf("unable to create file %q: %w", fileName, err)
			}
			_, err = io.Copy(f, &buf)
			f.Close() // not deferred because we're in a loop, so be extra careful not to miss any return paths
			if err != nil {
				slog.Info("error while writing image", "file", fileName, "error", err)
				continue
			}
		}
	}
	return nil
}

type contextHandler struct {
	slog.Handler
}

var correlationID = contextKey("correlation_id")

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if uuid, ok := ctx.Value(correlationID).(uuid.UUID); ok {
		r.AddAttrs(slog.String(string(correlationID), uuid.String()))
	}
	return h.Handler.Handle(ctx, r)
}

type contextKey string

func injectCorrelationID(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, correlationID, uuid.New())
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func runHTTPServerMode(ctx context.Context, bind string, dir string) error {
	ctx, shutdown := context.WithCancelCause(ctx)
	defer shutdown(nil)
	var err error

	updateInterval := 5 * time.Minute

	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return fmt.Errorf("setup failed: create dir %q: %w", dir, err)
	}

	slog.Info("retrieving game state from census")
	err = runMultiFileMode(ctx, dir, RenderMapImageDefaultPNG, 0, 0)
	if err != nil {
		return fmt.Errorf("setup failed: initial map state: %w", err)
	}

	slog.Info("generating map region images")
	err = runCropAllRegionsMode(ctx, dir)
	if err != nil {
		return fmt.Errorf("setup failed: generate regions: %w", err)
	}

	cacheControl := func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/regions/") ||
				strings.HasPrefix(r.URL.Path, "/facilities/") {
				w.Header().Set("Cache-Control", "public, max-age=2592000")
			} else {
				// assume all other paths inside this fileserver are live maps
				w.Header().Set("Refresh", "120")
				w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(updateInterval/time.Second)))
			}
			next.ServeHTTP(w, r)
		}
	}

	logRequest := func(next http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			slog.InfoContext(r.Context(), "incoming http request",
				"request", fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto),
			)
			next.ServeHTTP(w, r)
		}
	}

	router := http.NewServeMux()
	router.Handle("/", http.FileServer(http.Dir(dir)))

	var h http.Handler = router
	h = cacheControl(h)
	h = logRequest(h)
	// h = injectCorrelationID(h) // commented out after switching to serving pre-generated static files. put it back if any routes spawn other processes.

	srv := http.Server{
		Addr:    bind,
		Handler: h,
	}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(updateInterval):
				slog.Info("retrieving game state from census")
				runerr := runMultiFileMode(ctx, dir, RenderMapImageDefaultPNG, 0, 0)
				if runerr != nil {
					slog.Info("failed to generate new maps", "error", runerr)
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("starting http service", "bind", config.Bind)
		defer slog.Info("stopped http service")
		shutdown(srv.ListenAndServe())
	}()

	wg.Add(1)
	go func() {
		// this goroutine waits for a cancelled context and then tries to gracefully shut down the http server
		defer wg.Done()
		<-ctx.Done()
		wait := 5 * time.Second
		waitctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		if err := srv.Shutdown(waitctx); err != nil {
			slog.Info("error while stopping http server", "error", err, "wait", wait)
		}
	}()
	wg.Wait()
	<-ctx.Done()
	return context.Cause(ctx)
}

var errGracefulShutdown = errors.New("received shutdown signal")

//go:embed images
var imagedir embed.FS

const terrainDimensions = 512 // size of terrain maps that are embedded in imagedir

// imagecache caches the decoded terrain images.
// These terrain images are 512x512 because it is large enough to be viewed in relative detail in a discord message,
// while being small enough that memory usage, processor usage, and badwidth are not extreme.
var imagecache = map[ps2.ContinentID]image.Image{}

func getMapTerrainImage(continent ps2.ContinentID) image.Image {
	if img, ok := imagecache[continent]; ok {
		return img
	}
	return image.Rect(0, 0, terrainDimensions, terrainDimensions)
}

var mapdownloadmutex = sync.Mutex{}

func getFullsizeMapTerrainImage(continent ps2.ContinentID) image.Image {
	mapdownloadmutex.Lock()
	defer mapdownloadmutex.Unlock()

	// LOD0 is 8192x8192 for most continents, which uses at least 350MB of memory to load the entire image without any special optimizations
	// use the smallest image possible that's still large enough to see detail when cropping down to a single region
	const LOD = "1" // LOD1 is 4096x4096

	storagedir := filepath.Join(os.TempDir(), "mapgen-cache")
	filename := filepath.Join(storagedir, continent.String())
	LODs := map[ps2.ContinentID]string{
		ps2.Indar:   "https://raw.githubusercontent.com/cooltrain7/Planetside-2-API-Tracker/refs/heads/master/Maps/Lods/Indar/Indar_LOD" + LOD + ".png",
		ps2.Hossin:  "https://raw.githubusercontent.com/cooltrain7/Planetside-2-API-Tracker/refs/heads/master/Maps/Lods/Hossin/Hossin_LOD" + LOD + ".png",
		ps2.Amerish: "https://raw.githubusercontent.com/cooltrain7/Planetside-2-API-Tracker/refs/heads/master/Maps/Lods/Amerish/Amerish_LOD" + LOD + ".png",
		ps2.Esamir:  "https://raw.githubusercontent.com/cooltrain7/Planetside-2-API-Tracker/refs/heads/master/Maps/Lods/Esamir/Esamir_LOD" + LOD + ".png",
		ps2.Oshur:   "https://raw.githubusercontent.com/cooltrain7/Planetside-2-API-Tracker/refs/heads/master/Maps/Lods/Oshur/Oshur_LOD" + LOD + ".png",
	}

	reader, writer := io.Pipe()
	defer writer.Close()

	errstring := "falling back to lower resolution embedded image"

	f, err := os.Open(filename)
	if err != nil {
		slog.Debug("cache miss", "zone", continent, "file", filename)
		url, found := LODs[continent]
		if !found {
			slog.Debug(errstring, "error", "no terrain image available for zone", "zone", continent)
			return getMapTerrainImage(continent)
		}

		err := os.MkdirAll(storagedir, 0700)
		if err != nil {
			// this might be a configuration issue, so surface the message higher than debug
			slog.Info(errstring, "zone", continent, "error", "failed to create cache dir", "dir", storagedir, "oserr", err)
			return getMapTerrainImage(continent)
		}

		response, err := http.Get(url)
		if err != nil {
			slog.Info(errstring, "zone", continent, "error", err, "statuscode", response.StatusCode, "url", url)
			return getMapTerrainImage(continent)
		}
		defer response.Body.Close()

		if response.StatusCode != 200 {
			slog.Info(errstring, "zone", continent, "error", "http response returned bad status code", "code", response.Status, "url", url)
			return getMapTerrainImage(continent)
		}
		slog.Info("downloading full size map image", "zone", continent, "content_length", response.Header.Get("Content-Length"), "url", url, "savepath", filename)
		cachefile, err := os.Create(filename)
		if err != nil {
			slog.Info("failed to open file", "file", filename, "error", err, "zone", continent)
			// failing to open the file for caching is notable but doesn't prevent us from trying to load the image
		}
		defer cachefile.Close()

		w := io.MultiWriter(writer, cachefile)
		go func() {
			n, err := io.Copy(w, response.Body)
			if err != nil {
				writer.CloseWithError(err)
				cachefile.Close()
				rerr := os.Remove(cachefile.Name())
				slog.Debug("removing cache file", "file", cachefile.Name(), "error", rerr)
			}
			slog.Debug("downloading full size map image complete", "zone", continent, "bytes", n)
		}()
	} else {
		slog.Debug("cache hit", "zone", continent, "file", f.Name())
		go func() {
			_, err := io.Copy(writer, f)
			writer.CloseWithError(err)
		}()
	}

	img, _, err := image.Decode(reader)
	if err != nil {
		slog.Info(errstring, "zone", continent, "error", err)
		return getMapTerrainImage(continent)
	}

	return img
}

//go:embed mapdata.json
var mapdata []byte

var maps []psmap.Map

func getMapData(continent ps2.ContinentID) (data psmap.Map, err error) {
	for _, m := range maps {
		c, err := m.ZoneID.ContinentID()
		if err != nil {
			continue
		}
		if c == continent {
			return m, nil
		}
	}
	return data, fmt.Errorf("missing continent %d from data source", continent)
}

var locMapIcon image.Image

// NewAllMapDataJSONReader returns a new renderingFn that looks up map data from census for env and renders to JSON.
func NewAllMapDataJSONReader(ctx context.Context, env ps2.Environment) io.ReadCloser {
	r, w := io.Pipe()
	data, err := psmap.GetAllMapData(ctx, env)
	if err != nil {
		w.CloseWithError(err)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	go func() {
		w.CloseWithError(encoder.Encode(data))
	}()
	return r
}

// NewRenderZoneReader returns a new renderingFn that looks up map data from census for env and renders to JSON.
func NewRenderZoneReader(ctx context.Context, world ps2.WorldID, zone ps2.ContinentID, renderFn renderingFn) io.ReadCloser {
	// this pipe is only used to pipe the error into a reader
	r, w := io.Pipe()
	defer w.Close()

	renderErr := func(e error) io.ReadCloser {
		w.CloseWithError(e)
		return r
	}

	if world == 0 || zone == 0 {
		return renderErr(errors.New("the value for -world and -zone must me given when generating a single map"))
	}

	states, err := psmap.GetMapState(ctx, world, ps2.ZoneInstanceID(zone))
	if err != nil {
		return renderErr(fmt.Errorf("failed to get map state: %w", err))
	}
	if len(states) < 1 {
		return renderErr(errNotFound)
	}

	data, err := getMapData(zone)
	if err != nil {
		return renderErr(fmt.Errorf("failed to get map data: %w", err))
	}

	go func() {
		renderer := renderFn(data, states[0])
		defer renderer.Close()
		_, err := io.Copy(w, renderer)
		w.CloseWithError(err)
	}()
	return r

}

var errNotFound = errors.New("not found")

// renderingFn is a function that takes a map state and returns an io.ReadCloser.
// The reader returns a byte stream such as a PNG, json file, or any other format a map state might be rendered to.
type renderingFn func(psmap.Map, psmap.State) io.ReadCloser

// todo: pull the census request out of the rendering function

// RenderMapImageDefaultPNG is a renderingFn that renders a 512x512 PNG image with map terrain.
func RenderMapImageDefaultPNG(data psmap.Map, mapstate psmap.State) io.ReadCloser {
	r, w := io.Pipe()
	terrainImage := getMapTerrainImage(mapstate.ZoneID.ZoneID())
	img := image.NewRGBA(terrainImage.Bounds())
	draw.Draw(img, img.Bounds(), terrainImage, img.Bounds().Min, draw.Src)
	err := psmap.Draw(img, data, mapstate)
	if err != nil {
		w.CloseWithError(fmt.Errorf("unable to draw map: %w", err))
		return r
	}
	go func() {
		w.CloseWithError(png.Encode(w, img))
	}()
	return r
}

// RenderMapImageNoBackgroundPNG is a renderingFn that renders a 512x512 PNG image without map terrain.
func RenderMapImageNoBackgroundPNG(data psmap.Map, mapstate psmap.State) io.ReadCloser {
	r, w := io.Pipe()
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	err := psmap.Draw(img, data, mapstate)
	if err != nil {
		w.CloseWithError(fmt.Errorf("unable to draw map: %w", err))
		return r
	}
	go func() {
		encoder := png.Encoder{
			CompressionLevel: png.BestCompression,
		}
		w.CloseWithError(encoder.Encode(w, img))
	}()
	return r
}

// RenderMapImageDiscordThumbnailPNG is a renderingFn that renders a 128x128 PNG image with a transparent background.
func RenderMapImageDiscordThumbnailPNG(data psmap.Map, mapstate psmap.State) io.ReadCloser {
	r, w := io.Pipe()
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	err := psmap.Draw(img, data, mapstate)
	if err != nil {
		w.CloseWithError(fmt.Errorf("unable to draw map: %w", err))
		return r
	}
	go func() {
		w.CloseWithError(png.Encode(w, img))
	}()
	return r
}

// RenderMapStateJSON is a renderingFn that renders map state as json.
func RenderMapStateJSON(data psmap.Map, mapstate psmap.State) io.ReadCloser {
	r, w := io.Pipe()
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	go func() {
		w.CloseWithError(encoder.Encode(mapstate))
	}()
	return r
}

func RenderCroppedMapRegionPNG(terrainLOD image.Image, mapdata psmap.Map, reg psmap.Region, trim bool) io.ReadCloser {
	// img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	// img := base.SubImage(image.Rect(200, 200, 400, 400))
	r, w := io.Pipe()
	renderErr := func(e error) io.ReadCloser {
		w.CloseWithError(e)
		return r
	}

	if len(reg.Hexes) == 0 {
		return renderErr(errors.New("empty hex list"))
	}

	regionBounds, err := psmap.Bounds(terrainLOD.Bounds(), mapdata, reg.Hexes)
	if err != nil {
		return renderErr(err)
	}

	img := image.NewRGBA(regionBounds)
	draw.Draw(img, regionBounds, terrainLOD, regionBounds.Min, draw.Src)

	scale := float64(terrainLOD.Bounds().Dx()) / float64(mapdata.Size)

	if !trim {
		mask, err := psmap.GenerateMask(regionBounds, mapdata, reg.Hexes, scale, regionBounds.Min, color.Transparent, color.Opaque)
		if err != nil {
			return renderErr(err)
		}
		draw.DrawMask(img, img.Bounds(), image.NewUniform(color.Black), img.Bounds().Min, mask, mask.Bounds().Min, draw.Over)
	} else {
		mask, err := psmap.GenerateMask(regionBounds, mapdata, reg.Hexes, scale, regionBounds.Min, color.Opaque, color.Opaque)
		if err != nil {
			return renderErr(err)
		}
		draw.DrawMask(img, img.Bounds(), img, img.Bounds().Min, mask, mask.Bounds().Min, draw.Src)
	}

	go func() {
		w.CloseWithError(png.Encode(w, img))
	}()

	return r
}

func RenderMapRegionPNG(region ps2.RegionID) io.ReadCloser {
	// img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	// img := base.SubImage(image.Rect(200, 200, 400, 400))
	r, w := io.Pipe()
	renderErr := func(e error) io.ReadCloser {
		w.CloseWithError(e)
		return r
	}

	mapdata, reg, err := findRegion(region)
	if err != nil {
		return renderErr(err)
	}

	continent, err := mapdata.ZoneID.ContinentID()
	if err != nil {
		return renderErr(err)
	}
	terrainImage := getFullsizeMapTerrainImage(continent) //todo: get LOD0

	regionBounds, err := psmap.Bounds(terrainImage.Bounds(), mapdata, reg.Hexes)
	if err != nil {
		return renderErr(err)
	}

	img := image.NewRGBA(regionBounds)
	draw.Draw(img, regionBounds, terrainImage, regionBounds.Min, draw.Src)

	scale := float64(terrainImage.Bounds().Dx()) / float64(mapdata.Size)
	mask, err := psmap.GenerateMask(regionBounds, mapdata, reg.Hexes, scale, regionBounds.Min, color.Transparent, color.Opaque)
	if err != nil {
		return renderErr(err)
	}

	draw.DrawMask(img, img.Bounds(), image.NewUniform(color.Black), img.Bounds().Min, mask, mask.Bounds().Min, draw.Over)
	go func() {
		w.CloseWithError(png.Encode(w, img))
	}()

	return r
}

func RenderMapLoc(zone ps2.ContinentID, loc psmap.Loc) io.ReadCloser {
	r, w := io.Pipe()
	renderErr := func(e error) io.ReadCloser {
		w.CloseWithError(e)
		return r
	}
	if zone == 0 {
		return renderErr(fmt.Errorf("-zone is required with -loc"))
	}
	mapdata, err := getMapData(zone)
	if err != nil {
		return renderErr(err)
	}

	terrainImage := getFullsizeMapTerrainImage(zone)

	// get bounds for an area within the map terrain image with our loc in the center
	regionBounds, err := psmap.LocBounds(terrainImage.Bounds(), mapdata, loc)
	if err != nil {
		return renderErr(err)
	}

	// make a new image with the same dimensions as the returned bounds but located at 0,0
	img := image.NewRGBA(image.Rect(0, 0, regionBounds.Dx(), regionBounds.Dy()))

	// copy the bounded region from the location onto our output image, effectively cropping it
	draw.Draw(img, img.Bounds(), terrainImage, regionBounds.Min, draw.Src)

	if loc.Heading != 0 {
		// put heading 0.001 if you want the arrow rendered east
		var playerArrow image.Image = transform.Rotate(locMapIcon, loc.Bearing(), nil)
		// playerArrow = locMapIcon
		centerImg := image.Point{
			X: img.Bounds().Dx() / 2,
			Y: img.Bounds().Dy() / 2,
		}.Sub(image.Point{
			X: playerArrow.Bounds().Dx() / 2,
			Y: playerArrow.Bounds().Dy() / 2,
		})

		// draw the player map icon (triangle) on the center of our returned image
		draw.Draw(img, playerArrow.Bounds().Add(centerImg), playerArrow, playerArrow.Bounds().Min, draw.Over)
	}

	go func() {
		w.CloseWithError(png.Encode(w, img))
	}()

	return r
}

func findRegion(r ps2.RegionID) (psmap.Map, psmap.Region, error) {

	for _, mapdata := range maps {
		for _, region := range mapdata.Regions {
			if region.RegionID == r {
				return mapdata, region, nil
			}
		}
	}

	return psmap.Map{}, psmap.Region{}, fmt.Errorf("unable to find region %d in map data", r)
}

func parseWorld(s string) ps2.WorldID {
	s = strings.ToLower(s)
	switch s {
	case "connery":
		return ps2.Connery
	case "miller":
		return ps2.Miller
	case "cobalt":
		return ps2.Cobalt
	case "emerald":
		return ps2.Emerald
	case "jaeger":
		return ps2.Jaeger
	case "apex":
		return ps2.Apex
	case "briggs":
		return ps2.Briggs
	case "soltech":
		return ps2.SolTech
	case "genudine":
		return ps2.Genudine
	case "palos":
		return ps2.Palos
	case "crux":
		return ps2.Crux
	case "searhus":
		return ps2.Searhus
	case "xelas":
		return ps2.Xelas
	case "ceres":
		return ps2.Ceres
	case "lithcorp":
		return ps2.Lithcorp
	case "rashnu":
		return ps2.Rashnu
	default:
		return 0
	}
}

func worldName(w ps2.WorldID) string {
	switch w {
	case ps2.Connery:
		return "connery"
	case ps2.Miller:
		return "miller"
	case ps2.Cobalt:
		return "cobalt"
	case ps2.Emerald:
		return "emerald"
	case ps2.Jaeger:
		return "jaeger"
	case ps2.Apex:
		return "apex"
	case ps2.Briggs:
		return "briggs"
	case ps2.SolTech:
		return "soltech"
	case ps2.Genudine:
		return "genudine"
	case ps2.Palos:
		return "palos"
	case ps2.Crux:
		return "crux"
	case ps2.Searhus:
		return "searhus"
	case ps2.Xelas:
		return "xelas"
	case ps2.Ceres:
		return "ceres"
	case ps2.Lithcorp:
		return "lithcorp"
	case ps2.Rashnu:
		return "rashnu"
	default:
		return fmt.Sprintf("world-%d", w)
	}
}

func zoneName(c ps2.ContinentID) string {
	switch c {
	case ps2.Indar:
		return "indar"
	case ps2.Amerish:
		return "amerish"
	case ps2.Oshur:
		return "oshur"
	case ps2.Hossin:
		return "hossin"
	case ps2.Esamir:
		return "esamir"
	default:
		return fmt.Sprintf("zone-%d", c)
	}
}

func parseZone(s string) ps2.ContinentID {
	s = strings.ToLower(s)
	switch s {
	case "indar":
		return ps2.Indar
	case "amerish":
		return ps2.Amerish
	case "oshur":
		return ps2.Oshur
	case "hossin":
		return ps2.Hossin
	case "esamir":
		return ps2.Esamir
	default:
		return 0
	}
}
