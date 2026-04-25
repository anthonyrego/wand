package drumcircle

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/pkg/audio"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/ui"
)

const (
	maxEvents      = 32
	burstCount     = 30
	ringCount      = 20
	maxTrails      = 200
	accelCooldown  = 0.15 // seconds between hits
	gyroThreshold  = 25.0 // °/s to start trail emission
	eventLifetime  = 1.5  // seconds
	trailLifetime  = 0.8  // seconds
	ringExpandRate = 5.0  // units/s outward velocity
	burstDrag      = 0.96 // per-frame velocity multiplier
)

type particle struct {
	X, Y, Z    float32
	VX, VY, VZ float32
	Size       float32
	Age        float32
	R, G, B    uint8
}

type hitEvent struct {
	age              float32
	posX, posY, posZ float32
	r, g, b          uint8
	intensity        float32
	burst            []particle
	ring             []particle
}

type activeSound struct {
	freq      float32
	phase     float32
	age       float32
	amplitude float32
}

// 20-note minor pentatonic across 4 octaves (C3..A6). Face index (sorted by
// elevation then azimuth) selects which note plays on a hit.
var faceNotes = [20]float32{
	130.81, 146.83, 164.81, 196.00, 220.00, // C3..A3
	261.63, 293.66, 329.63, 392.00, 440.00, // C4..A4
	523.25, 587.33, 659.25, 783.99, 880.00, // C5..A5
	1046.50, 1174.66, 1318.51, 1567.98, 1760.00, // C6..A6
}

// 20 face centroids of a regular icosahedron, each paired with a rank into
// faceNotes (ascending by elevation, then azimuth). Built once at Init; used
// only for audio — the sphere isn't drawn.
type icosahedron struct {
	centroids [20]mgl32.Vec3
	noteIdx   [20]int
}

func buildIcosa() icosahedron {
	phi := float32(1.6180339887498949)
	raw := [12]mgl32.Vec3{
		{0, -1, -phi}, {0, 1, -phi}, {0, -1, phi}, {0, 1, phi},
		{-1, -phi, 0}, {1, -phi, 0}, {-1, phi, 0}, {1, phi, 0},
		{-phi, 0, -1}, {-phi, 0, 1}, {phi, 0, -1}, {phi, 0, 1},
	}
	var verts [12]mgl32.Vec3
	for i, v := range raw {
		verts[i] = v.Normalize()
	}
	faces := [20][3]int{
		{0, 1, 8}, {0, 8, 4}, {0, 4, 5}, {0, 5, 10}, {0, 10, 1},
		{1, 10, 7}, {1, 7, 6}, {1, 6, 8}, {8, 6, 9}, {8, 9, 4},
		{4, 9, 2}, {4, 2, 5}, {5, 2, 11}, {5, 11, 10}, {10, 11, 7},
		{7, 11, 3}, {7, 3, 6}, {6, 3, 9}, {9, 3, 2}, {2, 3, 11},
	}
	var ico icosahedron
	for i, f := range faces {
		c := verts[f[0]].Add(verts[f[1]]).Add(verts[f[2]]).Mul(1.0 / 3.0)
		ico.centroids[i] = c.Normalize()
	}

	order := make([]int, 20)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ca, cb := ico.centroids[order[a]], ico.centroids[order[b]]
		if math.Abs(float64(ca.Y()-cb.Y())) > 0.05 {
			return ca.Y() < cb.Y()
		}
		return math.Atan2(float64(ca.X()), float64(ca.Z())) < math.Atan2(float64(cb.X()), float64(cb.Z()))
	})
	for rank, faceI := range order {
		ico.noteIdx[faceI] = rank
	}
	return ico
}

func (ico *icosahedron) findFace(forward mgl32.Vec3) int {
	best, bestDot := 0, float32(-2.0)
	for i := range ico.centroids {
		d := ico.centroids[i].Dot(forward)
		if d > bestDot {
			bestDot = d
			best = i
		}
	}
	return best
}

type Game struct {
	wand  *wand.Listener
	pause *ui.PauseMenu
	time  float32

	ground *mesh.Mesh
	events []hitEvent
	trails []particle

	eventVB *sdl.GPUBuffer
	eventIB *sdl.GPUBuffer
	trailVB *sdl.GPUBuffer
	trailIB *sdl.GPUBuffer

	icosa icosahedron

	lastHitTime float32
	lastAccelMag float32

	// Audio
	stream *sdl.AudioStream
	sounds []activeSound
	mixBuf []float32

	// Tuning parameters
	hitThreshold    float32
	accelCooldown   float32
	maxAccel        float32
	visualExponent  float32
	audioExponent   float32
	gyroThreshold   float32

	wantsChange bool
	showDebug   bool
	debugMeshes [4]*mesh.Mesh
}

