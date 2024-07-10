package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/nsf/termbox-go"
)

const musicPath = "./musics"

var (
	pos              *Positioner
	nextPos          *Positioner
	buffer           *beep.Buffer
	nextBuffer       *beep.Buffer
	volume           *effects.Volume
	nextVolume       *effects.Volume
	control          *beep.Ctrl
	nextControl      *beep.Ctrl
	currentMusicName string
	shuffle          bool
	selectedPlaylist int
	prevMusicQueue   Queue
	musicQueue       Queue
	isPreLoading     bool
	callPlaySong     bool
)

type Queue struct {
	items []string
}

func (q *Queue) Enqueue(item string) {
	q.items = append(q.items, item)
}

func (q *Queue) Dequeue() string {
	if len(q.items) == 0 {
		return ""
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

type Volume struct {
	Streamer beep.Streamer
	Base     float64
	Volume   float64
	Silent   bool
}

type Positioner struct {
	Streamer beep.Streamer
	Position int
	Format   beep.Format
	Buffer   *beep.Buffer
}

func (p *Positioner) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = p.Streamer.Stream(samples)
	if ok {
		p.Position += n
	}
	return n, ok
}

func (p *Positioner) Err() error {
	return p.Streamer.Err()
}

func (p *Positioner) Seek(pos int) {
	p.Streamer = p.Buffer.Streamer(pos, p.Buffer.Len())
	p.Position = pos
}

func pathToMusicName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return ""
}

func preloadNextSong() {
	if musicQueue.items == nil || len(musicQueue.items) == 0 {
		return
	}
	isPreLoading = true
	music := musicQueue.items[0]
	f, err := os.Open(string(music))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	streamer2, nextFormat, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer2.Close()

	nextBuffer = beep.NewBuffer(nextFormat)
	nextBuffer.Append(streamer2)

	nextPos = &Positioner{
		Streamer: nextBuffer.Streamer(0, nextBuffer.Len()),
		Format:   nextFormat,
		Buffer:   nextBuffer,
	}
	nextControl := &beep.Ctrl{Streamer: nextPos, Paused: false}
	volumeValue := 0.0
	if volume != nil {
		volumeValue = volume.Volume
	}
	nextVolume = &effects.Volume{
		Streamer: nextControl,
		Base:     2,
		Volume:   volumeValue,
		Silent:   false,
	}
	isPreLoading = false
	if callPlaySong {
		callPlaySong = false
		playSong(false)
	}

}

func playSong(prevOrNext bool) { //if prev is true if next is false
	if isPreLoading {
		callPlaySong = true
		return
	}
	if nextBuffer != nil && !prevOrNext {
		speaker.Clear()
		music := musicQueue.Dequeue()
		prevMusicQueue.Enqueue(music)
		buffer = nextBuffer
		pos = nextPos
		control = nextControl
		volume = nextVolume
		format := buffer.Format()

		nextBuffer = nil
		nextPos = nil
		nextControl = nil
		nextVolume = nil
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		speaker.Play(volume)
		go preloadNextSong()

		go musicInfoRender()

		return
	}

	music := musicQueue.Dequeue()
	prevMusicQueue.Enqueue(music)
	currentMusicName = pathToMusicName(music)
	f, err := os.Open(string(music))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()

	buffer = beep.NewBuffer(format)
	buffer.Append(streamer)

	pos = &Positioner{
		Streamer: buffer.Streamer(0, buffer.Len()),
		Format:   format,
		Buffer:   buffer,
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	control = &beep.Ctrl{Streamer: pos, Paused: false}
	volumeValue := 0.0
	if volume != nil {
		volumeValue = volume.Volume
	}
	volume = &effects.Volume{
		Streamer: control,
		Base:     2,
		Volume:   volumeValue,
		Silent:   false,
	}
	speaker.Play(volume)

	go preloadNextSong()

	go musicInfoRender()
}

func musicInfoRender() {
	eventQueue := make(chan termbox.Event)
	go func() {
		for {
			eventQueue <- termbox.PollEvent()
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if buffer.Len()-pos.Position < pos.Format.SampleRate.N(time.Second*1) {
					if musicQueue.items == nil || len(musicQueue.items) == 0 {
						maintainWelcomePage(getPlaylists())
						return
					}
					playSong(false)
				}
				drawMusicInfo(pos, buffer, volume)
			case ev := <-eventQueue:
				if ev.Type == termbox.EventKey {
					switch ev.Key {
					case termbox.KeyEsc:
						speaker.Clear()
						os.Exit(0)
						return
					case termbox.KeySpace:
						control.Paused = !control.Paused
					case termbox.KeyArrowRight:
						newPos := pos.Position + pos.Format.SampleRate.N(time.Second*10)
						if newPos < buffer.Len() {
							pos.Seek(newPos)
						}
					case termbox.KeyArrowLeft:
						newPos := pos.Position - pos.Format.SampleRate.N(time.Second*10)
						if newPos > 0 {
							pos.Seek(newPos)
						} else {
							pos.Seek(0)
						}
					case termbox.KeyArrowUp:
						if volume.Silent {
							volume.Silent = false
						}
						if volume.Volume >= 2 {
							break
						}
						volume.Volume += 0.1
					case termbox.KeyArrowDown:
						if volume.Volume <= -4.0 {
							volume.Silent = true
							break
						}
						volume.Volume -= 0.1
					case termbox.KeyCtrlR:
						speaker.Clear()
						maintainWelcomePage(getPlaylists())
						return
					case termbox.KeyCtrlX:
						if len(musicQueue.items) > 0 {
							speaker.Clear()
							playSong(false)
						}
					case termbox.KeyCtrlZ:
						if len(prevMusicQueue.items) > 0 {
							var itemsToPrepend []string
							if pos.Position < pos.Format.SampleRate.N(time.Second*10) {
								itemsToPrepend = prevMusicQueue.items[len(prevMusicQueue.items)-2:]
							} else {
								itemsToPrepend = prevMusicQueue.items[len(prevMusicQueue.items)-1:]
							}

							musicQueue.items = append(itemsToPrepend, musicQueue.items...)
							speaker.Clear()
							playSong(true)
						}

					}

				}
			}
		}
	}()

	select {}
}

