package renderer

import (
	"errors"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand/pkg/camera"
	"github.com/anthonyrego/wand/pkg/shaders"
	"github.com/anthonyrego/wand/pkg/window"
)

// RenderFrame bundles per-frame state needed by entity rendering methods.
type RenderFrame struct {
	CmdBuf     *sdl.GPUCommandBuffer
	ScenePass  *sdl.GPURenderPass
	ViewProj   mgl32.Mat4
	Frustum    camera.Frustum
	CamPos     mgl32.Vec3
	CamRight   mgl32.Vec3 // for billboarding
	CamUp      mgl32.Vec3 // for billboarding
	CullDist   float32
	CullDistSq float32
	FadeStart  float32
	FadeRange  float32
}

const (
	defaultOffscreenWidth  = 320
	defaultOffscreenHeight = 180
)

type Vertex struct {
	X, Y, Z    float32
	R, G, B, A uint8
}

func NewVertex(x, y, z float32, r, g, b, a uint8) Vertex {
	return Vertex{X: x, Y: y, Z: z, R: r, G: g, B: b, A: a}
}

type LitVertex struct {
	X, Y, Z    float32 // location 0: FLOAT3
	NX, NY, NZ float32 // location 1: FLOAT3
	R, G, B, A uint8   // location 2: UBYTE4_NORM
	U, V       float32 // location 3: FLOAT2
}

type DrawCall struct {
	VertexBuffer *sdl.GPUBuffer
	IndexBuffer  *sdl.GPUBuffer
	IndexCount   uint32
	Transform    mgl32.Mat4
}

type LitDrawCall struct {
	VertexBuffer *sdl.GPUBuffer
	IndexBuffer  *sdl.GPUBuffer
	IndexCount   uint32
	MVP          mgl32.Mat4
	Model        mgl32.Mat4
	NoFog        bool
	NoDepthWrite bool    // Use depth-test-only pipeline (for sky dome)
	DepthBias    bool    // Use depth bias pipeline (for ground plane behind coplanar surfaces)
	Index32      bool    // Use 32-bit index buffer (for merged meshes)
	FadeFactor   float32 // 0=solid, 1=fully discarded (dithered fade-out)
	SurfaceType  int     // 0=none, 1=roadbed, 2=sidewalk, 3=park
	Highlight    float32 // 0=none, 1=full highlight (admin mode selection)
}

type LitVertexUniforms struct {
	MVP   mgl32.Mat4
	Model mgl32.Mat4
	Flags mgl32.Vec4 // x=1 to skip fog
}

type LightUniforms struct {
	LightPositions [512]mgl32.Vec4 // xyz=pos, w=unused
	LightColors    [512]mgl32.Vec4 // rgb=color, a=intensity
	AmbientColor   mgl32.Vec4
	CameraPos      mgl32.Vec4
	NumLights      mgl32.Vec4 // x=count
	SunDirection   mgl32.Vec4 // xyz=direction (toward sun), w=unused
	SunColor       mgl32.Vec4 // rgb=color, a=intensity
	FogColor       mgl32.Vec4 // rgb=fog color (pre-post-process)
	FogParams      mgl32.Vec4 // x=start distance, y=end distance, z=render distance (far plane fade)
	TextureParams  mgl32.Vec4 // x=scale (world meters per tile), y=strength (0=off, 1=full)
	SkyParams      mgl32.Vec4 // x=time, y=speed, z=scale, w=intensity
}

type PostProcessUniforms struct {
	Resolution mgl32.Vec4 // xy = offscreen size
	Dither     mgl32.Vec4 // x = strength (0=off, 1=full), y = color levels
	Tint       mgl32.Vec4 // rgb = per-channel multiplier
}

type Renderer struct {
	window   *window.Window
	pipeline *sdl.GPUGraphicsPipeline

	depthTexture *sdl.GPUTexture

	// Lit rendering
	litPipeline              *sdl.GPUGraphicsPipeline
	litNoDepthWritePipeline  *sdl.GPUGraphicsPipeline
	litDepthBiasPipeline     *sdl.GPUGraphicsPipeline
	postProcessPipeline *sdl.GPUGraphicsPipeline
	offscreenTexture    *sdl.GPUTexture
	offscreenDepth      *sdl.GPUTexture
	nearestSampler      *sdl.GPUSampler
	groundTexture       *sdl.GPUTexture
	placeholderTexture  *sdl.GPUTexture
	repeatSampler       *sdl.GPUSampler
	offscreenW          uint32
	offscreenH          uint32

	// UI overlay rendering
	uiPipeline *sdl.GPUGraphicsPipeline
}

