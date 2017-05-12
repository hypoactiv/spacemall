package main

import (
	"fmt"
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/generate"
	"jds/runstat"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	sdl_image "github.com/veandco/go-sdl2/sdl_image"
)

const (
	WIDTH    = 800
	HEIGHT   = 600
	TICKRATE = 10 // Target World ticks/second
)

var logstep int
var exit bool

type Tileset struct {
	Scale float32
	t     *sdl.Texture
	// Cell width and height
	w int
	h int
}

func LoadTileset(r *sdl.Renderer, f string) (*Tileset, error) {
	s, err := sdl_image.Load(f)
	if err != nil {
		return nil, err
	}
	defer s.Free()
	s.SetColorKey(1, sdl.MapRGB(s.Format, 255, 255, 255))
	t, err := r.CreateTextureFromSurface(s)
	ts := &Tileset{
		t: t,
	}
	ts.w = int(s.W / 16)
	ts.h = int(s.H / 16)
	return ts, nil
}

// With renderer r, draw character c, at position (x,y), with specified color
func (t *Tileset) Draw(r *sdl.Renderer, c uint8, x, y int, color *sdl.Color, scale float32) {
	src := &sdl.Rect{
		X: int32(c%10) * int32(t.w),
		Y: int32(c/10) * int32(t.h),
		W: int32(t.w),
		H: int32(t.h),
	}
	dst := &sdl.Rect{
		X: int32(x),
		Y: int32(y),
		W: int32(float32(t.w) * scale),
		H: int32(float32(t.h) * scale),
	}
	if color == nil {
		color = &sdl.Color{
			R: 255,
			G: 255,
			B: 255,
			A: 255,
		}
	}
	r.SetDrawColor(color.R, color.G, color.B, color.A)
	r.FillRect(dst)
	r.Copy(t.t, src, dst)
}

type TileEngine struct {
	T  *Tileset
	R  *sdl.Renderer
	pf uint32
	w  *world.World
	// Dimensions of window in pixels
	winw, winh uint
	// Zoom scale
	Scale float32
	// Top left of window in world coordinates
	tl game.Location
	// Layers
	background   *RenderLayer
	overlay      *RenderLayer
	layerError   *RenderLayer
	overlayColor game.Color
	Overlay      *layer.Layer
	// Layers to be drawn
	layers []*RenderLayer
}

func NewTileEngine(tileset string, W *world.World, w, h uint) (te *TileEngine, err error) {
	R, pf := sdlInit()
	ts, err := LoadTileset(R, tileset)
	if err != nil {
		return nil, err
	}
	te = &TileEngine{
		T:            ts,
		R:            R,
		w:            W,
		pf:           pf,
		winw:         w,
		winh:         h,
		Overlay:      layer.NewLayer(),
		overlayColor: colorRed,
		Scale:        0.5,
	}
	te.background = te.NewRenderLayer(&renderBackground{te})
	te.overlay = te.NewRenderLayer(&renderOverlay{te})
	return
}

// Renders a w-by-h pixel view, with top left corner at tl
func (te *TileEngine) Render() {
	defer runstat.Record(time.Now(), "Render")
	layers := []*RenderLayer{te.background, te.overlay}
	if te.layerError != nil {
		layers = []*RenderLayer{te.layerError}
	}
	x := -int(te.tl.X) * int(te.T.w)
	y := -int(te.tl.Y) * int(te.T.h)
	blockWidth := uint(float32(te.T.w*game.BLOCK_SIZE) * te.Scale)
	blockHeight := uint(float32(te.T.h*game.BLOCK_SIZE) * te.Scale)
	bi := te.tl.BlockId
	for i := uint(0); i < te.winw+blockWidth; i += blockWidth {
		bj := bi
		for j := uint(0); j < te.winh+blockHeight; j += blockHeight {
			for _, l := range layers {
				t := l.Render(bj)
				if t != nil {
					te.R.Copy(t, nil, &sdl.Rect{
						X: int32(x) + int32(i),
						Y: int32(y) + int32(j),
						W: int32(blockWidth),
						H: int32(blockHeight),
					})
				}
			}
			bj = bj.DownBlock()
		}
		bi = bi.RightBlock()
	}
	// Render entities
	for _, e := range te.w.Entities {
		l := e.Location()
		x, y := te.WorldToScreen(l)
		if x < 0 || y < 0 || x >= WIDTH || y >= HEIGHT {
			continue
		}
		ec := toSDLColor(e.Color())
		te.T.Draw(te.R, 30, x, y, &ec, te.Scale)
	}
	te.R.Present()
}

