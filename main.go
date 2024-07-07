package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
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
	buffer           *beep.Buffer
	volume           *effects.Volume
	control          *beep.Ctrl
	currentMusicName string
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

func playSong(musicQueue *Queue) {
	music := musicQueue.Dequeue()
	currentMusicName = music
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
	loopedStreamer := beep.Loop(1, control)
	volume = &effects.Volume{
		Streamer: loopedStreamer,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}
	speaker.Play(volume)
	cliRender(musicQueue)
}

func cliRender(musicQueue *Queue) {
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
				if pos.Position >= buffer.Len() {
					speaker.Clear()
					playSong(musicQueue)
					return
				}
				draw(pos, buffer, volume)
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
					}

				}
			}
		}
	}()

	select {}
}

func draw(pos *Positioner, buffer *beep.Buffer, volume *effects.Volume) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	printVolume := 0.0
	if volume.Volume > 0 {
		printVolume = float64(volume.Volume*25) + 50
	} else {
		printVolume = float64((4 - (-1)*volume.Volume) * 12.5)
	}
	positionStr := fmt.Sprintf("Position: %dm%ds / %dm%ds", pos.Position/pos.Format.SampleRate.N(time.Minute), (pos.Position/pos.Format.SampleRate.N(time.Second))%60, buffer.Len()/pos.Format.SampleRate.N(time.Minute), (buffer.Len()/pos.Format.SampleRate.N(time.Second))%60)
	volumeStr := fmt.Sprintf("Volume: %.1f", printVolume)
	volumeUpDown := "Volume Up: ↑, Volume Down: ↓"
	positionLeftRight := "Backward: ← , Forward: → "
	pause := "Pause: Space"
	exit := "Exit: Esc"

	for i, c := range currentMusicName {
		termbox.SetCell(i, 1, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range positionStr {
		termbox.SetCell(i, 2, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range volumeStr {
		termbox.SetCell(i, 3, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range volumeUpDown {
		termbox.SetCell(i, 6, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range positionLeftRight {
		termbox.SetCell(i, 7, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range pause {
		termbox.SetCell(i, 8, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range exit {
		termbox.SetCell(i, 9, c, termbox.ColorDefault, termbox.ColorDefault)
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

func main() {
	musicQueue := Queue{}

	files, err := os.ReadDir(musicPath)
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
	Shuffle(musics)

	for _, music := range musics {
		if music[len(music)-1] != '/' {
			musicQueue.Enqueue(musicPath + "/" + music)
		} else {

			dirPath := musicPath + "/" + music
			f, err := os.ReadDir(dirPath)
			if err != nil {
				log.Fatal(err)
			}
			for _, f1 := range f {
				if !f1.IsDir() {

					fullPath := dirPath + f1.Name()
					fmt.Println(fullPath)
					musicQueue.Enqueue(fullPath)
				}
			}
		}
	}
	err = termbox.Init()
	if err != nil {
		log.Fatalf("termbox.Init hata: %v", err)
	}
	defer termbox.Close()
	playSong(&musicQueue)
}