func New(w *wand.Listener) *Game {
	return &Game{
		wand:           w,
		hitThreshold:   7.0,
		accelCooldown:  0.15,
		maxAccel:       15.0,
		visualExponent: 2.5,
		audioExponent:  0.5,
		gyroThreshold:  25.0,
	}
}

func (g *Game) WantsChangeGame() bool {
	return g.wantsChange
}

func hsvToRGB(h, s, v float32) (uint8, uint8, uint8) {
	h = h - float32(math.Floor(float64(h)))
	h *= 6.0
	i := int(h)
	f := h - float32(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	var r, g, b float32
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}

func (g *Game) Init(e *engine.Engine) error {
	g.wantsChange = false

	e.SetMouseMode(false)

	e.Cam.Position = mgl32.Vec3{0, 3, 5}
	e.Cam.Pitch = -0.3 // slight downward look
	e.Cam.Far = 100

	// Dark ground plane to catch point light reflections
	var err error
	g.ground, err = mesh.NewGroundPlane(e.Rend, 50, 40, 40, 40)
	if err != nil {
		return err
	}

	g.icosa = buildIcosa()

	// Pause menu
	resolutions := e.Win.DisplayModes()
	g.pause = ui.NewPauseMenu(e.Rend, resolutions, e.Win.SupportsHDR())
	startResIdx := 0
	for i, res := range resolutions {
		if res.W == e.Win.Width() && res.H == e.Win.Height() {
			startResIdx = i
			break
		}
	}
	startRDIdx := 0
	for i, v := range ui.RenderDistances {
		if float32(v) == e.Cam.Far {
			startRDIdx = i
			break
		}
	}
	g.pause.SetAppliedState(e.Win.IsFullscreen(), startResIdx, startRDIdx, e.Win.HDR())

	// Lighting: near-total darkness, only point lights illuminate
	e.LightUniforms.AmbientColor = mgl32.Vec4{0.08, 0.08, 0.08, 1.0}
	e.LightUniforms.SunDirection = mgl32.Vec4{0, -1, 0, 0}
	e.LightUniforms.SunColor = mgl32.Vec4{0, 0, 0, 0}
	e.LightUniforms.NumLights = mgl32.Vec4{1, 0, 0, 0}
	e.LightUniforms.FogColor = mgl32.Vec4{0, 0, 0, 0}
	e.LightUniforms.FogParams = mgl32.Vec4{0, 0, 0, 0}

	// Post-process: light dither for lo-fi feel
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0.5, 8, 0, 0},
		Tint:   mgl32.Vec4{1.0, 1.0, 1.0, 0},
	}

	// Audio
	if e.Audio != nil {
		stream, err := e.Audio.NewStream()
		if err != nil {
			fmt.Println("Warning: drum circle audio:", err)
		} else {
			g.stream = stream
		}
	}

	return nil
}

