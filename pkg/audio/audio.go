package audio

import (
	"fmt"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
)

const (
	SampleRate = 44100
	channels   = 1
)

// Device wraps an SDL3 audio output device.
type Device struct {
	id   sdl.AudioDeviceID
	spec sdl.AudioSpec
}

// New opens the default audio playback device at 44100 Hz mono float32.
func New() (*Device, error) {
	spec := sdl.AudioSpec{
		Format:   sdl.AUDIO_F32,
		Channels: channels,
		Freq:     SampleRate,
	}
	devID, err := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDevice(&spec)
	if err != nil {
		return nil, fmt.Errorf("audio: open device: %w", err)
	}
	return &Device{id: devID, spec: spec}, nil
}

// NewStream creates an AudioStream bound to this device.
// The caller owns the stream and must call stream.Destroy() when done.
func (d *Device) NewStream() (*sdl.AudioStream, error) {
	stream, err := sdl.CreateAudioStream(&d.spec, &d.spec)
	if err != nil {
		return nil, fmt.Errorf("audio: create stream: %w", err)
	}
	if err := d.id.BindAudioStream(stream); err != nil {
		stream.Destroy()
		return nil, fmt.Errorf("audio: bind stream: %w", err)
	}
	return stream, nil
}

// Destroy closes the audio device.
func (d *Device) Destroy() {
	d.id.Close()
}

// PushSamples converts a float32 sample buffer to bytes and pushes to the stream.
func PushSamples(stream *sdl.AudioStream, samples []float32) error {
	if len(samples) == 0 {
		return nil
	}
	byteSlice := unsafe.Slice(
		(*byte)(unsafe.Pointer(&samples[0])),
		len(samples)*4,
	)
	return stream.PutData(byteSlice)
}
