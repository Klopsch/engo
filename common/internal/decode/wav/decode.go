// Package wav provides WAV (RIFF) decoder.
package wav

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klopsch/engo/common/internal/decode/convert"
)

// Stream is a decoded audio stream.
type Stream struct {
	inner convert.ReadSeekCloser
	size  int64
}

// Read is implementation of io.Reader's Read.
func (s *Stream) Read(p []byte) (int, error) {
	return s.inner.Read(p)
}

// Seek is implementation of io.Seeker's Seek.
//
// Note that Seek can take long since decoding is a relatively heavy task.
func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	return s.inner.Seek(offset, whence)
}

// Close is implementation of io.Closer's Close.
func (s *Stream) Close() error {
	return s.inner.Close()
}

// Length returns the size of decoded stream in bytes.
func (s *Stream) Length() int64 {
	return s.size
}

// Size is deprecated as of version 1.6.0-alpha. Use Length instead.
func (s *Stream) Size() int64 {
	return s.Length()
}

type stream struct {
	src        convert.ReadSeekCloser
	headerSize int64
	dataSize   int64
	remaining  int64
}

// Read is implementation of io.Reader's Read.
func (s *stream) Read(p []byte) (int, error) {
	if s.remaining <= 0 {
		return 0, io.EOF
	}
	if s.remaining < int64(len(p)) {
		p = p[0:s.remaining]
	}
	n, err := s.src.Read(p)
	s.remaining -= int64(n)
	return n, err
}

// Seek is implementation of io.Seeker's Seek.
func (s *stream) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		offset = offset + s.headerSize
	case io.SeekCurrent:
	case io.SeekEnd:
		offset = s.headerSize + s.dataSize + offset
		whence = io.SeekStart
	}
	n, err := s.src.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	if n-s.headerSize < 0 {
		return 0, fmt.Errorf("wav: invalid offset")
	}
	s.remaining = s.dataSize - (n - s.headerSize)
	// There could be a tail in wav file.
	if s.remaining < 0 {
		s.remaining = 0
		return s.dataSize, nil
	}
	return n - s.headerSize, nil
}

// Close is implementation of io.Closer's Close.
func (s *stream) Close() error {
	return s.src.Close()
}

// Length returns the size of decoded stream in bytes.
func (s *stream) Length() int64 {
	return s.dataSize
}

// Decode decodes WAV (RIFF) data to playable stream.
//
// The format must be 1 or 2 channels, 8bit or 16bit little endian PCM.
// The format is converted into 2 channels and 16bit.
//
// Decode returns error when decoding fails or IO error happens.
//
// Decode automatically resamples the stream to fit with the audio context if necessary.
func Decode(src convert.ReadSeekCloser, sr int) (*Stream, error) {
	buf := make([]byte, 12)
	n, err := io.ReadFull(src, buf)
	if n != len(buf) {
		return nil, fmt.Errorf("wav: invalid header")
	}
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(buf[0:4], []byte("RIFF")) {
		return nil, fmt.Errorf("wav: invalid header: 'RIFF' not found")
	}
	if !bytes.Equal(buf[8:12], []byte("WAVE")) {
		return nil, fmt.Errorf("wav: invalid header: 'WAVE' not found")
	}

	// Read chunks
	dataSize := int64(0)
	headerSize := int64(len(buf))
	sampleRateFrom := 0
	sampleRateTo := 0
	mono := false
	bitsPerSample := 0
chunks:
	for {
		buf := make([]byte, 8)
		n, err := io.ReadFull(src, buf)
		if n != len(buf) {
			return nil, fmt.Errorf("wav: invalid header")
		}
		if err != nil {
			return nil, err
		}
		headerSize += 8
		size := int64(buf[4]) | int64(buf[5])<<8 | int64(buf[6])<<16 | int64(buf[7])<<24
		switch {
		case bytes.Equal(buf[0:4], []byte("fmt ")):
			// Size of 'fmt' header is usually 16, but can be more than 16.
			if size < 16 {
				return nil, fmt.Errorf("wav: invalid header: maybe non-PCM file?")
			}
			buf2 := make([]byte, size)
			n, err := io.ReadFull(src, buf2)
			if n != len(buf2) {
				return nil, fmt.Errorf("wav: invalid header")
			}
			if err != nil {
				return nil, err
			}
			format := int(buf2[0]) | int(buf2[1])<<8
			if format != 1 {
				return nil, fmt.Errorf("wav: format must be linear PCM")
			}
			channelNum := int(buf2[2]) | int(buf2[3])<<8
			switch channelNum {
			case 1:
				mono = true
			case 2:
				mono = false
			default:
				return nil, fmt.Errorf("wav: channel num must be 1 or 2 but was %d", channelNum)
			}
			bitsPerSample = int(buf2[14]) | int(buf2[15])<<8
			if bitsPerSample != 8 && bitsPerSample != 16 {
				return nil, fmt.Errorf("wav: bits per sample must be 8 or 16 but was %d", bitsPerSample)
			}
			sampleRate := int64(buf2[4]) | int64(buf2[5])<<8 | int64(buf2[6])<<16 | int64(buf2[7])<<24
			if int64(sr) != sampleRate {
				sampleRateFrom = int(sampleRate)
				sampleRateTo = sr
			}
			headerSize += size
		case bytes.Equal(buf[0:4], []byte("data")):
			dataSize = size
			break chunks
		default:
			buf := make([]byte, size)
			n, err := io.ReadFull(src, buf)
			if n != len(buf) {
				return nil, fmt.Errorf("wav: invalid header")
			}
			if err != nil {
				return nil, err
			}
			headerSize += size
		}
	}
	var s convert.ReadSeekCloser = &stream{
		src:        src,
		headerSize: headerSize,
		dataSize:   dataSize,
		remaining:  dataSize,
	}
	if mono || bitsPerSample != 16 {
		s = convert.NewStereo16(s, mono, bitsPerSample != 16)
		if mono {
			dataSize *= 2
		}
		if bitsPerSample != 16 {
			dataSize *= 2
		}
	}
	if sampleRateFrom != sampleRateTo {
		r := convert.NewResampling(s, dataSize, sampleRateFrom, sampleRateTo)
		s = r
		dataSize = r.Length()
	}
	return &Stream{inner: s, size: dataSize}, nil
}
