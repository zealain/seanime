package playbackmanager

import (
	"errors"
	"github.com/seanime-app/seanime/internal/anilist"
	"github.com/seanime-app/seanime/internal/entities"
	"path/filepath"
	"strings"
)

//func (p *PlaybackManager) getLocalFileFromFilename(filename string, lfs []*entities.LocalFile) (*entities.LocalFile, bool) {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//	lf, ok := p.localFilesMap[path]
//	if !ok {
//		return nil, false
//	}
//
//	return p.anilistCollection.GetListEntryFromMediaId(lf.MediaId)
//}

// getListEntryFromLocalFilePath returns the list entry from the given entities.LocalFile path.
// This method should be called once everytime a new video is played
func (pm *PlaybackManager) getListEntryFromLocalFilePath(path string) (*anilist.MediaListEntry, *entities.LocalFile, *entities.LocalFileWrapperEntry, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// Normalize path
	path = filepath.ToSlash(strings.ToLower(path))

	// Find the local file from the path
	lfs, _, err := pm.Database.GetLocalFiles()
	if err != nil {
		return nil, nil, nil, errors.New("error getting local files")
	}

	var lf *entities.LocalFile
	// Find the local file from the path
	for _, l := range lfs {
		if l.GetNormalizedPath() == path {
			lf = l
			break
		}
	}
	// If the local file is not found, the path might be a filename (in the case of VLC)
	if lf == nil {
		for _, l := range lfs {
			if strings.ToLower(l.Name) == path {
				lf = l
				break
			}
		}
	}

	if lf == nil {
		return nil, nil, nil, errors.New("local file not found")
	}
	if lf.MediaId == 0 {
		return nil, nil, nil, errors.New("local file has not been matched")
	}

	ret, ok := pm.anilistCollection.GetListEntryFromMediaId(lf.MediaId)
	if !ok {
		return nil, nil, nil, errors.New("anilist list entry not found")
	}

	// Create local file wrapper
	lfw := entities.NewLocalFileWrapper(lfs)
	lfe, ok := lfw.GetLocalEntryById(lf.MediaId)
	if !ok {
		return nil, nil, nil, errors.New("local file wrapper entry not found")
	}

	return ret, lf, lfe, nil
}