func New(w *window.Window) (*Renderer, error) {
	device := w.Device()

	// Create shaders
	vertexShader, err := shaders.LoadShader(device, "PositionColorTransform.vert", 0, 1, 0, 0)
	if err != nil {
		return nil, errors.New("failed to create vertex shader: " + err.Error())
	}
	defer device.ReleaseShader(vertexShader)

	fragmentShader, err := shaders.LoadShader(device, "SolidColor.frag", 0, 0, 0, 0)
	if err != nil {
		return nil, errors.New("failed to create fragment shader: " + err.Error())
	}
	defer device.ReleaseShader(fragmentShader)

	// Create pipeline
	colorTargetDescriptions := []sdl.GPUColorTargetDescription{
		{
			Format: device.SwapchainTextureFormat(w.Handle()),
		},
	}

	vertexBufferDescriptions := []sdl.GPUVertexBufferDescription{
		{
			Slot:             0,
			InputRate:        sdl.GPU_VERTEXINPUTRATE_VERTEX,
			InstanceStepRate: 0,
			Pitch:            uint32(unsafe.Sizeof(Vertex{})),
		},
	}

	vertexAttributes := []sdl.GPUVertexAttribute{
		{
			BufferSlot: 0,
			Format:     sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3,
			Location:   0,
			Offset:     0,
		},
		{
			BufferSlot: 0,
			Format:     sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM,
			Location:   1,
			Offset:     uint32(unsafe.Sizeof(float32(0)) * 3),
		},
	}

	pipelineCreateInfo := sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: colorTargetDescriptions,
			HasDepthStencilTarget:   true,
			DepthStencilFormat:      sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		},
		DepthStencilState: sdl.GPUDepthStencilState{
			EnableDepthTest:  true,
			EnableDepthWrite: true,
			CompareOp:        sdl.GPU_COMPAREOP_GREATER_OR_EQUAL,
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: vertexBufferDescriptions,
			VertexAttributes:         vertexAttributes,
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode: sdl.GPU_FILLMODE_FILL,
			CullMode: sdl.GPU_CULLMODE_BACK,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   vertexShader,
		FragmentShader: fragmentShader,
	}

	pipeline, err := device.CreateGraphicsPipeline(&pipelineCreateInfo)
	if err != nil {
		return nil, errors.New("failed to create pipeline: " + err.Error())
	}

	// Create depth texture
	depthTexture, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		Width:             uint32(w.Width()),
		Height:            uint32(w.Height()),
		LayerCountOrDepth: 1,
		NumLevels:         1,
		Usage:             sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET,
	})
	if err != nil {
		device.ReleaseGraphicsPipeline(pipeline)
		return nil, errors.New("failed to create depth texture: " + err.Error())
	}

	r := &Renderer{
		window:       w,
		pipeline:     pipeline,
		depthTexture: depthTexture,
	}

	// Create lit rendering resources
	if err := r.initLitPipeline(); err != nil {
		r.Destroy()
		return nil, err
	}

	// Create UI overlay pipeline
	if err := r.initUIPipeline(); err != nil {
		r.Destroy()
		return nil, err
	}

	return r, nil
}