func (g *Game) Update(e *engine.Engine, dt float32) bool {
	action := g.pause.HandleInput(e.Input)
	switch action {
	case ui.ActionQuit:
		return false
	case ui.ActionApplySettings:
		fs := g.pause.PendingFullscreen()
		w, h := g.pause.PendingResolution()
		rd := g.pause.PendingRenderDistance()
		hdr := g.pause.PendingHDR()
		e.ApplyDisplaySettings(fs, w, h, rd, hdr)
		g.pause.ConfirmApply()
	case ui.ActionChangeGame:
		g.wantsChange = true
	}

	if e.Input.IsKeyPressed(sdl.K_GRAVE) {
		g.showDebug = !g.showDebug
	}

	g.time += dt

	if g.pause.IsActive() {
		return true
	}

	s := g.wand.State()

	// Linear accel is gravity-compensated by the firmware — magnitude is
	// zero when the wand is still, regardless of orientation.
	accelMag := float32(math.Sqrt(float64(s.LinAccelX*s.LinAccelX + s.LinAccelY*s.LinAccelY + s.LinAccelZ*s.LinAccelZ)))

	// Gyro magnitude
	gyroMag := float32(math.Sqrt(float64(s.GyroX*s.GyroX + s.GyroY*s.GyroY + s.GyroZ*s.GyroZ)))

	// Trail color tracks orientation (HSV). Derived Euler is only cosmetic
	// here — never used for control logic.
	roll, pitch, yaw := s.Euler()
	trailHue := (yaw + 180.0) / 360.0
	trailSat := float32(0.7 + 0.3*((float64(pitch)+90.0)/180.0))
	trailVal := float32(0.8 + 0.2*(math.Abs(float64(roll))/180.0))
	cr, cg, cb := hsvToRGB(trailHue, trailSat, trailVal)

	// Accel hit detection with cooldown
	// We detect a "hit" at the peak of the acceleration spike (when it starts to decrease).
	if accelMag > g.hitThreshold && accelMag < g.lastAccelMag && (g.time-g.lastHitTime) > g.accelCooldown {
		g.lastHitTime = g.time

		// Face selection from wand tip direction → note.
		wq := mgl32.Quat{W: s.Q.W, V: mgl32.Vec3{s.Q.X, s.Q.Y, s.Q.Z}}
		faceI := g.icosa.findFace(wq.Rotate(mgl32.Vec3{1, 0, 0}))
		noteIdx := g.icosa.noteIdx[faceI]

		// 1. Calculate normalized intensity (0.0 to 1.0) within the active range
		normalized := (accelMag - g.hitThreshold) / (g.maxAccel - g.hitThreshold)
		if normalized > 1.0 {
			normalized = 1.0
		}

		// 2. Use separate exponents for visual vs audio response curves
		visualIntensity := float32(math.Pow(float64(normalized), float64(g.visualExponent)))
		audioIntensity := float32(math.Pow(float64(normalized), float64(g.audioExponent)))

		g.spawnHitEvent(cr, cg, cb, visualIntensity)

		// Spawn audio tone at the face's note.
		if g.stream != nil {
			g.sounds = append(g.sounds, activeSound{
				freq:      faceNotes[noteIdx],
				amplitude: 0.3 + audioIntensity*0.7,
			})
		}
	}
	g.lastAccelMag = accelMag

	// Gyro trail emission
	if gyroMag > g.gyroThreshold && len(g.trails) < maxTrails {
		count := int(gyroMag/100.0) + 1
		if count > 3 {
			count = 3
		}
		for i := 0; i < count && len(g.trails) < maxTrails; i++ {
			g.trails = append(g.trails, particle{
				X:    rand.Float32()*4 - 2,
				Y:    rand.Float32()*3 + 1,
				Z:    -rand.Float32()*4 - 4,
				VX:   rand.Float32()*0.6 - 0.3,
				VY:   rand.Float32()*0.6 - 0.3,
				VZ:   rand.Float32()*0.6 - 0.3,
				Size: 0.04 + rand.Float32()*0.06,
				R:    cr,
				G:    cg,
				B:    cb,
			})
		}
	}

	// Update hit events
	alive := 0
	for i := range g.events {
		ev := &g.events[i]
		ev.age += dt

		if ev.age >= eventLifetime {
			continue
		}

		// Update burst particles
		for j := range ev.burst {
			p := &ev.burst[j]
			p.X += p.VX * dt
			p.Y += p.VY * dt
			p.Z += p.VZ * dt
			p.VX *= burstDrag
			p.VY *= burstDrag
			p.VZ *= burstDrag
		}

		// Update ring particles (expand outward, no drag)
		for j := range ev.ring {
			p := &ev.ring[j]
			p.X += p.VX * dt
			p.Y += p.VY * dt
			p.Z += p.VZ * dt
		}

		g.events[alive] = g.events[i]
		alive++
	}
	g.events = g.events[:alive]

	// Update trail particles
	trailAlive := 0
	for i := range g.trails {
		t := &g.trails[i]
		t.Age += dt
		if t.Age >= trailLifetime {
			continue
		}
		t.X += t.VX * dt
		t.Y += t.VY * dt
		t.Z += t.VZ * dt
		g.trails[trailAlive] = g.trails[i]
		trailAlive++
	}
	g.trails = g.trails[:trailAlive]

	// Update point lights from active events
	lightIdx := 1 // 0 is camera headlamp
	for i := range g.events {
		if lightIdx >= 512 {
			break
		}
		ev := &g.events[i]
		fade := float32(1.0) - ev.age/eventLifetime
		e.LightUniforms.LightPositions[lightIdx] = mgl32.Vec4{ev.posX, ev.posY, ev.posZ, 0}
		e.LightUniforms.LightColors[lightIdx] = mgl32.Vec4{
			float32(ev.r) / 255.0 * fade,
			float32(ev.g) / 255.0 * fade,
			float32(ev.b) / 255.0 * fade,
			ev.intensity * fade * 15.0,
		}
		lightIdx++
	}
	e.LightUniforms.NumLights = mgl32.Vec4{float32(lightIdx), 0, 0, 0}

	g.generateAudio(dt)

	return true
}