func toSDLColor(c game.Color) sdl.Color {
	return sdl.Color{
		R: c.R,
		G: c.G,
		B: c.B,
		A: c.A,
	}
}

func (te *TileEngine) SetTopLeft(tl game.Location) {
	te.tl = tl
}

// Screen coordinates to world coordinates
func (te *TileEngine) ScreenToWorld(x, y int) game.Location {
	relx := float32(x) / (te.Scale * float32(te.T.w))
	rely := float32(y) / (te.Scale * float32(te.T.h))
	return te.tl.JustOffset(int(relx), int(rely))
}

func (te *TileEngine) WorldToScreen(l game.Location) (x, y int) {
	x, y = te.tl.SmallDistance(l)
	return int(float32(x*te.T.w) * te.Scale), int(float32(y*te.T.h) * te.Scale)
}

// Interface overlay rendering
type renderOverlay struct {
	te *TileEngine
}

func (r *renderOverlay) ShouldRender(bid game.BlockId) bool {
	return r.te.Overlay.InBlockstore(bid)
}

func (r *renderOverlay) RenderBlock(bid game.BlockId) {
	defer runstat.Record(time.Now(), "renderOverlay")
	r.te.R.SetDrawColor(0, 0, 0, 0)
	r.te.R.Clear()
	r.te.R.SetDrawColor(
		r.te.overlayColor.R,
		r.te.overlayColor.G,
		r.te.overlayColor.B,
		r.te.overlayColor.A,
	)
	for l := range bid.Iterate() {
		if r.te.Overlay.Get(l) != 0 {
			r.te.R.FillRect(&sdl.Rect{
				X: int32(int(l.X) * r.te.T.w),
				Y: int32(int(l.Y) * r.te.T.h),
				H: int32(r.te.T.h),
				W: int32(r.te.T.w),
			})
		}
	}
}

// Layer error report rendering
type renderLayerError struct {
	te *TileEngine
	l  *layer.Layer
}

func (r renderLayerError) RenderBlock(bid game.BlockId) {
	l := game.Location{
		BlockId: bid,
	}
	for x := int8(0); x < game.BLOCK_SIZE; x++ {
		for y := int8(0); y < game.BLOCK_SIZE; y++ {
			l.X = x
			l.Y = y
			if rid := r.l.Get(l); rid != 0 {
				r.te.T.Draw(r.te.R, 224, int(x)*r.te.T.w, int(y)*r.te.T.h, IntToColor(int(rid)), 1)
			}
		}
	}
}

// Background layer rendering
type renderBackground struct {
	te *TileEngine
}

func IntToColor(r int) *sdl.Color {
	return &sdl.Color{
		R: uint8(16 * (r & 0x3)),
		G: uint8(16 * ((r >> 2) & 0x7)),
		B: uint8(16 * ((r >> 5) & 0x3)),
		A: 255,
	}
}

func (r *renderBackground) RenderBlock(bid game.BlockId) {
	defer runstat.Record(time.Now(), "renderBackground")
	tile := uint8(0)
	r.te.R.SetDrawColor(0, 0, 0, 255)
	r.te.R.Clear()
	var color *sdl.Color
	for l := range bid.Iterate() {
		switch r.te.w.Walls.Get(l) {
		case 1:
			if r.te.w.DoorIds.Get(l) != 0 {
				tile = 41
			} else {
				tile = 40
			}
			color = nil
		default:
			if rid := r.te.w.RoomIds.Get(l); rid != 0 {
				tile = 224
				color = IntToColor(int(rid))
			} else {
				tile = 0
				color = nil
			}
		}
		if tile != 0 {
			r.te.T.Draw(r.te.R, tile, int(l.X)*r.te.T.w, int(l.Y)*r.te.T.h, color, 1)
		}
	}
}

type BlockRenderer interface {
	RenderBlock(game.BlockId)
}

type RenderOracle interface {
	// Return false if rendering this block would produce a transparent texture
	ShouldRender(game.BlockId) bool
}

type RenderLayer struct {
	// Texture cache
	t      map[game.BlockId]*renderCacheEntry
	c      BlockRenderer
	te     *TileEngine
	oracle func(game.BlockId) bool
}

type renderCacheEntry struct {
	t           *sdl.Texture
	needsUpdate bool
}

func (te *TileEngine) NewRenderLayer(c BlockRenderer) *RenderLayer {
	rl := &RenderLayer{
		t:  make(map[game.BlockId]*renderCacheEntry),
		c:  c,
		te: te,
	}
	if c, ok := c.(RenderOracle); ok {
		rl.oracle = c.ShouldRender
	} else {
		rl.oracle = nil
	}
	return rl
}

