package flying

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/toybox/pkg/audio"
	"github.com/anthonyrego/toybox/pkg/engine"
	"github.com/anthonyrego/toybox/pkg/mesh"
	"github.com/anthonyrego/toybox/pkg/renderer"
)

const (
	activeRings    = 6
	ringHoleRadius = float32(3.4)  // hoop opening radius (world units)
	ringTube       = float32(0.32) // torus tube radius
	ringMajorSegs  = 32
	ringMinorSegs  = 10
	ringGap        = float32(22.0) // spacing between rings along the flight path
	ringFirstAhead = float32(26.0) // distance of the nearest ring at game start
	ringSpread     = float32(6.0)  // lateral jitter radius at spawn
	ringBehind     = float32(14.0) // recycle once this far behind the player

	popLifetime   = float32(0.7) // seconds a pass-through celebration lives
	popBurstCount = 24
	popRingCount  = 18
	popRingRate   = float32(9.0)  // expansion speed of the pop ring sparks (u/s)
	popBurstDrag  = float32(0.92) // per-frame velocity multiplier for burst sparks

	comboIdleReset = float32(3.0)  // seconds without a hit resets the combo streak
	flashDecay     = float32(0.18) // seconds for the screen flash to fade out
)

// Vibrant per-ring colors. Each gets its own prebuilt torus mesh (vertex color
// lives in the buffer, so distinct colors need distinct meshes).
var ringPalette = [][3]uint8{
	{255, 90, 40},  // orange
	{40, 200, 255}, // cyan
	{255, 220, 50}, // gold
	{120, 255, 90}, // green
	{230, 90, 255}, // magenta
	{90, 130, 255}, // blue
}

// Ascending major-pentatonic scale (C5 up). The combo streak indexes into it,
// so consecutive rings climb the scale for a rewarding melodic run.
var chimeScale = []float32{
	523.25, 587.33, 659.25, 783.99, 880.00, // C5 D5 E5 G5 A5
	1046.50, 1174.66, 1318.51, 1567.98, 1760.00, // C6 D6 E6 G6 A6
	2093.00, 2349.32, // C7 D7
}

type ring struct {
	pos      mgl32.Vec3
	axis     mgl32.Vec3 // unit hoop normal; the player passes along this
	colorIdx int
	prevSide float32 // signed distance to the ring plane last frame (+ = player side)
}

type ringSpark struct {
	x, y, z    float32
	vx, vy, vz float32
	size       float32
}

// pop is the transient celebration spawned when a ring is cleared: a sparkle
// burst plus an expanding ring of sparks in the hoop's plane, all additive.
type pop struct {
	pos       mgl32.Vec3
	right, up mgl32.Vec3 // in-plane basis for the expanding ring
	r, g, b   uint8
	age       float32
	burst     []ringSpark
}

type chimeVoice struct {
	freq      float32
	phase     float32
	age       float32
	amplitude float32
}

// buildTorus creates a torus mesh in local space lying in the XY plane with its
// hole facing ±Z (so the pass-through axis is local +Z).
func buildTorus(r *renderer.Renderer, hole, tube float32, majorSegs, minorSegs int, cr, cg, cb uint8) (*mesh.Mesh, error) {
	var verts []renderer.LitVertex
	for i := 0; i <= majorSegs; i++ {
		a := 2 * math.Pi * float64(i) / float64(majorSegs)
		ca, sa := math.Cos(a), math.Sin(a)
		for j := 0; j <= minorSegs; j++ {
			b := 2 * math.Pi * float64(j) / float64(minorSegs)
			cbn, sb := math.Cos(b), math.Sin(b)
			rad := float64(hole) + float64(tube)*cbn
			verts = append(verts, renderer.LitVertex{
				X:  float32(rad * ca),
				Y:  float32(rad * sa),
				Z:  float32(float64(tube) * sb),
				NX: float32(cbn * ca), NY: float32(cbn * sa), NZ: float32(sb),
				R: cr, G: cg, B: cb, A: 255,
			})
		}
	}

	stride := minorSegs + 1
	var idxs []uint16
	for i := 0; i < majorSegs; i++ {
		for j := 0; j < minorSegs; j++ {
			a := uint16(i*stride + j)
			b := uint16((i+1)*stride + j)
			idxs = append(idxs, a, b, a+1, a+1, b, b+1)
		}
	}

	vb, err := r.CreateLitVertexBuffer(verts)
	if err != nil {
		return nil, err
	}
	ib, err := r.CreateIndexBuffer(idxs)
	if err != nil {
		r.ReleaseBuffer(vb)
		return nil, err
	}
	return &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(idxs))}, nil
}