func (r *Renderer) initLitPipeline() error {
	device := r.window.Device()

	// --- Lit pipeline ---
	litVert, err := shaders.LoadShader(device, "Lit.vert", 0, 1, 0, 0)
	if err != nil {
		return errors.New("failed to create lit vertex shader: " + err.Error())
	}
	defer device.ReleaseShader(litVert)

	litFrag, err := shaders.LoadShader(device, "Lit.frag", 2, 1, 0, 0)
	if err != nil {
		return errors.New("failed to create lit fragment shader: " + err.Error())
	}
	defer device.ReleaseShader(litFrag)

	litPipeline, err := device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{Format: sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM},
			},
			HasDepthStencilTarget: true,
			DepthStencilFormat:    sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		},
		DepthStencilState: sdl.GPUDepthStencilState{
			EnableDepthTest:  true,
			EnableDepthWrite: true,
			CompareOp:        sdl.GPU_COMPAREOP_GREATER_OR_EQUAL,
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{
				{
					Slot:      0,
					InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
					Pitch:     uint32(unsafe.Sizeof(LitVertex{})),
				},
			},
			VertexAttributes: []sdl.GPUVertexAttribute{
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 0, Offset: 0},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 1, Offset: 12},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM, Location: 2, Offset: 24},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2, Location: 3, Offset: 28},
			},
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode: sdl.GPU_FILLMODE_FILL,
			CullMode: sdl.GPU_CULLMODE_BACK,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   litVert,
		FragmentShader: litFrag,
	})
	if err != nil {
		return errors.New("failed to create lit pipeline: " + err.Error())
	}
	r.litPipeline = litPipeline

	// Lit pipeline variant: depth test only, no depth write (for sky dome)
	litNoDepthWritePipeline, err := device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{Format: sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM},
			},
			HasDepthStencilTarget: true,
			DepthStencilFormat:    sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		},
		DepthStencilState: sdl.GPUDepthStencilState{
			EnableDepthTest:  true,
			EnableDepthWrite: false,
			CompareOp:        sdl.GPU_COMPAREOP_GREATER_OR_EQUAL,
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{
				{
					Slot:      0,
					InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
					Pitch:     uint32(unsafe.Sizeof(LitVertex{})),
				},
			},
			VertexAttributes: []sdl.GPUVertexAttribute{
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 0, Offset: 0},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 1, Offset: 12},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM, Location: 2, Offset: 24},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2, Location: 3, Offset: 28},
			},
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode: sdl.GPU_FILLMODE_FILL,
			CullMode: sdl.GPU_CULLMODE_BACK,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   litVert,
		FragmentShader: litFrag,
	})
	if err != nil {
		return errors.New("failed to create lit no-depth-write pipeline: " + err.Error())
	}
	r.litNoDepthWritePipeline = litNoDepthWritePipeline

	// Lit pipeline variant: depth bias (pushes fragments back to avoid z-fighting)
	litDepthBiasPipeline, err := device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{Format: sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM},
			},
			HasDepthStencilTarget: true,
			DepthStencilFormat:    sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		},
		DepthStencilState: sdl.GPUDepthStencilState{
			EnableDepthTest:  true,
			EnableDepthWrite: true,
			CompareOp:        sdl.GPU_COMPAREOP_GREATER_OR_EQUAL,
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{
				{
					Slot:      0,
					InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
					Pitch:     uint32(unsafe.Sizeof(LitVertex{})),
				},
			},
			VertexAttributes: []sdl.GPUVertexAttribute{
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 0, Offset: 0},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 1, Offset: 12},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM, Location: 2, Offset: 24},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2, Location: 3, Offset: 28},
			},
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode:                sdl.GPU_FILLMODE_FILL,
			CullMode:                sdl.GPU_CULLMODE_BACK,
			EnableDepthBias:         true,
			DepthBiasConstantFactor: -4,
			DepthBiasSlopeFactor:    -2,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   litVert,
		FragmentShader: litFrag,
	})
	if err != nil {
		return errors.New("failed to create lit depth-bias pipeline: " + err.Error())
	}
	r.litDepthBiasPipeline = litDepthBiasPipeline

	// --- Post-process pipeline ---
	ppVert, err := shaders.LoadShader(device, "PostProcess.vert", 0, 0, 0, 0)
	if err != nil {
		return errors.New("failed to create post-process vertex shader: " + err.Error())
	}
	defer device.ReleaseShader(ppVert)

	ppFrag, err := shaders.LoadShader(device, "PostProcess.frag", 1, 1, 0, 0)
	if err != nil {
		return errors.New("failed to create post-process fragment shader: " + err.Error())
	}
	defer device.ReleaseShader(ppFrag)

	postProcessPipeline, err := device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{Format: device.SwapchainTextureFormat(r.window.Handle())},
			},
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode: sdl.GPU_FILLMODE_FILL,
			CullMode: sdl.GPU_CULLMODE_NONE,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   ppVert,
		FragmentShader: ppFrag,
	})
	if err != nil {
		return errors.New("failed to create post-process pipeline: " + err.Error())
	}
	r.postProcessPipeline = postProcessPipeline

	// --- Offscreen textures ---
	if err := r.createOffscreenTargets(defaultOffscreenWidth, defaultOffscreenHeight); err != nil {
		return err
	}

	// --- Nearest sampler ---
	nearestSampler, err := device.CreateSampler(&sdl.GPUSamplerCreateInfo{
		MinFilter:    sdl.GPU_FILTER_NEAREST,
		MagFilter:    sdl.GPU_FILTER_NEAREST,
		MipmapMode:   sdl.GPU_SAMPLERMIPMAPMODE_NEAREST,
		AddressModeU: sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE,
		AddressModeV: sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE,
		AddressModeW: sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE,
	})
	if err != nil {
		return errors.New("failed to create nearest sampler: " + err.Error())
	}
	r.nearestSampler = nearestSampler

	// --- Repeat sampler (for ground texture tiling) ---
	repeatSampler, err := device.CreateSampler(&sdl.GPUSamplerCreateInfo{
		MinFilter:    sdl.GPU_FILTER_NEAREST,
		MagFilter:    sdl.GPU_FILTER_NEAREST,
		MipmapMode:   sdl.GPU_SAMPLERMIPMAPMODE_NEAREST,
		AddressModeU: sdl.GPU_SAMPLERADDRESSMODE_REPEAT,
		AddressModeV: sdl.GPU_SAMPLERADDRESSMODE_REPEAT,
		AddressModeW: sdl.GPU_SAMPLERADDRESSMODE_REPEAT,
	})
	if err != nil {
		return errors.New("failed to create repeat sampler: " + err.Error())
	}
	r.repeatSampler = repeatSampler

	// --- Textures ---
	if err := r.createGroundTexture(); err != nil {
		return err
	}
	if err := r.createPlaceholderTexture(); err != nil {
		return err
	}

	return nil
}