func (g *Game) generateAudio(dt float32) {
	if g.stream == nil {
		return
	}

	numSamples := int(float32(audio.SampleRate) * dt)
	if numSamples <= 0 {
		return
	}

	// Grow/reuse mix buffer
	if cap(g.mixBuf) < numSamples {
		g.mixBuf = make([]float32, numSamples)
	}
	g.mixBuf = g.mixBuf[:numSamples]
	for i := range g.mixBuf {
		g.mixBuf[i] = 0
	}

	// Mix all active sounds
	alive := 0
	for i := range g.sounds {
		s := &g.sounds[i]
		if s.age > 0.5 {
			continue
		}

		for j := 0; j < numSamples; j++ {
			t := s.age + float32(j)/float32(audio.SampleRate)

			// Exponential decay envelope
			env := s.amplitude * float32(math.Exp(float64(-t*8.0)))

			// Sine tone
			sample := float32(math.Sin(float64(s.phase))) * env

			// Noise burst at attack (first 20ms)
			if t < 0.02 {
				noiseMix := (0.02 - t) / 0.02
				noise := (rand.Float32()*2 - 1) * noiseMix * s.amplitude * 0.3
				sample += noise
			}

			g.mixBuf[j] += sample

			// Advance phase
			s.phase += 2 * math.Pi * s.freq / float32(audio.SampleRate)
			if s.phase > 2*math.Pi {
				s.phase -= 2 * math.Pi
			}
		}

		s.age += dt
		g.sounds[alive] = g.sounds[i]
		alive++
	}
	g.sounds = g.sounds[:alive]

	if alive == 0 {
		return
	}

	// Clamp to [-1, 1]
	for i := range g.mixBuf {
		if g.mixBuf[i] > 1.0 {
			g.mixBuf[i] = 1.0
		} else if g.mixBuf[i] < -1.0 {
			g.mixBuf[i] = -1.0
		}
	}

	audio.PushSamples(g.stream, g.mixBuf)
}

func (g *Game) spawnHitEvent(r, cg, b uint8, intensity float32) {
	if len(g.events) >= maxEvents {
		// Remove oldest
		g.events = g.events[1:]
	}

	posX := rand.Float32()*10 - 5
	posY := rand.Float32()*3 + 1
	posZ := -rand.Float32()*6 - 3

	// Burst particles: radiate outward in all directions
	bCount := 20 + int(intensity*float32(burstCount-20))
	burst := make([]particle, bCount)
	for i := range burst {
		// Random direction on unit sphere via rejection sampling
		var dx, dy, dz float32
		for {
			dx = rand.Float32()*2 - 1
			dy = rand.Float32()*2 - 1
			dz = rand.Float32()*2 - 1
			lenSq := dx*dx + dy*dy + dz*dz
			if lenSq > 0.01 && lenSq <= 1.0 {
				inv := float32(1.0 / math.Sqrt(float64(lenSq)))
				dx *= inv
				dy *= inv
				dz *= inv
				break
			}
		}
		speed := (1.0 + rand.Float32()*3.0) * (0.5 + intensity)
		burst[i] = particle{
			X: posX, Y: posY, Z: posZ,
			VX: dx * speed, VY: dy * speed, VZ: dz * speed,
			Size: 0.06 + rand.Float32()*0.09,
			R:    r, G: cg, B: b,
		}
	}

	// Ring particles: arranged in a circle in XY plane, expanding outward
	ring := make([]particle, ringCount)
	for i := range ring {
		angle := float64(i) * 2.0 * math.Pi / float64(ringCount)
		dx := float32(math.Cos(angle))
		dy := float32(math.Sin(angle))
		ring[i] = particle{
			X: posX + dx*0.3, Y: posY + dy*0.3, Z: posZ,
			VX: dx * ringExpandRate, VY: dy * ringExpandRate, VZ: 0,
			Size: 0.08 + rand.Float32()*0.04,
			R:    r, G: cg, B: b,
		}
	}

	g.events = append(g.events, hitEvent{
		posX: posX, posY: posY, posZ: posZ,
		r: r, g: cg, b: b,
		intensity: intensity,
		burst:     burst,
		ring:      ring,
	})
}

