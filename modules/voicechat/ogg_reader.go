package voicechat

import (
	"encoding/binary"
	"io"
)

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