func (g *Game) initRings(e *engine.Engine) error {
	g.ringMesh = make([]*mesh.Mesh, len(ringPalette))
	for i, c := range ringPalette {
		m, err := buildTorus(e.Rend, ringHoleRadius, ringTube, ringMajorSegs, ringMinorSegs, c[0], c[1], c[2])
		if err != nil {
			return err
		}
		g.ringMesh[i] = m
	}

	forward := g.orientation.Rotate(mgl32.Vec3{0, 0, -1})
	g.rings = make([]ring, activeRings)
	for i := range g.rings {
		g.placeRing(&g.rings[i], forward, ringFirstAhead+float32(i)*ringGap)
	}
	return nil
}

// placeRing positions a ring dist units ahead of the player along forward, with
// a small lateral jitter, and orients its hole to face the player.
func (g *Game) placeRing(rg *ring, forward mgl32.Vec3, dist float32) {
	right := forward.Cross(mgl32.Vec3{0, 1, 0})
	if right.Len() < 1e-4 {
		right = mgl32.Vec3{1, 0, 0}
	}
	right = right.Normalize()
	up := right.Cross(forward).Normalize()

	ang := rand.Float32() * 2 * math.Pi
	rad := ringSpread * float32(math.Sqrt(float64(rand.Float32())))
	offset := right.Mul(float32(math.Cos(float64(ang))) * rad).
		Add(up.Mul(float32(math.Sin(float64(ang))) * rad))

	rg.pos = g.position.Add(forward.Mul(dist)).Add(offset)
	rg.axis = g.position.Sub(rg.pos).Normalize()
	rg.prevSide = rg.axis.Dot(g.position.Sub(rg.pos)) // > 0: player starts on the +axis side
	rg.colorIdx = rand.Intn(len(ringPalette))
}

// recycleRing sends a ring a gap beyond the current frontmost ring so the stream
// of rings stays roughly evenly spaced ahead of the player.
func (g *Game) recycleRing(rg *ring, forward mgl32.Vec3) {
	maxAhead := float32(0)
	for i := range g.rings {
		if d := forward.Dot(g.rings[i].pos.Sub(g.position)); d > maxAhead {
			maxAhead = d
		}
	}
	g.placeRing(rg, forward, maxAhead+ringGap)
}

// updateRings advances the ring stream, detects pass-throughs (segment vs. the
// ring plane, robust at flight speed), and advances pop celebrations and lights.
func (g *Game) updateRings(e *engine.Engine, dt float32, prevPos mgl32.Vec3) {
	forward := g.orientation.Rotate(mgl32.Vec3{0, 0, -1})

	for i := range g.rings {
		rg := &g.rings[i]
		side := rg.axis.Dot(g.position.Sub(rg.pos))

		if rg.prevSide > 0 && side <= 0 {
			// Crossed the ring plane this frame — find where and test the hole.
			t := float32(1)
			if denom := rg.prevSide - side; denom > 1e-6 {
				t = rg.prevSide / denom
			}
			cross := prevPos.Add(g.position.Sub(prevPos).Mul(t))
			radial := cross.Sub(rg.pos)
			radial = radial.Sub(rg.axis.Mul(radial.Dot(rg.axis))) // drop the axial part
			if radial.Len() <= ringHoleRadius+ringTube {
				g.onRingHit(rg)
			}
			g.recycleRing(rg, forward)
			continue
		}
		rg.prevSide = side

		// Steered past without crossing the plane — retire it once well behind.
		if forward.Dot(rg.pos.Sub(g.position)) < -ringBehind {
			g.recycleRing(rg, forward)
		}
	}

	// Combo streak relaxes back to zero after a quiet spell.
	if g.combo > 0 && g.time-g.lastHitTime > comboIdleReset {
		g.combo = 0
	}

	// Screen flash fades back to neutral.
	if g.flash > 0 {
		g.flash -= dt / flashDecay
		if g.flash < 0 {
			g.flash = 0
		}
	}

	// Advance pops; drop the expired ones.
	alive := 0
	for i := range g.pops {
		p := &g.pops[i]
		p.age += dt
		if p.age >= popLifetime {
			continue
		}
		for j := range p.burst {
			s := &p.burst[j]
			s.x += s.vx * dt
			s.y += s.vy * dt
			s.z += s.vz * dt
			s.vx *= popBurstDrag
			s.vy *= popBurstDrag
			s.vz *= popBurstDrag
		}
		g.pops[alive] = g.pops[i]
		alive++
	}
	g.pops = g.pops[:alive]

	// Each active pop drives a colored point light pulse (lights the nearby rings).
	n := 0
	for i := range g.pops {
		if n >= 510 {
			break
		}
		p := &g.pops[i]
		fade := 1 - p.age/popLifetime
		e.LightUniforms.LightPositions[n] = mgl32.Vec4{p.pos.X(), p.pos.Y(), p.pos.Z(), 0}
		e.LightUniforms.LightColors[n] = mgl32.Vec4{
			float32(p.r) / 255 * fade,
			float32(p.g) / 255 * fade,
			float32(p.b) / 255 * fade,
			fade * 8.0,
		}
		n++
	}
	e.LightUniforms.NumLights = mgl32.Vec4{float32(n), 0, 0, 0}
}