func (r *Renderer) createOffscreenTargets(w, h uint32) error {
	device := r.window.Device()

	offscreenTexture, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM,
		Width:             w,
		Height:            h,
		LayerCountOrDepth: 1,
		NumLevels:         1,
		Usage:             sdl.GPU_TEXTUREUSAGE_SAMPLER | sdl.GPU_TEXTUREUSAGE_COLOR_TARGET,
	})
	if err != nil {
		return errors.New("failed to create offscreen texture: " + err.Error())
	}

	offscreenDepth, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		Width:             w,
		Height:            h,
		LayerCountOrDepth: 1,
		NumLevels:         1,
		Usage:             sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET,
	})
	if err != nil {
		device.ReleaseTexture(offscreenTexture)
		return errors.New("failed to create offscreen depth texture: " + err.Error())
	}

	r.offscreenTexture = offscreenTexture
	r.offscreenDepth = offscreenDepth
	r.offscreenW = w
	r.offscreenH = h
	return nil
}

func (r *Renderer) SetOffscreenResolution(w, h uint32) error {
	if w == r.offscreenW && h == r.offscreenH {
		return nil
	}

	device := r.window.Device()

	// Wait for GPU to finish using the old textures
	device.WaitForIdle()

	if r.offscreenDepth != nil {
		device.ReleaseTexture(r.offscreenDepth)
		r.offscreenDepth = nil
	}
	if r.offscreenTexture != nil {
		device.ReleaseTexture(r.offscreenTexture)
		r.offscreenTexture = nil
	}

	return r.createOffscreenTargets(w, h)
}

func (r *Renderer) CreateVertexBuffer(vertices []Vertex) (*sdl.GPUBuffer, error) {
	device := r.window.Device()

	bufferSize := uint32(len(vertices)) * uint32(unsafe.Sizeof(Vertex{}))

	buffer, err := device.CreateBuffer(&sdl.GPUBufferCreateInfo{
		Usage: sdl.GPU_BUFFERUSAGE_VERTEX,
		Size:  bufferSize,
	})
	if err != nil {
		return nil, errors.New("failed to create vertex buffer: " + err.Error())
	}

	// Create transfer buffer and upload data
	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseBuffer(buffer)
		return nil, errors.New("failed to create transfer buffer: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to map transfer buffer: " + err.Error())
	}

	vertexData := unsafe.Slice((*Vertex)(unsafe.Pointer(transferDataPtr)), len(vertices))
	copy(vertexData, vertices)

	device.UnmapTransferBuffer(transferBuffer)

	// Upload to GPU
	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{
			TransferBuffer: transferBuffer,
			Offset:         0,
		},
		&sdl.GPUBufferRegion{
			Buffer: buffer,
			Offset: 0,
			Size:   bufferSize,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)

	return buffer, nil
}

