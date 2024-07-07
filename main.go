package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/nsf/termbox-go"
)

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

func main() {

	err := termbox.Init()
	if err != nil {
		log.Fatalf("termbox.Init hata: %v", err)
	}
	defer termbox.Close()

	f, err := os.Open("duality.mp3")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()

	buffer := beep.NewBuffer(format)
	buffer.Append(streamer)

	pos := &Positioner{
		Streamer: buffer.Streamer(0, buffer.Len()),
		Format:   format,
		Buffer:   buffer,
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	control := &beep.Ctrl{Streamer: pos, Paused: false}
	loopedStreamer := beep.Loop(1, control)
	volume := &effects.Volume{
		Streamer: loopedStreamer,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}
	speaker.Play(volume)

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
	positionLeftRight := "Backward 10 sec: ← , Forward: → "
	pause := "Pause: Space"
	exit := "Exit: Esc"

	for i, c := range positionStr {
		termbox.SetCell(i, 0, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range volumeStr {
		termbox.SetCell(i, 1, c, termbox.ColorDefault, termbox.ColorDefault)
	}

	for i, c := range volumeUpDown {
		termbox.SetCell(i, 2, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range positionLeftRight {
		termbox.SetCell(i, 3, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range pause {
		termbox.SetCell(i, 4, c, termbox.ColorDefault, termbox.ColorDefault)
	}
	for i, c := range exit {
		termbox.SetCell(i, 5, c, termbox.ColorDefault, termbox.ColorDefault)
	}

	termbox.Flush()
}