func (g *Game) Render(e *engine.Engine, frame renderer.RenderFrame) {
	// 1. Draw ground plane (lit pipeline, receives point light illumination)
	groundModel := mgl32.Ident4()
	groundMVP := frame.ViewProj.Mul4(groundModel)
	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: g.ground.VertexBuffer,
		IndexBuffer:  g.ground.IndexBuffer,
		IndexCount:   g.ground.IndexCount,
		MVP:          groundMVP,
		Model:        groundModel,
		NoFog:        true,
	})

	// 2. Draw event particles (burst + ring merged into one buffer)
	g.renderEventParticles(e, frame)

	// 3. Draw trail particles
	g.renderTrailParticles(e, frame)
}

func (g *Game) renderEventParticles(e *engine.Engine, frame renderer.RenderFrame) {
	if g.eventVB != nil {
		e.Rend.ReleaseBuffer(g.eventVB)
		g.eventVB = nil
	}
	if g.eventIB != nil {
		e.Rend.ReleaseBuffer(g.eventIB)
		g.eventIB = nil
	}

	// Count total particles
	total := 0
	for i := range g.events {
		total += len(g.events[i].burst) + len(g.events[i].ring)
	}
	if total == 0 {
		return
	}

	vertices := make([]renderer.LitVertex, 0, total*4)
	indices := make([]uint16, 0, total*6)

	right := frame.CamRight
	up := frame.CamUp

	for i := range g.events {
		ev := &g.events[i]
		fade := float32(1.0) - ev.age/eventLifetime

		// Burst particles
		for j := range ev.burst {
			g.appendBillboard(&vertices, &indices, &ev.burst[j], fade, right, up)
		}

		// Ring particles
		for j := range ev.ring {
			g.appendBillboard(&vertices, &indices, &ev.ring[j], fade, right, up)
		}
	}

	vb, err := e.Rend.CreateLitVertexBuffer(vertices)
	if err != nil {
		return
	}
	ib, err := e.Rend.CreateIndexBuffer(indices)
	if err != nil {
		e.Rend.ReleaseBuffer(vb)
		return
	}

	g.eventVB = vb
	g.eventIB = ib

	e.Rend.DrawFireball(frame.CmdBuf, frame.ScenePass, renderer.FireballDrawCall{
		VertexBuffer: g.eventVB,
		IndexBuffer:  g.eventIB,
		IndexCount:   uint32(len(indices)),
		MVP:          frame.ViewProj,
		Time:         g.time,
	})
}

func (g *Game) renderTrailParticles(e *engine.Engine, frame renderer.RenderFrame) {
	if g.trailVB != nil {
		e.Rend.ReleaseBuffer(g.trailVB)
		g.trailVB = nil
	}
	if g.trailIB != nil {
		e.Rend.ReleaseBuffer(g.trailIB)
		g.trailIB = nil
	}

	if len(g.trails) == 0 {
		return
	}

	vertices := make([]renderer.LitVertex, 0, len(g.trails)*4)
	indices := make([]uint16, 0, len(g.trails)*6)

	right := frame.CamRight
	up := frame.CamUp

	for i := range g.trails {
		t := &g.trails[i]
		fade := float32(1.0) - t.Age/trailLifetime
		g.appendBillboard(&vertices, &indices, t, fade, right, up)
	}

	vb, err := e.Rend.CreateLitVertexBuffer(vertices)
	if err != nil {
		return
	}
	ib, err := e.Rend.CreateIndexBuffer(indices)
	if err != nil {
		e.Rend.ReleaseBuffer(vb)
		return
	}

	g.trailVB = vb
	g.trailIB = ib

	e.Rend.DrawFireball(frame.CmdBuf, frame.ScenePass, renderer.FireballDrawCall{
		VertexBuffer: g.trailVB,
		IndexBuffer:  g.trailIB,
		IndexCount:   uint32(len(indices)),
		MVP:          frame.ViewProj,
		Time:         g.time,
	})
}

