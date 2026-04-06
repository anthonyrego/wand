package shaders

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed compiled/spirv/*.spv compiled/msl/*.msl compiled/dxil/*.dxil
var shaderFS embed.FS

func LoadShader(
	device *sdl.GPUDevice,
	shaderFilename string,
	samplerCount uint32,
	uniformBufferCount uint32,
	storageBufferCount uint32,
	storageTextureCount uint32,
) (*sdl.GPUShader, error) {
	var stage sdl.GPUShaderStage
	if strings.Contains(shaderFilename, ".vert") {
		stage = sdl.GPU_SHADERSTAGE_VERTEX
	} else if strings.Contains(shaderFilename, ".frag") {
		stage = sdl.GPU_SHADERSTAGE_FRAGMENT
	} else {
		return nil, errors.New("invalid shader stage")
	}

	path := ""
	backendFormats := device.ShaderFormats()
	format := sdl.GPU_SHADERFORMAT_INVALID
	entrypoint := ""

	if backendFormats&sdl.GPU_SHADERFORMAT_SPIRV == sdl.GPU_SHADERFORMAT_SPIRV {
		path = fmt.Sprintf("compiled/spirv/%s.spv", shaderFilename)
		format = sdl.GPU_SHADERFORMAT_SPIRV
		entrypoint = "main"
	} else if backendFormats&sdl.GPU_SHADERFORMAT_MSL == sdl.GPU_SHADERFORMAT_MSL {
		path = fmt.Sprintf("compiled/msl/%s.msl", shaderFilename)
		format = sdl.GPU_SHADERFORMAT_MSL
		entrypoint = "main0"
	} else if backendFormats&sdl.GPU_SHADERFORMAT_DXIL == sdl.GPU_SHADERFORMAT_DXIL {
		path = fmt.Sprintf("compiled/dxil/%s.dxil", shaderFilename)
		format = sdl.GPU_SHADERFORMAT_DXIL
		entrypoint = "main"
	} else {
		return nil, errors.New("unrecognized backend shader format")
	}

	code, err := shaderFS.ReadFile(path)
	if err != nil {
		return nil, errors.New("failed to open shader: " + err.Error())
	}

	shaderInfo := sdl.GPUShaderCreateInfo{
		Code:               code,
		Entrypoint:         entrypoint,
		Format:             format,
		Stage:              stage,
		NumSamplers:        samplerCount,
		NumUniformBuffers:  uniformBufferCount,
		NumStorageBuffers:  storageBufferCount,
		NumStorageTextures: storageTextureCount,
	}

	shader, err := device.CreateGPUShader(&shaderInfo)
	if err != nil {
		return nil, errors.New("failed to create shader: " + err.Error())
	}

	return shader, nil
}