func (rl *RenderLayer) Render(bid game.BlockId) (t *sdl.Texture) {
	if rl.oracle != nil && rl.oracle(bid) == false {
		// Oracle says there's no point in rendering, so don't
		return nil
	}
	rce := rl.t[bid]
	if rce == nil {
		func() {
			defer runstat.Record(time.Now(), "Create Texture")
			var err error
			t, err = rl.te.R.CreateTexture(
				rl.te.pf,
				sdl.TEXTUREACCESS_TARGET,
				rl.te.T.w*game.BLOCK_SIZE,
				rl.te.T.h*game.BLOCK_SIZE,
			)
			t.SetBlendMode(sdl.BLENDMODE_BLEND)
			if err != nil {
				panic(err)
			}
			rce = &renderCacheEntry{
				t:           t,
				needsUpdate: true,
			}
			rl.t[bid] = rce
		}()
	}
	t = rce.t
	if rce.needsUpdate {
		err := rl.te.R.SetRenderTarget(t)
		if err != nil {
			panic(err)
		}
		rl.te.R.SetDrawColor(0, 0, 0, 255)
		rl.te.R.Clear()
		// Call render callback
		rl.c.RenderBlock(bid)
		err = rl.te.R.SetRenderTarget(nil)
		if err != nil {
			panic(err)
		}
		rce.needsUpdate = false
	}
	return
}

func (rl *RenderLayer) UpdateAll() {
	for _, v := range rl.t {
		v.needsUpdate = true
	}
}

func (rl *RenderLayer) UpdateBulk(m game.ModMap) {
	for k := range m.Blocks() {
		if _, ok := rl.t[k]; ok {
			rl.t[k].needsUpdate = true
		}
	}
}

func (rl *RenderLayer) Update(bid game.BlockId) {
	if rce := rl.t[bid]; rce != nil {
		rce.needsUpdate = true
	}
}

func handleSigint() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			fmt.Println("sigint caught")
			if exit == true {
				pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
				panic("KABOOM BABY")
			}
			exit = true
		}
	}()
}

func sdlInit() (*sdl.Renderer, uint32) {
	var err error
	chkerr := func() {
		if err != nil {
			panic(err)
		}
	}
	err = sdl.Init(sdl.INIT_VIDEO)
	chkerr()
	w, err := sdl.CreateWindow(
		"space mall",
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		WIDTH,
		HEIGHT,
		0,
	)
	chkerr()
	pf, err := w.GetPixelFormat()
	chkerr()
	r, err := sdl.CreateRenderer(w, -1, 0)
	chkerr()
	r.SetDrawColor(0, 0, 0, 255)
	r.Clear()
	r.Present()
	return r, pf
}

func startProfile() {
	f, err := os.Create("cpu.pprof")
	if err != nil {
		panic(err)
	}
	pprof.StartCPUProfile(f)
}

func (te *TileEngine) Interactive() (err interface{}) {
	defer func() {
		err = recover()
		if err != nil {
			fmt.Println(err)
			debug.PrintStack()
		}
	}()
	var last game.Location
	var toolMode int
	tool := toolset[toolMode].Create(te.w)
	lastStats := time.Now()
	lastStatTick := te.w.Now()
	te.w.Think()
	for !exit {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {
			case *sdl.MouseButtonEvent:
				if event.Button == 1 && event.State == 1 {
					//logEnc.Encode(reflect.TypeOf(event).String())
					//logEnc.Encode(*event)
					l := te.ScreenToWorld(int(event.X), int(event.Y))
					te.background.UpdateBulk(tool.Click(l))
				} else if event.Button == 3 && event.State == 1 {
					l := te.ScreenToWorld(int(event.X), int(event.Y))
					te.background.UpdateBulk(tool.RightClick(l))
				}
			case *sdl.MouseMotionEvent:
				l := te.ScreenToWorld(int(event.X), int(event.Y))
				if l != last {
					fmt.Println("Cursor location:", l)
					last = l
					c, color := tool.Preview(l)
					te.overlayColor = color
					te.Overlay.Discard()
					for l = range c {
						te.Overlay.Set(l, 1)
					}
					te.overlay.UpdateAll()
				}
			case *sdl.KeyDownEvent:
				//logEnc.Encode(reflect.TypeOf(event).String())
				//logEnc.Encode(*event)
				if event.Keysym.Sym == sdl.K_SPACE {
					toolMode = (toolMode + 1) % len(toolset)
					tool = toolset[toolMode].Create(te.w)
					fmt.Println("tool mode", toolMode, toolset[toolMode].Name)
					te.Overlay.Discard()
					te.overlay.UpdateAll()
				} else if event.Keysym.Sym == sdl.K_f {
					return
				}
			default:
				//fmt.Printf("event type %T\n", event)
			}
		}
		startFrame := time.Now()
		te.Render()
		// Think at least once per frame, and possibly more to hit TICKRATE
		// Target
		mult := 1
		//startThink := time.Now()
		te.w.Think()
		for time.Since(startFrame) < 50*time.Millisecond && mult < TICKRATE/20 {
			te.w.Think()
			mult++
		}
		if time.Since(lastStats) > 1*time.Second {
			ticks := te.w.Now() - lastStatTick
			fmt.Println("Avg workers per tick:", float32(te.w.ThinkStats.Workers)/float32(ticks))
			fmt.Println("Avg Actions per worker:", float32(te.w.ThinkStats.Actions)/float32(te.w.ThinkStats.Workers))
			fmt.Println("Avg time per Think:", te.w.ThinkStats.Elapsed/time.Duration(ticks))
			fmt.Println("Avg Actions per second:", float64(te.w.ThinkStats.Actions)/te.w.ThinkStats.Elapsed.Seconds())
			te.w.ThinkStats.Actions = 0
			te.w.ThinkStats.Workers = 0
			te.w.ThinkStats.Elapsed = 0
			lastStatTick = te.w.Now()
			lastStats = time.Now()
		}
	}
	return
}