func drawMusicInfo(pos *Positioner, buffer *beep.Buffer, volume *effects.Volume) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	printVolume := 0.0
	if volume.Volume > 0 {
		printVolume = float64(volume.Volume*25) + 50
	} else {
		printVolume = float64((4 - (-1)*volume.Volume) * 12.5)
	}
	positionStr := fmt.Sprintf("Position: %dm%ds / %dm%ds", pos.Position/pos.Format.SampleRate.N(time.Minute), (pos.Position/pos.Format.SampleRate.N(time.Second))%60, buffer.Len()/pos.Format.SampleRate.N(time.Minute), (buffer.Len()/pos.Format.SampleRate.N(time.Second))%60)
	volumeStr := fmt.Sprintf("Volume: %.1f", printVolume)
	prevOrNext := "Prev: Ctrl+Z, Next: Ctrl+X"
	volumeUpDown := "Volume Up: ↑, Volume Down: ↓"
	positionLeftRight := "Backward: ← , Forward: → "
	pause := "Pause: Space"
	mainPage := "Main Page: Ctrl+R"
	exit := "Exit: Esc"

	nextMusics := "Next Musics: "

	musicBarLength := 60
	currBarLength := int(float64(musicBarLength) * (float64(pos.Position) / float64(buffer.Len())))

	for i, c := range currentMusicName {
		termbox.SetCell(5+i, 1, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range positionStr {
		termbox.SetCell(5+i, 2, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range volumeStr {
		termbox.SetCell(5+i, 3, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	termbox.SetCell(5, 4, '[', termbox.ColorDefault, termbox.ColorDefault)
	for i := 0; i < currBarLength; i++ {
		termbox.SetCell(6+i, 4, '•', termbox.ColorGreen, termbox.ColorDefault)
	}
	for i := currBarLength; i < musicBarLength; i++ {
		termbox.SetCell(6+i, 4, '•', termbox.ColorDefault, termbox.ColorDefault)
	}
	termbox.SetCell(5+musicBarLength, 4, ']', termbox.ColorDefault, termbox.ColorDefault)
	for i, c := range prevOrNext {
		termbox.SetCell(5+i, 5, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range volumeUpDown {
		termbox.SetCell(5+i, 6, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range positionLeftRight {
		termbox.SetCell(5+i, 7, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range pause {
		termbox.SetCell(5+i, 8, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range mainPage {
		termbox.SetCell(5+i, 9, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range exit {
		termbox.SetCell(5+i, 10, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range nextMusics {
		termbox.SetCell(5+i, 12, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, k := range musicQueue.items {
		if i < 5 {
			k = pathToMusicName(k)
			for j, c := range k {
				termbox.SetCell(5+j, 13+i, c, termbox.ColorDefault, termbox.ColorDefault)
			}
		}
	}

	termbox.Flush()
}

func Shuffle(slice []string) {
	rand.Seed(time.Now().UnixNano())
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func addMusicToQueue(playlist string) {
	localMusicPath := musicPath + "/" + playlist
	files, err := os.ReadDir(localMusicPath)
	if err != nil {
		log.Fatal(err)
	}
	musics := make([]string, 0, len(files))
	for _, file := range files {
		fileName := file.Name()
		if file.IsDir() {
			fileName += "/"
		}
		musics = append(musics, fileName)
	}
	if shuffle {
		Shuffle(musics)
	}
	if len(musics) == 0 {
		maintainWelcomePage(getPlaylists())
	}
	for _, music := range musics {
		if music[len(music)-1] != '/' {
			musicQueue.Enqueue(localMusicPath + "/" + music)
		} else {
			dirPath := localMusicPath + "/" + music
			f, err := os.ReadDir(dirPath)
			if err != nil {
				log.Fatal(err)
			}
			for _, f1 := range f {
				if !f1.IsDir() {

					fullPath := dirPath + f1.Name()
					musicQueue.Enqueue(fullPath)
				}
			}
		}
	}
}

func drawWelcomePage(playlists []string, selectedPlaylist int) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.Clear(termbox.ColorBlue, termbox.ColorDefault)
	welcome := "Welcome to Music Player"
	pressEnter := "Press Enter to start"
	upDown := "Use Up and Down arrows to select playlist"
	exitInfo := "Press Esc to exit"
	playlistsInfo := "Playlists: "
	shuffleInfo := "Press Ctrl+S to shuffle. Shuffle:" + strconv.FormatBool(shuffle)

	for i, c := range welcome {
		termbox.SetCell(5+i, 1, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range pressEnter {
		termbox.SetCell(5+i, 2, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range shuffleInfo {
		termbox.SetCell(5+i, 3, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range exitInfo {
		termbox.SetCell(5+i, 4, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range upDown {
		termbox.SetCell(5+i, 5, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range playlistsInfo {
		termbox.SetCell(5+i, 7, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, playlist := range playlists {
		if selectedPlaylist == i {
			playlist += " <-"
			for j, c := range playlist {
				termbox.SetCell(5+j, 8+i, c, termbox.ColorBlue, termbox.ColorDefault)
			}
		} else {
			for j, c := range playlist {
				termbox.SetCell(5+j, 8+i, c, termbox.ColorDefault, termbox.ColorDefault)
			}
		}

	}
	termbox.Flush()
}

func getPlaylists() []string {
	files, err := os.ReadDir(musicPath)
	if err != nil {
		log.Fatal(err)
	}
	playlists := make([]string, 0, len(files))
	for _, file := range files {
		playlists = append(playlists, file.Name())
	}
	return playlists
}

func maintainWelcomePage(playlists []string) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.Clear(termbox.ColorBlue, termbox.ColorDefault)
	termbox.Flush()
	eventQueue := make(chan termbox.Event)
	go func() {
		for {
			eventQueue <- termbox.PollEvent()
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				drawWelcomePage(playlists, selectedPlaylist)
			case ev := <-eventQueue:
				if ev.Type == termbox.EventKey {
					switch ev.Key {
					case termbox.KeyArrowDown:
						if selectedPlaylist < len(playlists)-1 {
							selectedPlaylist++
						}
					case termbox.KeyArrowUp:
						if selectedPlaylist > 0 {
							selectedPlaylist--
						}
					case termbox.KeyEnter:
						addMusicToQueue(playlists[selectedPlaylist])
						playSong(false)
						ticker.Stop()
						return
					case termbox.KeyCtrlS:
						shuffle = !shuffle
					case termbox.KeyEsc:
						speaker.Clear()
						os.Exit(0)
						return
					case termbox.KeyCtrlC:
						speaker.Clear()
						os.Exit(0)
						return
					}

				}
			}
		}
	}()
	select {}

}

func main() {
	err := termbox.Init()
	if err != nil {
		log.Fatalf("termbox.Init hata: %v", err)
	}
	defer termbox.Close()
	musicQueue = Queue{}
	prevMusicQueue = Queue{}
	shuffle = false
	selectedPlaylist = 0

	playlists := getPlaylists()

	maintainWelcomePage(playlists)
}
