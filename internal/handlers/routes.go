package handlers

import (
	"errors"
	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/rs/zerolog"
	"github.com/seanime-app/seanime/internal/core"
	"github.com/seanime-app/seanime/internal/util"
	util2 "github.com/seanime-app/seanime/internal/util/proxies"
	"strings"
	"sync"
)

// InitRoutes initializes the routes for the backend server.
// It takes the App instance and the Fiber app instance as arguments.
// The App instance is passed to the route handlers, so they can access the app's state.
func InitRoutes(app *core.App, fiberApp *fiber.App) {

	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Set up a custom logger for fiber.
	// This is not instantiated in `core.NewFiberApp` because we do not want to log requests for the static file server.
	fiberLogger := fiberzerolog.New(fiberzerolog.Config{
		Logger: app.Logger,
		SkipURIs: []string{
			"/internal/metrics",
			"/_next",
			"/icons",
			"/api/v1/image-proxy",
		},
		Fields:   []string{"method", "error", "status", "url"},
		Messages: []string{"req: error", "req: client error", "req: Success"},
		Levels:   []zerolog.Level{zerolog.ErrorLevel, zerolog.WarnLevel, zerolog.InfoLevel},
	})
	fiberApp.Use(fiberLogger)

	api := fiberApp.Group("/api")
	v1 := api.Group("/v1")

	if app.IsOffline() {
		v1.Use(func(c *fiber.Ctx) error {
			uriS := strings.Split(c.Request().URI().String(), "v1")
			if len(uriS) > 1 {
				if strings.HasPrefix(uriS[1], "/offline") ||
					strings.HasPrefix(uriS[1], "/settings") ||
					strings.HasPrefix(uriS[1], "/theme") ||
					strings.HasPrefix(uriS[1], "/status") ||
					strings.HasPrefix(uriS[1], "/media-player") ||
					strings.HasPrefix(uriS[1], "/playback-manager") ||
					strings.HasPrefix(uriS[1], "/playlists") ||
					strings.HasPrefix(uriS[1], "/directory-selector") ||
					strings.HasPrefix(uriS[1], "/manga") ||
					strings.HasPrefix(uriS[1], "/open-in-explorer") {
					return c.Next()
				} else {
					return c.Status(200).SendString("offline")
				}
			}
			return c.Next()
		})
	}

	// Image Proxy
	imageProxy := &util2.ImageProxy{}
	v1.Get("/image-proxy", imageProxy.ProxyImage)

	//
	// General
	//
	v1.Get("/status", makeHandler(app, HandleStatus))

	// Auth
	v1.Post("/auth/login", makeHandler(app, HandleLogin))
	v1.Post("/auth/logout", makeHandler(app, HandleLogout))

	// Settings
	v1.Get("/settings", makeHandler(app, HandleGetSettings))
	v1.Patch("/settings", makeHandler(app, HandleSaveSettings))
	v1.Patch("/settings/list-sync", makeHandler(app, HandleSaveListSyncSettings))
	v1.Patch("/settings/auto-downloader", makeHandler(app, HandleSaveAutoDownloaderSettings))

	// List Sync
	v1.Get("/list-sync/anime-diffs", makeHandler(app, HandleGetListSyncAnimeDiffs))
	v1.Post("/list-sync/cache", makeHandler(app, HandleDeleteListSyncCache))
	v1.Post("/list-sync/anime", makeHandler(app, HandleSyncAnime))

	// Auto Downloader
	v1.Post("/auto-downloader/run", makeHandler(app, HandleRunAutoDownloader))
	v1.Get("/auto-downloader/rule/:id", makeHandler(app, HandleGetAutoDownloaderRule))
	v1.Get("/auto-downloader/rules", makeHandler(app, HandleGetAutoDownloaderRules))
	v1.Post("/auto-downloader/rule", makeHandler(app, HandleCreateAutoDownloaderRule))
	v1.Patch("/auto-downloader/rule", makeHandler(app, HandleUpdateAutoDownloaderRule))
	v1.Delete("/auto-downloader/rule/:id", makeHandler(app, HandleDeleteAutoDownloaderRule))

	v1.Get("/auto-downloader/items", makeHandler(app, HandleGetAutoDownloaderItems))
	v1.Delete("/auto-downloader/item", makeHandler(app, HandleDeleteAutoDownloaderItem))

	// Other
	v1.Post("/test-dump", makeHandler(app, HandleTestDump))

	// Directory selector input
	// POST /v1/directory-selector
	v1.Post("/directory-selector", makeHandler(app, HandleDirectorySelector))

	// Open directory in explorer
	// POST /v1/open-in-explorer
	v1.Post("/open-in-explorer", makeHandler(app, HandleOpenInExplorer))

	// Open Media Player
	// POST /v1/media-player/start
	v1.Post("/media-player/start", makeHandler(app, HandleStartDefaultMediaPlayer))

	// POST /v1/media-player/play
	v1.Post("/media-player/play", makeHandler(app, HandlePlayVideo))

	//
	// AniList
	//

	v1Anilist := v1.Group("/anilist")

	// Get "cached" AniList collection
	// GET /v1/anilist/collection
	v1Anilist.Get("/collection", makeHandler(app, HandleGetAnilistCollection))

	// Get (up-to-date) AniList collection
	// This refreshes the collection held by the app
	// POST /v1/anilist/collection
	v1Anilist.Post("/collection", makeHandler(app, HandleGetAnilistCollection))

	// Get details for AniList media
	// GET /v1/anilist/media-details
	v1Anilist.Get("/media-details/:id", makeHandler(app, HandleGetAnilistMediaDetails))

	// Edit AniList List Entry
	// POST /v1/anilist/list-entry
	v1Anilist.Post("/list-entry", makeHandler(app, HandleEditAnilistListEntry))

	// Delete AniList List Entry
	// POST /v1/anilist/list-entry
	v1Anilist.Delete("/list-entry", makeHandler(app, HandleDeleteAnilistListEntry))

	v1Anilist.Post("/list-anime", makeHandler(app, HandleAnilistListAnime))
	v1Anilist.Post("/list-recent-anime", makeHandler(app, HandleAnilistListRecentAiringAnime))

	//
	// MAL
	//

	// Authenticate user with MAL
	// POST /v1/mal/auth
	v1.Post("/mal/auth", makeHandler(app, HandleMALAuth))
	// Logout from MAL
	// POST /v1/mal/logout
	v1.Post("/mal/logout", makeHandler(app, HandleMALLogout))

	//
	// Library
	//

	v1Library := v1.Group("/library")

	// Scan the library
	v1Library.Post("/scan", makeHandler(app, HandleScanLocalFiles))

	// DELETE /v1/library/empty-directories
	v1Library.Delete("/empty-directories", makeHandler(app, HandleRemoveEmptyDirectories))

	// Get all the local files from the database
	// GET /v1/library/local-files
	v1Library.Get("/local-files", makeHandler(app, HandleGetLocalFiles))

	// POST /v1/library/local-files
	v1Library.Post("/local-files", makeHandler(app, HandleLocalFileBulkAction))

	// DELETE /v1/library/local-files
	v1Library.Delete("/local-files", makeHandler(app, HandleDeleteLocalFiles))

	// Get the library collection
	// GET /v1/library/collection
	v1Library.Get("/collection", makeHandler(app, HandleGetLibraryCollection))

	// Get the latest scan summaries
	// GET /v1/library/scan-summaries
	v1Library.Get("/scan-summaries", makeHandler(app, HandleGetLatestScanSummaries))

	// Get missing episodes
	// GET /v1/library/missing-episodes
	v1Library.Get("/missing-episodes", makeHandler(app, HandleGetMissingEpisodes))

	// Update local file data
	// PATCH /v1/library/local-file
	v1Library.Patch("/local-file", makeHandler(app, HandleUpdateLocalFileData))

	// Retrieve MediaEntry
	// GET /v1/library/media-entry
	v1Library.Get("/media-entry/:id", makeHandler(app, HandleGetMediaEntry))

	// Get suggestions for a prospective Media Entry
	// POST /v1/library/collection
	v1Library.Post("/media-entry/suggestions", makeHandler(app, HandleFindProspectiveMediaEntrySuggestions))

	// Create Media Entry from directory path and AniList media id
	// POST /v1/library/media-entry/manual-match
	v1Library.Post("/media-entry/manual-match", makeHandler(app, HandleMediaEntryManualMatch))

	// Media Entry Bulk Action
	// PATCH /v1/library/entry/bulk-action
	v1Library.Patch("/media-entry/bulk-action", makeHandler(app, HandleMediaEntryBulkAction))

	// Open Media Entry in File Explorer
	// POST /v1/library/media-entry/open-in-explorer
	v1Library.Post("/media-entry/open-in-explorer", makeHandler(app, HandleOpenMediaEntryInExplorer))

	// Add unknown media by IDs
	// POST /v1/library/unknown-media
	v1Library.Post("/media-entry/unknown-media", makeHandler(app, HandleAddUnknownMedia))

	v1Library.Post("/media-entry/update-progress", makeHandler(app, HandleUpdateProgress))

	v1Library.Get("/media-entry/silence/:id", makeHandler(app, HandleGetMediaEntrySilenceStatus))
	v1Library.Post("/media-entry/silence", makeHandler(app, HandleToggleMediaEntrySilenceStatus))

	//
	// Torrent / Torrent Client
	//

	v1.Post("/torrent-search", makeHandler(app, HandleTorrentSearch))
	v1.Post("/torrent-nsfw-search", makeHandler(app, HandleNsfwTorrentSearch))
	v1.Post("/torrent-client/download", makeHandler(app, HandleTorrentClientDownload))
	v1.Get("/torrent-client/list", makeHandler(app, HandleGetActiveTorrentList))
	v1.Post("/torrent-client/action", makeHandler(app, HandleTorrentClientAction))
	v1.Post("/torrent-client/rule-magnet", makeHandler(app, HandleTorrentClientAddMagnetFromRule))

	//
	// Download
	//

	v1.Post("/download-torrent-file", makeHandler(app, HandleDownloadTorrentFile))

	//
	// Updates
	//

	v1.Get("/latest-update", makeHandler(app, HandleGetLatestUpdate))
	v1.Post("/download-release", makeHandler(app, HandleDownloadRelease))

	//
	// Theme
	//

	v1.Get("/theme", makeHandler(app, HandleGetTheme))
	v1.Patch("/theme", makeHandler(app, HandleUpdateTheme))

	//
	// Playback Manager
	//

	v1.Post("/playback-manager/sync-current-progress", makeHandler(app, HandlePlaybackSyncCurrentProgress))
	v1.Post("/playback-manager/start-playlist", makeHandler(app, HandlePlaybackStartPlaylist))
	v1.Post("/playback-manager/playlist-next", makeHandler(app, HandlePlaybackPlaylistNext))
	v1.Post("/playback-manager/cancel-playlist", makeHandler(app, HandlePlaybackCancelCurrentPlaylist))
	v1.Post("/playback-manager/next-episode", makeHandler(app, HandlePlaybackPlayNextEpisode))

	//
	// Playlists
	//

	v1.Get("/playlists", makeHandler(app, HandleGetPlaylists))
	v1.Post("/playlist", makeHandler(app, HandleCreatePlaylist))
	v1.Patch("/playlist", makeHandler(app, HandleUpdatePlaylist))
	v1.Delete("/playlist", makeHandler(app, HandleDeletePlaylist))
	v1.Get("/playlist/episodes/:id/:progress", makeHandler(app, HandleGetPlaylistEpisodes))

	//
	// Onlinestream
	//

	v1.Post("/onlinestream/episode-source", makeHandler(app, HandleGetOnlineStreamEpisodeSource))
	v1.Post("/onlinestream/episode-list", makeHandler(app, HandleGetOnlineStreamEpisodeList))
	v1.Delete("/onlinestream/cache", makeHandler(app, HandleOnlineStreamEmptyCache))

	//
	// Metadata Provider
	//

	v1.Post("/metadata-provider/tvdb-episodes", makeHandler(app, HandlePopulateTVDBEpisodes))
	v1.Delete("/metadata-provider/tvdb-episodes", makeHandler(app, HandleEmptyTVDBEpisodes))

	//
	// Manga
	//

	v1Manga := v1.Group("/manga")
	v1Manga.Post("/anilist/collection", makeHandler(app, HandleGetAnilistMangaCollection))
	v1Manga.Post("/anilist/list", makeHandler(app, HandleAnilistListManga))
	v1Manga.Get("/collection", makeHandler(app, HandleGetMangaCollection))
	v1Manga.Get("/entry/:id", makeHandler(app, HandleGetMangaEntry))
	v1Manga.Get("/entry/:id/details", makeHandler(app, HandleGetMangaEntryDetails))
	v1Manga.Delete("/entry/cache", makeHandler(app, HandleEmptyMangaEntryCache))
	v1Manga.Post("/chapters", makeHandler(app, HandleGetMangaEntryChapters))
	v1Manga.Post("/pages", makeHandler(app, HandleGetMangaEntryPages))
	//v1Manga.Post("/entry/backups", makeHandler(app, HandleGetMangaEntryBackups))
	//v1Manga.Post("/download-chapter", makeHandler(app, HandleDownloadMangaChapter))
	v1Manga.Post("/update-progress", makeHandler(app, HandleUpdateMangaProgress))

	v1Manga.Get("/downloads", makeHandler(app, HandleGetMangaDownloadsList))
	v1Manga.Post("/download-chapters", makeHandler(app, HandleDownloadMangaChapters))
	v1Manga.Post("/download-data", makeHandler(app, HandleGetMangaDownloadData))
	v1Manga.Delete("/download-chapter", makeHandler(app, HandleDeleteMangaChapterDownload))
	v1Manga.Post("/download-data/refresh", makeHandler(app, HandleRefreshMangaDownloadData))
	v1Manga.Get("/download-queue", makeHandler(app, HandleGetMangaDownloadQueue))
	v1Manga.Post("/download-queue/start", makeHandler(app, HandleStartMangaDownloadQueue))
	v1Manga.Post("/download-queue/stop", makeHandler(app, HandleStopMangaDownloadQueue))
	v1Manga.Delete("/download-queue", makeHandler(app, HandleClearAllChapterDownloadQueue))
	v1Manga.Post("/download-queue/reset-errored", makeHandler(app, HandleResetErroredChapterDownloadQueue))

	//
	// File Cache
	//

	v1FileCache := v1.Group("/filecache")
	v1FileCache.Get("/total-size", makeHandler(app, HandleGetFileCacheTotalSize))
	v1FileCache.Delete("/bucket", makeHandler(app, HandleRemoveFileCacheBucket))

	//
	// Discord
	//

	v1Discord := v1.Group("/discord")
	v1Discord.Post("/presence/manga", makeHandler(app, HandleSetDiscordMangaActivity))
	v1Discord.Post("/presence/cancel", makeHandler(app, HandleCancelDiscordActivity))

	//
	// Offline
	//

	v1.Get("/offline/snapshot", makeHandler(app, HandleGetOfflineSnapshot))
	v1.Get("/offline/snapshot-entry", makeHandler(app, HandleGetOfflineSnapshotEntry))
	v1.Post("/offline/snapshot", makeHandler(app, HandleCreateOfflineSnapshot))
	v1.Patch("/offline/snapshot-entry", makeHandler(app, HandleUpdateOfflineEntryListData))

	//
	// Websocket
	//

	fiberApp.Use("/events", websocketUpgradeMiddleware)
	// Create a new websocket event handler.
	// This will be used to send real-time events to the client.
	// It also attaches the websocket connection to the app instance, so it is available to other handlers.
	fiberApp.Get("/events", newWebSocketEventHandler(app))

}