func (te *TileEngine) ShowLayerError(le world.LayerError) {
	te.background.UpdateAll()
	rLayerError := te.NewRenderLayer(renderLayerError{te, le.Layer})
errorDisplayLoop:
	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {
			case *sdl.KeyDownEvent:
				break errorDisplayLoop
				if event.Keysym.Sym == sdl.K_SPACE {
				}
			}
		}
		te.layerError = rLayerError
		te.Render()
		time.Sleep(1 * time.Second)
		te.layerError = nil
		te.Render()
		time.Sleep(1 * time.Second)
	}
}

func FuzzAndDebug() {
	//logFile, err := os.OpenFile(time.Now().Format("2006.01.02.15.04.05.log"), os.O_CREATE|os.O_RDWR, 0644)
	//if err != nil {
	//	panic("could not open log file")
	//}
	//logGzip := gzip.NewWriter(logFile)
	//logEnc := gob.NewEncoder(logGzip)
	//defer logGzip.Close()
	//w := world.NewWorld(0)
	w := generate.NewGridWorld(50, 50) //world.NewWorld(0) //world.STRICT_FSCK_EVERY_OP)
	te, err := NewTileEngine("tileset.png", w, WIDTH, HEIGHT)
	rand.Seed(int64(time.Now().Nanosecond()))
	if err != nil {
		panic(err)
	}
	/*lastLen := 0
	for len(w.Rooms) < 200 {
		a := te.RandomLocation(50)
		b := te.RandomLocation(50)
		w.Drawbox(a, b)
		if len(w.Rooms) == lastLen && lastLen > 50 {
			break
		}
		lastLen = len(w.Rooms)
	}
	for len(w.Rooms) > 3 {
		var r *world.Room
		for _, r = range w.Rooms {
		}
		w.DeleteFromWallTree(r.LNPL())
	}
	w.Drawbox(te.ScreenToWorld(0, 0), te.ScreenToWorld(WIDTH, HEIGHT))*/
	for !exit {
		if len(os.Args) > 1 {
			fuzzError := te.Fuzz(-1)
			if fuzzError != nil {
				fmt.Println(fuzzError)
				fmt.Println("last op", world.LastOp)
				if fuzzError, ok := fuzzError.(world.LayerError); ok {
					te.ShowLayerError(fuzzError)
				}
				te.background.UpdateAll()
			}
			f, err := os.Create("mem.pprof")
			if err != nil {
				panic(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}
		err := te.Interactive()
		if err != nil {
			fmt.Println(err)
			fmt.Println("last op", world.LastOp)
			if err, ok := err.(world.LayerError); ok {
				te.ShowLayerError(err)
			}
			te.background.UpdateAll()
		}
	}
}

func main() {
	handleSigint()
	startProfile()
	defer pprof.StopCPUProfile()
	FuzzAndDebug()
	metricNames := make([]string, 0, len(runstat.Metrics))
	for k := range runstat.Metrics {
		metricNames = append(metricNames, k)
	}
	sort.Strings(metricNames)
	// runstat metrics
	for _, k := range metricNames {
		fmt.Println(k, runstat.Metrics[k].Average(), "max:", runstat.Metrics[k].Max, "total:", runstat.Metrics[k].Total)
	}

}