func (r *Renderer) CreateLitVertexBuffer(vertices []LitVertex) (*sdl.GPUBuffer, error) {
	device := r.window.Device()

	bufferSize := uint32(len(vertices)) * uint32(unsafe.Sizeof(LitVertex{}))

	buffer, err := device.CreateBuffer(&sdl.GPUBufferCreateInfo{
		Usage: sdl.GPU_BUFFERUSAGE_VERTEX,
		Size:  bufferSize,
	})
	if err != nil {
		return nil, errors.New("failed to create lit vertex buffer: " + err.Error())
	}

	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseBuffer(buffer)
		return nil, errors.New("failed to create transfer buffer: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to map transfer buffer: " + err.Error())
	}

	vertexData := unsafe.Slice((*LitVertex)(unsafe.Pointer(transferDataPtr)), len(vertices))
	copy(vertexData, vertices)

	device.UnmapTransferBuffer(transferBuffer)

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{
			TransferBuffer: transferBuffer,
			Offset:         0,
		},
		&sdl.GPUBufferRegion{
			Buffer: buffer,
			Offset: 0,
			Size:   bufferSize,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)

	return buffer, nil
}

func (r *Renderer) CreateIndexBuffer(indices []uint16) (*sdl.GPUBuffer, error) {
	device := r.window.Device()

	bufferSize := uint32(len(indices)) * uint32(unsafe.Sizeof(uint16(0)))

	buffer, err := device.CreateBuffer(&sdl.GPUBufferCreateInfo{
		Usage: sdl.GPU_BUFFERUSAGE_INDEX,
		Size:  bufferSize,
	})
	if err != nil {
		return nil, errors.New("failed to create index buffer: " + err.Error())
	}

	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseBuffer(buffer)
		return nil, errors.New("failed to create transfer buffer: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to map transfer buffer: " + err.Error())
	}

	indexData := unsafe.Slice((*uint16)(unsafe.Pointer(transferDataPtr)), len(indices))
	copy(indexData, indices)

	device.UnmapTransferBuffer(transferBuffer)

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{
			TransferBuffer: transferBuffer,
			Offset:         0,
		},
		&sdl.GPUBufferRegion{
			Buffer: buffer,
			Offset: 0,
			Size:   bufferSize,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)

	return buffer, nil
}

func (r *Renderer) CreateIndexBuffer32(indices []uint32) (*sdl.GPUBuffer, error) {
	device := r.window.Device()

	bufferSize := uint32(len(indices)) * uint32(unsafe.Sizeof(uint32(0)))

	buffer, err := device.CreateBuffer(&sdl.GPUBufferCreateInfo{
		Usage: sdl.GPU_BUFFERUSAGE_INDEX,
		Size:  bufferSize,
	})
	if err != nil {
		return nil, errors.New("failed to create index buffer: " + err.Error())
	}

	transferBuffer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  bufferSize,
	})
	if err != nil {
		device.ReleaseBuffer(buffer)
		return nil, errors.New("failed to create transfer buffer: " + err.Error())
	}

	transferDataPtr, err := device.MapTransferBuffer(transferBuffer, false)
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to map transfer buffer: " + err.Error())
	}

	indexData := unsafe.Slice((*uint32)(unsafe.Pointer(transferDataPtr)), len(indices))
	copy(indexData, indices)

	device.UnmapTransferBuffer(transferBuffer)

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		device.ReleaseBuffer(buffer)
		device.ReleaseTransferBuffer(transferBuffer)
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	copyPass := cmdBuf.BeginCopyPass()
	copyPass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{
			TransferBuffer: transferBuffer,
			Offset:         0,
		},
		&sdl.GPUBufferRegion{
			Buffer: buffer,
			Offset: 0,
			Size:   bufferSize,
		},
		false,
	)
	copyPass.End()
	cmdBuf.Submit()

	device.ReleaseTransferBuffer(transferBuffer)

	return buffer, nil
}

// --- Original rendering methods ---

func (r *Renderer) BeginFrame() (*sdl.GPUCommandBuffer, *sdl.GPURenderPass, error) {
	device := r.window.Device()

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		return nil, nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	swapchainTexture, err := cmdBuf.WaitAndAcquireGPUSwapchainTexture(r.window.Handle())
	if err != nil {
		return nil, nil, errors.New("failed to acquire swapchain texture: " + err.Error())
	}

	if swapchainTexture == nil {
		cmdBuf.Submit()
		return nil, nil, nil
	}

	colorTargetInfo := sdl.GPUColorTargetInfo{
		Texture:    swapchainTexture.Texture,
		ClearColor: sdl.FColor{R: 0.1, G: 0.1, B: 0.15, A: 1.0},
		LoadOp:     sdl.GPU_LOADOP_CLEAR,
		StoreOp:    sdl.GPU_STOREOP_STORE,
	}

	depthStencilTargetInfo := sdl.GPUDepthStencilTargetInfo{
		Texture:        r.depthTexture,
		ClearDepth:     0.0, // reversed-Z: far=0, near=1
		LoadOp:         sdl.GPU_LOADOP_CLEAR,
		StoreOp:        sdl.GPU_STOREOP_DONT_CARE,
		StencilLoadOp:  sdl.GPU_LOADOP_DONT_CARE,
		StencilStoreOp: sdl.GPU_STOREOP_DONT_CARE,
		Cycle:          true,
		ClearStencil:   0,
	}

	renderPass := cmdBuf.BeginRenderPass(
		[]sdl.GPUColorTargetInfo{colorTargetInfo},
		&depthStencilTargetInfo,
	)

	renderPass.BindGraphicsPipeline(r.pipeline)

	return cmdBuf, renderPass, nil
}

