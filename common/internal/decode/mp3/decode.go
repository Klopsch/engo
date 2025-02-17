package mp3

import (
	"github.com/hajimehoshi/go-mp3"

	"github.com/klopsch/engo/common/internal/decode/convert"
)

// Stream is a decoded stream.
type Stream struct {
	orig       *mp3.Decoder
	resampling *convert.Resampling
}

// Read is implementation of io.Reader's Read.
func (s *Stream) Read(buf []byte) (int, error) {
	if s.resampling != nil {
		return s.resampling.Read(buf)
	}
	return s.orig.Read(buf)
}

// Seek is implementation of io.Seeker's Seek.
func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	if s.resampling != nil {
		return s.resampling.Seek(offset, whence)
	}
	return s.orig.Seek(offset, whence)
}

// Close is implementation of io.Closer's Close.
func (s *Stream) Close() error {
	if s.resampling != nil {
		return s.resampling.Close()
	}
	return nil
}

// Length returns the size of decoded stream in bytes.
func (s *Stream) Length() int64 {
	if s.resampling != nil {
		return s.resampling.Length()
	}
	return s.orig.Length()
}

// Size is deprecated as of 1.6.0-alpha. Use Length instead.
func (s *Stream) Size() int64 {
	return s.Length()
}

// Decode decodes MP3 source and returns a decoded stream.
//
// Decode returns error when decoding fails or IO error happens.
//
// Decode automatically resamples the stream to fit with the audio context if necessary.
func Decode(src convert.ReadSeekCloser, sr int) (*Stream, error) {
	d, err := mp3.NewDecoder(src)
	if err != nil {
		return nil, err
	}
	var r *convert.Resampling
	stream := &Stream{
		orig:       d,
		resampling: r,
	}
	if d.SampleRate() != sr {
		stream.resampling = convert.NewResampling(stream, stream.orig.Length(), stream.orig.SampleRate(), sr)
	}
	return stream, nil
}