func (g *Game) appendBillboard(vertices *[]renderer.LitVertex, indices *[]uint16, p *particle, fade float32, right, up mgl32.Vec3) {
	cr := uint8(float32(p.R) * fade)
	cg := uint8(float32(p.G) * fade)
	cb := uint8(float32(p.B) * fade)

	base := uint16(len(*vertices))
	s := p.Size

	rx := right[0] * s
	ry := right[1] * s
	rz := right[2] * s
	ux := up[0] * s
	uy := up[1] * s
	uz := up[2] * s

	*vertices = append(*vertices,
		renderer.LitVertex{
			X: p.X - rx - ux, Y: p.Y - ry - uy, Z: p.Z - rz - uz,
			R: cr, G: cg, B: cb, A: 255,
			U: 0, V: 0,
		},
		renderer.LitVertex{
			X: p.X + rx - ux, Y: p.Y + ry - uy, Z: p.Z + rz - uz,
			R: cr, G: cg, B: cb, A: 255,
			U: 1, V: 0,
		},
		renderer.LitVertex{
			X: p.X + rx + ux, Y: p.Y + ry + uy, Z: p.Z + rz + uz,
			R: cr, G: cg, B: cb, A: 255,
			U: 1, V: 1,
		},
		renderer.LitVertex{
			X: p.X - rx + ux, Y: p.Y - ry + uy, Z: p.Z - rz + uz,
			R: cr, G: cg, B: cb, A: 255,
			U: 0, V: 1,
		},
	)

	*indices = append(*indices,
		base, base+1, base+2,
		base, base+2, base+3,
	)
}

func (g *Game) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	if g.pause.IsActive() {
		g.pause.Render(e.Rend, cmdBuf, target, e.Win.Width(), e.Win.Height())
		return
	}

	for i, m := range g.debugMeshes {
		if m != nil {
			m.Destroy(e.Rend)
			g.debugMeshes[i] = nil
		}
	}

	if !g.showDebug {
		return
	}

	s := g.wand.State()
	accelMag := float32(math.Sqrt(float64(s.LinAccelX*s.LinAccelX + s.LinAccelY*s.LinAccelY + s.LinAccelZ*s.LinAccelZ)))
	gyroMag := float32(math.Sqrt(float64(s.GyroX*s.GyroX + s.GyroY*s.GyroY + s.GyroZ*s.GyroZ)))
	roll, pitch, yaw := s.Euler()

	lines := [4]string{
		fmt.Sprintf("ROLL  %7.1f", roll),
		fmt.Sprintf("PITCH %7.1f", pitch),
		fmt.Sprintf("YAW   %7.1f", yaw),
		fmt.Sprintf("ACC %5.1f GYR %5.1f", accelMag, gyroMag),
	}

	const ps = float32(3)
	ortho := mgl32.Ortho2D(0, float32(e.Win.Width()), float32(e.Win.Height()), 0)
	pass := e.Rend.BeginUIPass(cmdBuf, target)

	for i, text := range lines {
		m, _, err := ui.NewTextMesh(e.Rend, text, ps, 255, 255, 255, 200)
		if err != nil {
			continue
		}
		g.debugMeshes[i] = m
		y := float32(10) + float32(i)*ps*8
		e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
			VertexBuffer: m.VertexBuffer,
			IndexBuffer:  m.IndexBuffer,
			IndexCount:   m.IndexCount,
			Transform:    ortho.Mul4(mgl32.Translate3D(10, y, 0)),
		})
	}

	e.Rend.EndUIPass(pass)
}

func (g *Game) Destroy(e *engine.Engine) {
	if g.stream != nil {
		g.stream.Destroy()
	}
	g.pause.Destroy(e.Rend)
	g.ground.Destroy(e.Rend)
	if g.eventVB != nil {
		e.Rend.ReleaseBuffer(g.eventVB)
	}
	if g.eventIB != nil {
		e.Rend.ReleaseBuffer(g.eventIB)
	}
	if g.trailVB != nil {
		e.Rend.ReleaseBuffer(g.trailVB)
	}
	if g.trailIB != nil {
		e.Rend.ReleaseBuffer(g.trailIB)
	}
	for i, m := range g.debugMeshes {
		if m != nil {
			m.Destroy(e.Rend)
			g.debugMeshes[i] = nil
		}
	}
}