func (r *Renderer) Draw(cmdBuf *sdl.GPUCommandBuffer, renderPass *sdl.GPURenderPass, call DrawCall) {
	// Push MVP matrix
	cmdBuf.PushVertexUniformData(0, unsafe.Slice(
		(*byte)(unsafe.Pointer(&call.Transform)), unsafe.Sizeof(call.Transform),
	))

	renderPass.BindVertexBuffers([]sdl.GPUBufferBinding{
		{Buffer: call.VertexBuffer, Offset: 0},
	})

	renderPass.BindIndexBuffer(&sdl.GPUBufferBinding{
		Buffer: call.IndexBuffer, Offset: 0,
	}, sdl.GPU_INDEXELEMENTSIZE_16BIT)

	renderPass.DrawIndexedPrimitives(call.IndexCount, 1, 0, 0, 0)
}

func (r *Renderer) EndFrame(cmdBuf *sdl.GPUCommandBuffer, renderPass *sdl.GPURenderPass) {
	renderPass.End()
	cmdBuf.Submit()
}

// --- Lit rendering methods ---

func (r *Renderer) BeginLitFrame() (*sdl.GPUCommandBuffer, error) {
	device := r.window.Device()

	cmdBuf, err := device.AcquireCommandBuffer()
	if err != nil {
		return nil, errors.New("failed to acquire command buffer: " + err.Error())
	}

	return cmdBuf, nil
}

func (r *Renderer) BeginScenePass(cmdBuf *sdl.GPUCommandBuffer) *sdl.GPURenderPass {
	colorTargetInfo := sdl.GPUColorTargetInfo{
		Texture:    r.offscreenTexture,
		ClearColor: sdl.FColor{R: 0.02, G: 0.01, B: 0.02, A: 1.0},
		LoadOp:     sdl.GPU_LOADOP_CLEAR,
		StoreOp:    sdl.GPU_STOREOP_STORE,
	}

	depthStencilTargetInfo := sdl.GPUDepthStencilTargetInfo{
		Texture:        r.offscreenDepth,
		ClearDepth:     0.0, // reversed-Z: far=0, near=1
		LoadOp:         sdl.GPU_LOADOP_CLEAR,
		StoreOp:        sdl.GPU_STOREOP_DONT_CARE,
		StencilLoadOp:  sdl.GPU_LOADOP_DONT_CARE,
		StencilStoreOp: sdl.GPU_STOREOP_DONT_CARE,
		Cycle:          true,
	}

	renderPass := cmdBuf.BeginRenderPass(
		[]sdl.GPUColorTargetInfo{colorTargetInfo},
		&depthStencilTargetInfo,
	)

	renderPass.BindGraphicsPipeline(r.litPipeline)
	r.bindGroundTexture(renderPass)

	return renderPass
}

func (r *Renderer) bindGroundTexture(renderPass *sdl.GPURenderPass) {
	renderPass.BindFragmentSamplers([]sdl.GPUTextureSamplerBinding{
		{Texture: r.groundTexture, Sampler: r.repeatSampler},
		{Texture: r.placeholderTexture, Sampler: r.repeatSampler},
	})
}

// BindBuildingAtlas binds the ground texture + a building atlas texture for textured rendering.
func (r *Renderer) BindBuildingAtlas(renderPass *sdl.GPURenderPass, atlas *sdl.GPUTexture) {
	renderPass.BindFragmentSamplers([]sdl.GPUTextureSamplerBinding{
		{Texture: r.groundTexture, Sampler: r.repeatSampler},
		{Texture: atlas, Sampler: r.repeatSampler},
	})
}

// CreateTextureFromRGBA creates a GPU texture from RGBA pixel data.
func (r *Renderer) CreateTextureFromRGBA(width, height uint32, pixels []byte) (*sdl.GPUTexture, error) {
	return createTextureFromPixels(r.window.Device(), width, height, pixels)
}

