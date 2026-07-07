package voicechat

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	customOggPageHeaderTypeBeginningOfStream = 0x02
	customOggPageHeaderSignature             = "OggS"

	customOggIdPageBasePayloadLength = 19
	customOggPageHeaderLen           = 27
)

var (
	customOggErrNilStream                       = errors.New("stream is nil")
	customOggErrBadIDPageSignature              = errors.New("bad header signature")
	customOggErrBadIDPageType                   = errors.New("wrong header, expected beginning of stream")
	customOggErrBadIDPageLength                 = errors.New("payload for id page must be 19 bytes")
	customOggErrBadIDPagePayloadSignature       = errors.New("bad payload signature")
	customOggErrShortPageHeader                 = errors.New("not enough data for payload header")
	customOggErrChecksumMismatch                = errors.New("expected and actual checksum do not match")
	customOggErrUnsupportedChannelMappingFamily = errors.New("unsupported channel mapping family")
)

type CustomOggReader struct {
	stream                io.Reader
	bytesReadSuccessfully int64
	checksumTable         *[256]uint32
	doChecksum            bool
	lastSegSizes          []byte
}

type CustomOggHeader struct {
	ChannelMap     uint8
	Channels       uint8
	OutputGain     uint16
	PreSkip        uint16
	SampleRate     uint32
	Version        uint8
	StreamCount    uint8
	CoupledCount   uint8
	ChannelMapping string
}

type CustomOggPageHeader struct {
	GranulePosition uint64
	sig             [4]byte
	version         uint8
	headerType      uint8
	Serial          uint32
	index           uint32
	segmentsCount   uint8
}

type CustomOggHeaderType string

const (
	customOggHeaderUnknown  CustomOggHeaderType = ""
	CustomOggHeaderOpusID   CustomOggHeaderType = "OpusHead"
	CustomOggHeaderOpusTags CustomOggHeaderType = "OpusTags"
)

func customOggOpusPayloadSignature(payload []byte) (CustomOggHeaderType, bool) {
	if len(payload) < 8 {
		return customOggHeaderUnknown, false
	}

	sig := CustomOggHeaderType(payload[:8])
	if sig == CustomOggHeaderOpusID || sig == CustomOggHeaderOpusTags {
		return sig, true
	}

	return customOggHeaderUnknown, false
}

func CustomOggNewWith(in io.Reader) (*CustomOggReader, *CustomOggHeader, error) {
	return customOggNewWith(in, true)
}

func customOggNewWith(in io.Reader, doChecksum bool) (*CustomOggReader, *CustomOggHeader, error) {
	if in == nil {
		return nil, nil, customOggErrNilStream
	}

	reader := &CustomOggReader{
		stream:        in,
		checksumTable: customOggGenerateChecksumTable(),
		doChecksum:    doChecksum,
	}

	header, err := reader.readOpusHeader()
	if err != nil {
		return nil, nil, err
	}

	return reader, header, nil
}

func (o *CustomOggReader) readOpusHeader() (*CustomOggHeader, error) {
	payload, pageHeader, err := o.ParseNextPage()
	if err != nil {
		return nil, err
	}

	if err := customOggValidateOpusPageHeader(pageHeader, payload); err != nil {
		return nil, err
	}

	header := customOggParseBasicHeaderFields(payload)
	if err := customOggParseChannelMapping(header, payload); err != nil {
		return nil, err
	}

	return header, nil
}

func customOggValidateOpusPageHeader(pageHeader *CustomOggPageHeader, payload []byte) error {
	if string(pageHeader.sig[:]) != customOggPageHeaderSignature {
		return customOggErrBadIDPageSignature
	}

	if pageHeader.headerType != customOggPageHeaderTypeBeginningOfStream {
		return customOggErrBadIDPageType
	}

	if len(payload) < customOggIdPageBasePayloadLength {
		return customOggErrBadIDPageLength
	}

	if sig, ok := customOggOpusPayloadSignature(payload); !ok || sig != CustomOggHeaderOpusID {
		return fmt.Errorf("%w: expected OpusHead, got %s", customOggErrBadIDPagePayloadSignature, sig)
	}

	return nil
}

func customOggParseBasicHeaderFields(payload []byte) *CustomOggHeader {
	header := &CustomOggHeader{}
	header.Version = payload[8]
	header.Channels = payload[9]
	header.PreSkip = binary.LittleEndian.Uint16(payload[10:12])
	header.SampleRate = binary.LittleEndian.Uint32(payload[12:16])
	header.OutputGain = binary.LittleEndian.Uint16(payload[16:18])
	header.ChannelMap = payload[18]

	return header
}

