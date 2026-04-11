# Game Ideas

Ideas for wand games targeting 10-12 month olds. At that age, the key developmental hooks are cause-and-effect discovery, object permanence, rhythm/repetition, and sensory feedback from gross motor movement (shaking, waving, pointing). Fine motor precision is limited, so games should reward any movement and make big, obvious things happen.

## Shake-to-Hatch

Shake the wand to crack open an egg (or seed, cocoon, etc). Accelerometer magnitude drives crack progress — screen shows cracks spreading, shell fragments flying off. When it hatches, a colorful creature/flower/butterfly bursts out with particles and light. Then it resets with a new egg. Pure cause-and-effect loop. Teaches "I shake -> thing happens." Could cycle through different creatures/colors.

**Input**: Accelerometer magnitude (shaking)
**Complexity**: Low

## Drum Circle / Rhythm Splash

Each axis of wand movement triggers a different visual "splash" — roll makes circular ripples, pitch makes vertical waves, yaw makes horizontal sweeps. Each paired with a distinct color palette. Accelerometer hits above a threshold create big "cymbal crash" bursts. The screen becomes a canvas of overlapping ripples and splashes. No goal, just painting with motion.

**Input**: All axes of orientation + accelerometer
**Complexity**: Medium

## Fireflies / Catch the Light

Glowing particles drift around the screen. When the wand points toward a cluster (orientation maps to a cursor/spotlight), they scatter and reform elsewhere with a satisfying burst. No score, no fail state — just the delight of "I pointed and they moved."

**Input**: Orientation (pointing direction)
**Complexity**: Medium

## Bubble Pop

Bubbles float up from the bottom at varying speeds. Shaking the wand (accel magnitude) creates a "wind" that pushes them around and pops them with particle bursts. Harder shakes pop more bubbles. Bubbles respawn continuously. Immediate and physical feedback loop.

**Input**: Accelerometer magnitude (shaking intensity)
**Complexity**: Low-Medium

## Day/Night Cycle World

A simple landscape (ground plane + sky dome) where wand pitch controls the sun's elevation — tilt up for day (bright, warm colors, animated shapes), tilt down for night (stars, moon, cool colors). Rolling the wand shifts the color palette. The whole world responds to their arm position.

**Input**: Pitch (sun elevation), Roll (color shift)
**Complexity**: Medium

## Kaleidoscope

Wand orientation drives a procedural kaleidoscope pattern (mirror-symmetric copies of colored shapes). Rotation morphs the pattern. Acceleration shifts color/speed. The existing swirl shader could serve as a foundation with mirror symmetry applied.

**Input**: Orientation (pattern morphing), Accelerometer (color/speed)
**Complexity**: Medium (shader work)

## Peek-a-Boo Faces

Simple shapes (circle face with eyes) hide behind a "curtain." When the baby shakes/waves the wand (accel threshold), the curtain pulls away revealing the face — which reacts with a happy animation (eyes widen, color change, particle burst). After a beat, it hides again. Object permanence game. Cycles through different faces/colors.

**Input**: Accelerometer magnitude (shaking threshold)
**Complexity**: Low-Medium