// ReleaseTexture releases a GPU texture.
func (r *Renderer) ReleaseTexture(tex *sdl.GPUTexture) {
	r.window.Device().ReleaseTexture(tex)
}

func (r *Renderer) PushLightUniforms(cmdBuf *sdl.GPUCommandBuffer, lights LightUniforms) {
	cmdBuf.PushFragmentUniformData(0, unsafe.Slice(
		(*byte)(unsafe.Pointer(&lights)), unsafe.Sizeof(lights),
	))
}

func (r *Renderer) DrawLit(cmdBuf *sdl.GPUCommandBuffer, renderPass *sdl.GPURenderPass, call LitDrawCall) {
	if call.NoDepthWrite {
		renderPass.BindGraphicsPipeline(r.litNoDepthWritePipeline)
		r.bindGroundTexture(renderPass)
	} else if call.DepthBias {
		renderPass.BindGraphicsPipeline(r.litDepthBiasPipeline)
		r.bindGroundTexture(renderPass)
	}

	uniforms := LitVertexUniforms{
		MVP:   call.MVP,
		Model: call.Model,
	}
	var noFogFlag float32
	if call.NoFog {
		noFogFlag = 1
	}
	uniforms.Flags = mgl32.Vec4{noFogFlag, call.FadeFactor, float32(call.SurfaceType), call.Highlight}
	cmdBuf.PushVertexUniformData(0, unsafe.Slice(
		(*byte)(unsafe.Pointer(&uniforms)), unsafe.Sizeof(uniforms),
	))

	renderPass.BindVertexBuffers([]sdl.GPUBufferBinding{
		{Buffer: call.VertexBuffer, Offset: 0},
	})

	indexSize := sdl.GPU_INDEXELEMENTSIZE_16BIT
	if call.Index32 {
		indexSize = sdl.GPU_INDEXELEMENTSIZE_32BIT
	}
	renderPass.BindIndexBuffer(&sdl.GPUBufferBinding{
		Buffer: call.IndexBuffer, Offset: 0,
	}, indexSize)

	renderPass.DrawIndexedPrimitives(call.IndexCount, 1, 0, 0, 0)

	if call.NoDepthWrite || call.DepthBias {
		renderPass.BindGraphicsPipeline(r.litPipeline)
		r.bindGroundTexture(renderPass)
	}
}

func (r *Renderer) EndScenePass(renderPass *sdl.GPURenderPass) {
	renderPass.End()
}

func (r *Renderer) RunPostProcess(cmdBuf *sdl.GPUCommandBuffer, swapchainTexture *sdl.GPUTexture, uniforms PostProcessUniforms) {
	uniforms.Resolution = mgl32.Vec4{float32(r.offscreenW), float32(r.offscreenH), 0, 0}

	colorTargetInfo := sdl.GPUColorTargetInfo{
		Texture: swapchainTexture,
		LoadOp:  sdl.GPU_LOADOP_DONT_CARE,
		StoreOp: sdl.GPU_STOREOP_STORE,
	}

	renderPass := cmdBuf.BeginRenderPass(
		[]sdl.GPUColorTargetInfo{colorTargetInfo},
		nil,
	)

	renderPass.BindGraphicsPipeline(r.postProcessPipeline)

	renderPass.BindFragmentSamplers([]sdl.GPUTextureSamplerBinding{
		{Texture: r.offscreenTexture, Sampler: r.nearestSampler},
	})

	cmdBuf.PushFragmentUniformData(0, unsafe.Slice(
		(*byte)(unsafe.Pointer(&uniforms)), unsafe.Sizeof(uniforms),
	))

	renderPass.DrawPrimitives(3, 1, 0, 0)
	renderPass.End()
}

func (r *Renderer) EndLitFrame(cmdBuf *sdl.GPUCommandBuffer) {
	cmdBuf.Submit()
}

// --- UI overlay rendering methods ---

