package mpv

import (
	"errors"
	"github.com/jannson/mpvipc"
	"github.com/rs/zerolog"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

var (
	ErrConnClosed = errors.New("connection closed")
)

const (
	StartExecCommand = iota
	StartDetectPlayback
	StartExecPath
	StartExec
)

type (
	Playback struct {
		Filename  string
		Paused    bool
		Position  float64
		Duration  float64
		IsRunning bool
		Filepath  string
	}

	Mpv struct {
		Logger     *zerolog.Logger
		ExitCh     chan error
		CloseCh    chan struct{}
		Playback   *Playback
		SocketName string
		AppPath    string
		conn       *mpvipc.Connection
		isRunning  bool
		mu         sync.Mutex
		playbackMu sync.RWMutex
		process    *os.Process
	}
)

func New(logger *zerolog.Logger, socketName string, appPath string) *Mpv {
	if socketName == "" {
		socketName = getSocketName()
	}
	return &Mpv{
		Logger:     logger,
		ExitCh:     make(chan error),
		CloseCh:    make(chan struct{}),
		Playback:   &Playback{},
		mu:         sync.Mutex{},
		playbackMu: sync.RWMutex{},
		SocketName: socketName,
		AppPath:    appPath,
	}
}

func getSocketName() string {
	switch runtime.GOOS {
	case "windows":
		return "\\\\.\\pipe\\mpv_ipc"
	case "linux":
		return "/tmp/mpv_socket"
	case "darwin":
		return "/tmp/mpv_socket"
	default:
		return "/tmp/mpv_socket"
	}
}

func (m *Mpv) Play(filepath string, start int) error {

	// Open and play the file if not running
	if !m.isRunning {
		return m.OpenAndPlay(filepath, start)
	}

	// If running, just play the file
	//_, err := m.conn.Call("loadfile", filepath, "replace")
	//
	panic("not implemented")
}

func (m *Mpv) launchPlayer(start int, filePath string) error {
	var cmd *exec.Cmd

	switch start {
	case StartExecPath, StartExec:
		if m.AppPath == "" {
			return errors.New("mpv path is not set")
		}
		cmd = exec.Command(m.AppPath, "--input-ipc-server="+m.SocketName, filePath)
	default:
		cmd = exec.Command("mpv", "--input-ipc-server="+m.SocketName, filePath)
	}

	err := cmd.Start()
	if err != nil {
		return err
	}

	m.process = cmd.Process
	return nil
}

func (m *Mpv) OpenAndPlay(filePath string, start int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCh = make(chan struct{}, 1)
	m.ExitCh = make(chan error, 1)
	m.Playback = &Playback{}

	// Launch player
	if m.isRunning && m.process != nil {
		m.process.Kill()
	}
	err := m.launchPlayer(start, filePath)
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// Establish connection
	m.conn = mpvipc.NewConnection(m.SocketName)
	err = m.conn.Open()
	if err != nil {
		return err
	}

	m.isRunning = true
	m.Logger.Debug().Msg("mpv: Connection established")

	// Listen for events in a goroutine
	go func() {
		// Close the connection when the goroutine ends
		defer func() {
			m.Logger.Debug().Msg("mpv: Closing socket connection")
			m.ResetPlaybackStatus()
			m.isRunning = false
			m.conn.Close()
			if m.process != nil {
				m.process.Kill()
			}
			m.ExitCh <- ErrConnClosed
			m.Logger.Debug().Msg("mpv: Connection closed")
		}()

		events, stopListening := m.conn.NewEventListener()

		_, err = m.conn.Get("path")
		if err != nil {
			m.ExitCh <- err
			return
		}

		_, err = m.conn.Call("observe_property", 42, "time-pos")
		if err != nil {
			m.ExitCh <- err
			return
		}
		_, err = m.conn.Call("observe_property", 43, "pause")
		if err != nil {
			m.ExitCh <- err
			return
		}
		_, err = m.conn.Call("observe_property", 44, "duration")
		if err != nil {
			m.ExitCh <- err
			return
		}
		_, err = m.conn.Call("observe_property", 45, "filename")
		if err != nil {
			m.ExitCh <- err
			return
		}
		_, err = m.conn.Call("observe_property", 46, "path")
		if err != nil {
			m.ExitCh <- err
			return
		}

		// Listen for close event
		go func() {
			m.conn.WaitUntilClosed()
			stopListening <- struct{}{}
		}()

		// Listen for events
		for event := range events {
			m.Playback.IsRunning = true
			if event.Data != nil {
				//m.Logger.Trace().Msgf("received event: %s, %v, %+v", event.Name, event.ID, event.Data)
				switch event.ID {
				case 43:
					m.Playback.Paused = event.Data.(bool)
				case 42:
					m.Playback.Position = event.Data.(float64)
				case 44:
					m.Playback.Duration = event.Data.(float64)
				case 45:
					m.Playback.Filename = event.Data.(string)
				case 46:
					m.Playback.Filepath = event.Data.(string)
				}
			}
		}
	}()

	return nil
}

func (m *Mpv) GetPlaybackStatus() (*Playback, error) {
	m.playbackMu.RLock()
	defer m.playbackMu.RUnlock()
	if m.Playback.IsRunning == false {
		return nil, errors.New("mpv is not running")
	}
	if m.Playback == nil {
		return nil, errors.New("no playback status")
	}
	if m.Playback.Filename == "" {
		return nil, errors.New("no media found")
	}
	return m.Playback, nil
}

func (m *Mpv) ResetPlaybackStatus() {
	m.playbackMu.Lock()
	//m.Logger.Debug().Msg("mpv: resetting playback status")
	m.Playback.Filename = ""
	m.Playback.Filepath = ""
	m.Playback.Paused = false
	m.Playback.Position = 0
	m.Playback.Duration = 0
	m.playbackMu.Unlock()
	return
}

func (m *Mpv) Close() {
	m.conn.Close()
	m.ResetPlaybackStatus()
	m.isRunning = false
	if m.process != nil {
		m.process.Kill()
	}
	m.ExitCh <- ErrConnClosed
}