func (g *Game) onRingHit(rg *ring) {
	g.combo++
	g.lastHitTime = g.time

	c := ringPalette[rg.colorIdx]
	g.flash = 1
	g.flashR, g.flashG, g.flashB = c[0], c[1], c[2]

	g.spawnPop(rg.pos, rg.axis, c)
	g.playChime()
}

func (g *Game) spawnPop(pos, axis mgl32.Vec3, c [3]uint8) {
	right := axis.Cross(mgl32.Vec3{0, 1, 0})
	if right.Len() < 1e-4 {
		right = mgl32.Vec3{1, 0, 0}
	}
	right = right.Normalize()
	up := right.Cross(axis).Normalize()

	burst := make([]ringSpark, popBurstCount)
	for i := range burst {
		var dx, dy, dz float32
		for {
			dx, dy, dz = rand.Float32()*2-1, rand.Float32()*2-1, rand.Float32()*2-1
			if l := dx*dx + dy*dy + dz*dz; l > 0.01 && l <= 1 {
				inv := float32(1 / math.Sqrt(float64(l)))
				dx, dy, dz = dx*inv, dy*inv, dz*inv
				break
			}
		}
		sp := 2.0 + rand.Float32()*4.0
		burst[i] = ringSpark{
			x: pos.X(), y: pos.Y(), z: pos.Z(),
			vx: dx * sp, vy: dy * sp, vz: dz * sp,
			size: 0.12 + rand.Float32()*0.14,
		}
	}

	g.pops = append(g.pops, pop{
		pos: pos, right: right, up: up,
		r: c[0], g: c[1], b: c[2],
		burst: burst,
	})
}

func (g *Game) playChime() {
	if g.stream == nil {
		return
	}
	step := g.combo - 1
	if step < 0 {
		step = 0
	}
	if step >= len(chimeScale) {
		step = len(chimeScale) - 1
	}
	// Root note plus a soft fifth for a brighter chime.
	g.voices = append(g.voices,
		chimeVoice{freq: chimeScale[step], amplitude: 0.5},
		chimeVoice{freq: chimeScale[step] * 1.5, amplitude: 0.18},
	)
}

func (g *Game) generateAudio(dt float32) {
	if g.stream == nil {
		return
	}
	numSamples := int(float32(audio.SampleRate) * dt)
	if numSamples <= 0 {
		return
	}
	if cap(g.mixBuf) < numSamples {
		g.mixBuf = make([]float32, numSamples)
	}
	g.mixBuf = g.mixBuf[:numSamples]
	for i := range g.mixBuf {
		g.mixBuf[i] = 0
	}

	alive := 0
	for i := range g.voices {
		v := &g.voices[i]
		if v.age > 1.2 {
			continue
		}
		for j := 0; j < numSamples; j++ {
			t := v.age + float32(j)/float32(audio.SampleRate)
			env := v.amplitude * float32(math.Exp(float64(-t*4.5)))
			sample := float32(math.Sin(float64(v.phase))) * env
			if t < 0.015 { // brief attack noise
				noiseMix := (0.015 - t) / 0.015
				sample += (rand.Float32()*2 - 1) * noiseMix * v.amplitude * 0.25
			}
			g.mixBuf[j] += sample
			v.phase += 2 * math.Pi * v.freq / float32(audio.SampleRate)
			if v.phase > 2*math.Pi {
				v.phase -= 2 * math.Pi
			}
		}
		v.age += dt
		g.voices[alive] = g.voices[i]
		alive++
	}
	g.voices = g.voices[:alive]
	if alive == 0 {
		return
	}

	for i := range g.mixBuf {
		if g.mixBuf[i] > 1 {
			g.mixBuf[i] = 1
		} else if g.mixBuf[i] < -1 {
			g.mixBuf[i] = -1
		}
	}
	audio.PushSamples(g.stream, g.mixBuf)
}