func customOggParseChannelMapping(header *CustomOggHeader, payload []byte) error {
	switch header.ChannelMap {
	case 0:
		return customOggValidatePayloadLength(payload, customOggIdPageBasePayloadLength)
	case 1, 2, 255:
		return customOggParseExtendedChannelMapping(header, payload)
	case 3:
		return fmt.Errorf("%w: ambisonics family type 3 is not supported", customOggErrUnsupportedChannelMappingFamily)
	default:
		return customOggErrUnsupportedChannelMappingFamily
	}
}

func customOggValidatePayloadLength(payload []byte, expectedLen int) error {
	if len(payload) != expectedLen {
		return customOggErrBadIDPageLength
	}

	return nil
}

func customOggParseExtendedChannelMapping(header *CustomOggHeader, payload []byte) error {
	expectedPayloadLen := 21 + int(header.Channels)
	if err := customOggValidatePayloadLength(payload, expectedPayloadLen); err != nil {
		return err
	}

	header.StreamCount = payload[19]
	header.CoupledCount = payload[20]
	header.ChannelMapping = string(payload[21:expectedPayloadLen])

	return nil
}

func (o *CustomOggReader) ParseNextPage() ([]byte, *CustomOggPageHeader, error) {
	header := make([]byte, customOggPageHeaderLen)

	n, err := io.ReadFull(o.stream, header)
	if err != nil {
		return nil, nil, err
	} else if n < len(header) {
		return nil, nil, customOggErrShortPageHeader
	}

	pageHeader := &CustomOggPageHeader{
		sig: [4]byte{header[0], header[1], header[2], header[3]},
	}

	pageHeader.version = header[4]
	pageHeader.headerType = header[5]
	pageHeader.GranulePosition = binary.LittleEndian.Uint64(header[6 : 6+8])
	pageHeader.Serial = binary.LittleEndian.Uint32(header[14 : 14+4])
	pageHeader.index = binary.LittleEndian.Uint32(header[18 : 18+4])
	pageHeader.segmentsCount = header[26]

	sizeBuffer := make([]byte, pageHeader.segmentsCount)
	if _, err = io.ReadFull(o.stream, sizeBuffer); err != nil {
		return nil, nil, err
	}

	payloadSize := 0
	for _, s := range sizeBuffer {
		payloadSize += int(s)
	}

	payload := make([]byte, payloadSize)
	if _, err = io.ReadFull(o.stream, payload); err != nil {
		return nil, nil, err
	}

	if o.doChecksum {
		var checksum uint32
		updateChecksum := func(v byte) {
			checksum = (checksum << 8) ^ o.checksumTable[byte(checksum>>24)^v]
		}

		for index := range header {
			if index > 21 && index < 26 {
				updateChecksum(0)
				continue
			}
			updateChecksum(header[index])
		}
		for _, s := range sizeBuffer {
			updateChecksum(s)
		}
		for index := range payload {
			updateChecksum(payload[index])
		}

		if binary.LittleEndian.Uint32(header[22:22+4]) != checksum {
			return nil, nil, customOggErrChecksumMismatch
		}
	}

	o.bytesReadSuccessfully += int64(len(header) + len(sizeBuffer) + len(payload))
	o.lastSegSizes = sizeBuffer

	return payload, pageHeader, nil
}

func (o *CustomOggReader) ParseNextPageSegments() ([][]byte, *CustomOggPageHeader, error) {
	payload, hdr, err := o.ParseNextPage()
	if err != nil {
		return nil, nil, err
	}

	segs := make([][]byte, 0, hdr.segmentsCount)
	off, start := 0, 0
	inPacket := false
	for i := 0; i < int(hdr.segmentsCount); i++ {
		size := int(o.lastSegmentSizes(i))
		if !inPacket {
			start = off
			inPacket = true
		}
		off += size
		if size < 255 {
			segs = append(segs, payload[start:off])
			inPacket = false
		}
	}
	if inPacket {
		segs = append(segs, payload[start:off])
	}
	return segs, hdr, nil
}

func (o *CustomOggReader) lastSegmentSizes(i int) byte {
	if i < 0 || i >= len(o.lastSegSizes) {
		return 0
	}
	return o.lastSegSizes[i]
}

func (o *CustomOggReader) LastPageLastSegmentSize() byte {
	if len(o.lastSegSizes) == 0 {
		return 0
	}
	return o.lastSegSizes[len(o.lastSegSizes)-1]
}

func customOggGenerateChecksumTable() *[256]uint32 {
	var table [256]uint32
	const poly = 0x04c11db7

	for i := range table {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if (r & 0x80000000) != 0 {
				r = (r << 1) ^ poly
			} else {
				r <<= 1
			}
		}
		table[i] = r & 0xffffffff
	}

	return &table
}
