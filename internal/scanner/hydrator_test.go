package scanner

import (
	"github.com/seanime-app/seanime/internal/anilist"
	"github.com/seanime-app/seanime/internal/anizip"
	"github.com/seanime-app/seanime/internal/entities"
	"github.com/seanime-app/seanime/internal/limiter"
	"github.com/seanime-app/seanime/internal/util"
	"testing"
)

func TestFileHydrator_HydrateMetadata(t *testing.T) {

	allMedia := getMockedAllMedia(t)

	baseMediaCache := anilist.NewBaseMediaCache()
	anizipCache := anizip.NewCache()
	anilistClientWrapper := anilist.MockAnilistClientWrapper()
	anilistRateLimiter := limiter.NewAnilistLimiter()
	logger := util.NewLogger()

	tests := []struct {
		name            string
		paths           []string
		expectedMediaId int
	}{
		{
			name: "should be hydrated with id 131586",
			paths: []string{
				"E:/Anime/[SubsPlease] 86 - Eighty Six (01-23) (1080p) [Batch]/[SubsPlease] 86 - Eighty Six - 20v2 (1080p) [30072859].mkv",
				"E:/Anime/[SubsPlease] 86 - Eighty Six (01-23) (1080p) [Batch]/[SubsPlease] 86 - Eighty Six - 21v2 (1080p) [4B1616A5].mkv",
				"E:/Anime/[SubsPlease] 86 - Eighty Six (01-23) (1080p) [Batch]/[SubsPlease] 86 - Eighty Six - 22v2 (1080p) [58BF43B4].mkv",
				"E:/Anime/[SubsPlease] 86 - Eighty Six (01-23) (1080p) [Batch]/[SubsPlease] 86 - Eighty Six - 23v2 (1080p) [D94B4894].mkv",
			},
			expectedMediaId: 131586, // 86 - Eighty Six Part 2
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			scanLogger, err := NewConsoleScanLogger()
			if err != nil {
				t.Fatal("expected result, got error:", err.Error())
			}

			// +---------------------+
			// |   Local Files       |
			// +---------------------+

			var lfs []*entities.LocalFile
			for _, path := range tt.paths {
				lf := entities.NewLocalFile(path, "E:/Anime")
				lfs = append(lfs, lf)
			}

			// +---------------------+
			// |   MediaContainer    |
			// +---------------------+

			mc := NewMediaContainer(&MediaContainerOptions{
				AllMedia:   allMedia,
				ScanLogger: scanLogger,
			})

			for _, nm := range mc.NormalizedMedia {
				t.Logf("media id: %d, title: %s", nm.ID, nm.GetTitleSafe())
			}

			// +---------------------+
			// |      Matcher        |
			// +---------------------+

			matcher := &Matcher{
				LocalFiles:     lfs,
				MediaContainer: mc,
				BaseMediaCache: nil,
				Logger:         util.NewLogger(),
				ScanLogger:     scanLogger,
			}

			err = matcher.MatchLocalFilesWithMedia()
			if err != nil {
				t.Fatal("expected result, got error:", err.Error())
			}

			// +---------------------+
			// |   FileHydrator      |
			// +---------------------+

			fh := FileHydrator{
				LocalFiles:           lfs,
				AllMedia:             mc.NormalizedMedia,
				BaseMediaCache:       baseMediaCache,
				AnizipCache:          anizipCache,
				AnilistClientWrapper: anilistClientWrapper,
				AnilistRateLimiter:   anilistRateLimiter,
				Logger:               logger,
				ScanLogger:           scanLogger,
			}

			fh.HydrateMetadata()

			for _, lf := range fh.LocalFiles {
				if lf.MediaId != tt.expectedMediaId {
					t.Fatalf("expected media id %d, got %d", tt.expectedMediaId, lf.MediaId)
				}

				t.Logf("local file: %s,\nmedia id: %d\n", lf.Name, lf.MediaId)
			}

		})
	}

}

// TestSpecialCase will test special cases where hydration fails in some way.
//   - This uses a real AniList collection query, so add necessary media to the dummy account.
func TestSpecialCase(t *testing.T) {

	baseMediaCache := anilist.NewBaseMediaCache()
	anizipCache := anizip.NewCache()
	_, anilistClientWrapper, userData := anilist.MockAnilistClientWrappers()
	anilistRateLimiter := limiter.NewAnilistLimiter()
	logger := util.NewLogger()

	tests := []struct {
		name            string
		paths           []string
		expectedMediaId int
	}{
		{
			name: "should be hydrated with id 31",
			paths: []string{
				"E:\\ANIME\\Neon Genesis Evangelion Death & Rebirth\\[Anime Time] Neon Genesis Evangelion - Death (True)².mkv",
				"E:\\ANIME\\Neon Genesis Evangelion Death & Rebirth\\[Anime Time] Neon Genesis Evangelion - Rebirth.mkv",
			},
			expectedMediaId: 31, // Neon Genesis Evangelion: Death & Rebirth
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			scanLogger, err := NewConsoleScanLogger()
			if err != nil {
				t.Fatal("expected result, got error:", err.Error())
			}

			// +---------------------+
			// |   Local Files       |
			// +---------------------+

			var lfs []*entities.LocalFile
			for _, path := range tt.paths {
				lf := entities.NewLocalFile(path, "E:/Anime")
				lfs = append(lfs, lf)
			}

			// +---------------------+
			// |   MediaFetcher      |
			// +---------------------+

			mf, err := NewMediaFetcher(&MediaFetcherOptions{
				Enhanced:             false,
				Username:             userData.Username2,
				AnilistClientWrapper: anilistClientWrapper,
				LocalFiles:           lfs,
				BaseMediaCache:       baseMediaCache,
				AnizipCache:          anizipCache,
				Logger:               logger,
				AnilistRateLimiter:   anilistRateLimiter,
				ScanLogger:           scanLogger,
				UseAnilistCollection: true,
			})

			// +---------------------+
			// |   MediaContainer    |
			// +---------------------+

			mc := NewMediaContainer(&MediaContainerOptions{
				AllMedia:   mf.AllMedia,
				ScanLogger: scanLogger,
			})

			for _, nm := range mc.NormalizedMedia {
				t.Logf("media id: %d, title: %s", nm.ID, nm.GetTitleSafe())
			}

			// +---------------------+
			// |      Matcher        |
			// +---------------------+

			matcher := &Matcher{
				LocalFiles:     lfs,
				MediaContainer: mc,
				BaseMediaCache: nil,
				Logger:         util.NewLogger(),
				ScanLogger:     scanLogger,
			}

			err = matcher.MatchLocalFilesWithMedia()
			if err != nil {
				t.Fatal("expected result, got error:", err.Error())
			}

			// +---------------------+
			// |   FileHydrator      |
			// +---------------------+

			fh := FileHydrator{
				LocalFiles:           lfs,
				AllMedia:             mc.NormalizedMedia,
				BaseMediaCache:       baseMediaCache,
				AnizipCache:          anizipCache,
				AnilistClientWrapper: anilistClientWrapper,
				AnilistRateLimiter:   anilistRateLimiter,
				Logger:               logger,
				ScanLogger:           scanLogger,
			}

			fh.HydrateMetadata()

			for _, lf := range fh.LocalFiles {
				if lf.MediaId != tt.expectedMediaId {
					t.Fatalf("expected media id %d, got %d", tt.expectedMediaId, lf.MediaId)
				}

				t.Logf("local file: %s,\nmedia id: %d\n", lf.Name, lf.MediaId)
			}

		})
	}

}