// renderRings draws the solid hoops through the lit pipeline. The caller must
// bind the lit pipeline first (DrawSwirl/DrawFireball leave their own bound).
func (g *Game) renderRings(e *engine.Engine, frame renderer.RenderFrame, viewProj mgl32.Mat4) {
	for i := range g.rings {
		rg := &g.rings[i]
		rot := mgl32.QuatBetweenVectors(mgl32.Vec3{0, 0, 1}, rg.axis).Mat4()
		model := mgl32.Translate3D(rg.pos.X(), rg.pos.Y(), rg.pos.Z()).Mul4(rot)
		m := g.ringMesh[rg.colorIdx]
		e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
			VertexBuffer: m.VertexBuffer,
			IndexBuffer:  m.IndexBuffer,
			IndexCount:   m.IndexCount,
			MVP:          viewProj.Mul4(model),
			Model:        model,
			NoFog:        true,
		})
	}
}

// renderPops draws the additive pass-through celebrations (sparkle burst + an
// expanding ring of sparks in the hoop's plane).
func (g *Game) renderPops(e *engine.Engine, frame renderer.RenderFrame, viewProj mgl32.Mat4, right, up mgl32.Vec3) {
	if g.popVB != nil {
		e.Rend.ReleaseBuffer(g.popVB)
		g.popVB = nil
	}
	if g.popIB != nil {
		e.Rend.ReleaseBuffer(g.popIB)
		g.popIB = nil
	}
	if len(g.pops) == 0 {
		return
	}

	var verts []renderer.LitVertex
	var idxs []uint16

	for i := range g.pops {
		p := &g.pops[i]
		fade := 1 - p.age/popLifetime

		for j := range p.burst {
			s := &p.burst[j]
			appendPopQuad(&verts, &idxs, s.x, s.y, s.z, s.size, p.r, p.g, p.b, fade, right, up)
		}

		radius := ringHoleRadius + p.age*popRingRate
		for k := 0; k < popRingCount; k++ {
			ang := 2 * math.Pi * float64(k) / float64(popRingCount)
			dir := p.right.Mul(float32(math.Cos(ang))).Add(p.up.Mul(float32(math.Sin(ang))))
			pos := p.pos.Add(dir.Mul(radius))
			appendPopQuad(&verts, &idxs, pos.X(), pos.Y(), pos.Z(), 0.16, p.r, p.g, p.b, fade*fade, right, up)
		}
	}

	if len(verts) == 0 {
		return
	}
	vb, err := e.Rend.CreateLitVertexBuffer(verts)
	if err != nil {
		return
	}
	ib, err := e.Rend.CreateIndexBuffer(idxs)
	if err != nil {
		e.Rend.ReleaseBuffer(vb)
		return
	}
	g.popVB = vb
	g.popIB = ib
	e.Rend.DrawFireball(frame.CmdBuf, frame.ScenePass, renderer.FireballDrawCall{
		VertexBuffer: g.popVB,
		IndexBuffer:  g.popIB,
		IndexCount:   uint32(len(idxs)),
		MVP:          viewProj,
		Time:         g.time,
	})
}

func appendPopQuad(verts *[]renderer.LitVertex, idxs *[]uint16, x, y, z, size float32, r, g, b uint8, fade float32, right, up mgl32.Vec3) {
	if fade < 0 {
		fade = 0
	}
	cr := uint8(float32(r) * fade)
	cg := uint8(float32(g) * fade)
	cb := uint8(float32(b) * fade)
	base := uint16(len(*verts))

	rx, ry, rz := right[0]*size, right[1]*size, right[2]*size
	ux, uy, uz := up[0]*size, up[1]*size, up[2]*size

	*verts = append(*verts,
		renderer.LitVertex{X: x - rx - ux, Y: y - ry - uy, Z: z - rz - uz, R: cr, G: cg, B: cb, A: 255, U: 0, V: 0},
		renderer.LitVertex{X: x + rx - ux, Y: y + ry - uy, Z: z + rz - uz, R: cr, G: cg, B: cb, A: 255, U: 1, V: 0},
		renderer.LitVertex{X: x + rx + ux, Y: y + ry + uy, Z: z + rz + uz, R: cr, G: cg, B: cb, A: 255, U: 1, V: 1},
		renderer.LitVertex{X: x - rx + ux, Y: y - ry + uy, Z: z - rz + uz, R: cr, G: cg, B: cb, A: 255, U: 0, V: 1},
	)
	*idxs = append(*idxs, base, base+1, base+2, base, base+2, base+3)
}