func (r *Renderer) initUIPipeline() error {
	device := r.window.Device()

	vertShader, err := shaders.LoadShader(device, "PositionColorTransform.vert", 0, 1, 0, 0)
	if err != nil {
		return errors.New("failed to create UI vertex shader: " + err.Error())
	}
	defer device.ReleaseShader(vertShader)

	fragShader, err := shaders.LoadShader(device, "SolidColor.frag", 0, 0, 0, 0)
	if err != nil {
		return errors.New("failed to create UI fragment shader: " + err.Error())
	}
	defer device.ReleaseShader(fragShader)

	pipeline, err := device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{
					Format: device.SwapchainTextureFormat(r.window.Handle()),
					BlendState: sdl.GPUColorTargetBlendState{
						EnableBlend:         true,
						SrcColorBlendfactor: sdl.GPU_BLENDFACTOR_SRC_ALPHA,
						DstColorBlendfactor: sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA,
						ColorBlendOp:        sdl.GPU_BLENDOP_ADD,
						SrcAlphaBlendfactor: sdl.GPU_BLENDFACTOR_ONE,
						DstAlphaBlendfactor: sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA,
						AlphaBlendOp:        sdl.GPU_BLENDOP_ADD,
					},
				},
			},
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{
				{
					Slot:      0,
					InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
					Pitch:     uint32(unsafe.Sizeof(Vertex{})),
				},
			},
			VertexAttributes: []sdl.GPUVertexAttribute{
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Location: 0, Offset: 0},
				{BufferSlot: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM, Location: 1, Offset: uint32(unsafe.Sizeof(float32(0)) * 3)},
			},
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode: sdl.GPU_FILLMODE_FILL,
			CullMode: sdl.GPU_CULLMODE_NONE,
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   vertShader,
		FragmentShader: fragShader,
	})
	if err != nil {
		return errors.New("failed to create UI pipeline: " + err.Error())
	}
	r.uiPipeline = pipeline
	return nil
}

func (r *Renderer) BeginUIPass(cmdBuf *sdl.GPUCommandBuffer, swapchainTexture *sdl.GPUTexture) *sdl.GPURenderPass {
	renderPass := cmdBuf.BeginRenderPass(
		[]sdl.GPUColorTargetInfo{
			{
				Texture: swapchainTexture,
				LoadOp:  sdl.GPU_LOADOP_LOAD,
				StoreOp: sdl.GPU_STOREOP_STORE,
			},
		},
		nil,
	)
	renderPass.BindGraphicsPipeline(r.uiPipeline)
	return renderPass
}

func (r *Renderer) DrawUI(cmdBuf *sdl.GPUCommandBuffer, renderPass *sdl.GPURenderPass, call DrawCall) {
	cmdBuf.PushVertexUniformData(0, unsafe.Slice(
		(*byte)(unsafe.Pointer(&call.Transform)), unsafe.Sizeof(call.Transform),
	))
	renderPass.BindVertexBuffers([]sdl.GPUBufferBinding{
		{Buffer: call.VertexBuffer, Offset: 0},
	})
	renderPass.BindIndexBuffer(&sdl.GPUBufferBinding{
		Buffer: call.IndexBuffer, Offset: 0,
	}, sdl.GPU_INDEXELEMENTSIZE_16BIT)
	renderPass.DrawIndexedPrimitives(call.IndexCount, 1, 0, 0, 0)
}

func (r *Renderer) EndUIPass(renderPass *sdl.GPURenderPass) {
	renderPass.End()
}

func (r *Renderer) Window() *window.Window {
	return r.window
}

func (r *Renderer) ReleaseBuffer(buffer *sdl.GPUBuffer) {
	r.window.Device().ReleaseBuffer(buffer)
}

func (r *Renderer) Destroy() {
	device := r.window.Device()

	if r.groundTexture != nil {
		device.ReleaseTexture(r.groundTexture)
	}
	if r.placeholderTexture != nil {
		device.ReleaseTexture(r.placeholderTexture)
	}
	if r.repeatSampler != nil {
		device.ReleaseSampler(r.repeatSampler)
	}
	if r.nearestSampler != nil {
		device.ReleaseSampler(r.nearestSampler)
	}
	if r.offscreenDepth != nil {
		device.ReleaseTexture(r.offscreenDepth)
	}
	if r.offscreenTexture != nil {
		device.ReleaseTexture(r.offscreenTexture)
	}
	if r.uiPipeline != nil {
		device.ReleaseGraphicsPipeline(r.uiPipeline)
	}
	if r.postProcessPipeline != nil {
		device.ReleaseGraphicsPipeline(r.postProcessPipeline)
	}
	if r.litPipeline != nil {
		device.ReleaseGraphicsPipeline(r.litPipeline)
	}
	if r.depthTexture != nil {
		device.ReleaseTexture(r.depthTexture)
	}
	if r.pipeline != nil {
		device.ReleaseGraphicsPipeline(r.pipeline)
	}
}
