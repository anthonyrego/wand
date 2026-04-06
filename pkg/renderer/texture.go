package renderer

import (
	"errors"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
)

const groundTexSize = 64

func generateGroundTextureData() []byte {
	data := make([]byte, groundTexSize*groundTexSize*4)
	for y := 0; y < groundTexSize; y++ {
		for x := 0; x < groundTexSize; x++ {
			idx := (y*groundTexSize + x) * 4

			// R channel: asphalt grain (subtle variation with occasional darker speckles)
			h := pixelHash(x, y)
			r := 220 + int(h%36) // 220-255 (~0.86-1.0)
			if h%13 == 0 {       // occasional dark speckle
				r = 180 + int(h%30) // 180-209 (~0.71-0.82)
			}

			// G channel: sidewalk slabs (16px grid with per-slab brightness variation)
			slabX := x / 16
			slabY := y / 16
			slabH := pixelHash(slabX+97, slabY+131)
			g := 220 + int(slabH%36) // per-slab base brightness
			bx := x % 16
			by := y % 16
			if bx == 0 || by == 0 { // joint lines
				g = 170 + int(slabH%20) // darker at joints
			}

			// B channel: grass variation (two-octave noise with clumps)
			h2 := pixelHash(x+53, y+71)
			h3 := pixelHash(x/3+29, y/3+37) // larger scale octave
			b := 200 + int((h2%28+h3%28)/2)  // 200-227
			if h2%5 == 0 {                    // clump variation
				b = 160 + int(h2%40) // 160-199
			}

			data[idx+0] = clampByte(r)
			data[idx+1] = clampByte(g)
			data[idx+2] = clampByte(b)
			data[idx+3] = 255
		}
	}
	return data
}

func pixelHash(x, y int) uint32 {
	h := uint32(x*374761393 + y*668265263)
	h = (h ^ (h >> 13)) * 1274126177
	h = h ^ (h >> 16)
	return h
}

func clampByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func (r *Renderer) createGroundTexture() error {
	device := r.window.Device()

	pixels := generateGroundTextureData()
	bufferSize := uint32(len(pixels))

	tex, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM,
		Width:             groundTexSize,
		Height:            groundTexSize,
		LayerCountOrDepth: 1,
		NumLevels:         1,
		Usage:             sdl.GPU_TEXTUREUSAGE_SAMPLER,
	})
	if err != nil {
		return errors.New("failed to create ground texture: " + err.Error())
	}

	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseTexture(tex)
		return errors.New("failed to create transfer buffer for ground texture: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseTexture(tex)
		device.ReleaseTransferBuffer(transferBuffer)
		return errors.New("failed to map transfer buffer for ground texture: " + err.Error())
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(transferDataPtr)), len(pixels))
	copy(dst, pixels)

	device.UnmapTransferBuffer(transferBuffer)

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseTexture(tex)
		device.ReleaseTransferBuffer(transferBuffer)
		return errors.New("failed to acquire command buffer for ground texture: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUTexture(
		&sdl.GPUTextureTransferInfo{
			TransferBuffer: transferBuffer,
			Offset:         0,
			PixelsPerRow:   groundTexSize,
			RowsPerLayer:   groundTexSize,
		},
		&sdl.GPUTextureRegion{
			Texture:  tex,
			MipLevel: 0,
			Layer:    0,
			X:        0,
			Y:        0,
			Z:        0,
			W:        groundTexSize,
			H:        groundTexSize,
			D:        1,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)

	r.groundTexture = tex
	return nil
}

func (r *Renderer) createPlaceholderTexture() error {
	pixels := []byte{255, 255, 255, 255} // 1x1 white
	tex, err := createTextureFromPixels(r.window.Device(), 1, 1, pixels)
	if err != nil {
		return err
	}
	r.placeholderTexture = tex
	return nil
}

func createTextureFromPixels(device *sdl.GPUDevice, width, height uint32, pixels []byte) (*sdl.GPUTexture, error) {
	bufferSize := uint32(len(pixels))

	tex, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM,
		Width:             width,
		Height:            height,
		LayerCountOrDepth: 1,
		NumLevels:         1,
		Usage:             sdl.GPU_TEXTUREUSAGE_SAMPLER,
	})
	if err != nil {
		return nil, errors.New("failed to create texture: " + err.Error())
	}

	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseTexture(tex)
		return nil, errors.New("failed to create transfer buffer: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseTexture(tex)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to map transfer buffer: " + err.Error())
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(transferDataPtr)), len(pixels))
	copy(dst, pixels)
	device.UnmapTransferBuffer(transferBuffer)

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseTexture(tex)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUTexture(
		&sdl.GPUTextureTransferInfo{
			TransferBuffer: transferBuffer,
			Offset:         0,
			PixelsPerRow:   width,
			RowsPerLayer:   height,
		},
		&sdl.GPUTextureRegion{
			Texture: tex,
			W:       width,
			H:       height,
			D:       1,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)
	return tex, nil
}