//----------------------------------------------------------------------------------------------------------------------

// RouteCtx is a context object that is passed to route handlers.
// It contains the App instance and the Fiber context.
type RouteCtx struct {
	App   *core.App
	Fiber *fiber.Ctx
}

// RouteCtx pool
// This is used to avoid allocating memory for each request
var syncPool = sync.Pool{
	New: func() interface{} {
		return &RouteCtx{}
	},
}

// makeHandler creates a new route handler function.
// It takes the App instance and a custom handler function as arguments.
// The custom handler function is similar to a fiber handler, but it takes a RouteCtx as an argument, allowing route handlers to access the app's state.
// We use a sync.Pool to avoid allocating memory for each request.
func makeHandler(app *core.App, handler func(*RouteCtx) error) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) (err error) {
		defer util.HandlePanicInModuleThen("handlers/routes", func() {
			err = errors.New("runtime panic")
		})

		ctx := syncPool.Get().(*RouteCtx)
		defer syncPool.Put(ctx)
		ctx.App = app
		ctx.Fiber = c
		return handler(ctx)
	}
}

func (c *RouteCtx) AcceptJSON() {
	c.Fiber.Accepts(fiber.MIMEApplicationJSON)
}

// RespondWithData responds with a JSON response containing the given data.
func (c *RouteCtx) RespondWithData(data any) error {
	return c.Fiber.Status(200).JSON(NewDataResponse(data))
}

// RespondWithError responds with a JSON response containing the given error.
func (c *RouteCtx) RespondWithError(err error) error {
	return c.Fiber.Status(500).JSON(NewErrorResponse(err))
}
