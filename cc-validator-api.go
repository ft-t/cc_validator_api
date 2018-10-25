package sl500_api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/howeyc/crc16"
	"github.com/tarm/serial"
)

const StartCode byte = 0x02
const PeripheralAddress byte = 0x03

type Baud int

const (
	Baud9600  Baud = 9600
	Baud19200 Baud = 19200
)

type CCValidator struct {
	config *serial.Config
	port   *serial.Port
}

func NewConnection(path string, baud Baud) (CCValidator, error) {
	c := &serial.Config{Name: path, Baud: int(baud), ReadTimeout: 5 * time.Second} // TODO
	o, err := serial.OpenPort(c)

	res := CCValidator{}

	if err != nil {
		return res, err
	}

	res.config = c
	res.port = o

	_, err = res.Reset()
	if err != nil {
		return res, err
	}

	return res, nil
}

func (s *CCValidator) Reset() ([]byte, error) {
	sendRequest(s.port, 0x30, []byte{})
	return readResponse(s.port)
}

func (s *CCValidator) GetStatus() ([]byte, error) {
	sendRequest(s.port, 0x31, []byte{})
	return readResponse(s.port)
}

func (s *CCValidator) SetSecurity(data []byte) ([]byte, error) {
	sendRequest(s.port, 0x32, data)
	return readResponse(s.port)
}

func (s *CCValidator) Poll() ([]byte, error) {
	sendRequest(s.port, 0x33, []byte{})
	return readResponse(s.port)
}

func (s *CCValidator) Ack() ([]byte, error) {
	sendRequest(s.port, 0x00, []byte{})
	return readResponse(s.port)
}

func (s *CCValidator) Nack() ([]byte, error) {
	sendRequest(s.port, 0xFF, []byte{})
	return readResponse(s.port)
}

func readResponse(port *serial.Port) ([]byte, error) {
	var buf []byte
	innerBuf := make([]byte, 256)

	totalRead := 0
	readTriesCount := 0
	maxReadCount := 50

	for ; ; {
		readTriesCount += 1

		if readTriesCount >= maxReadCount {
			return nil, fmt.Errorf("Reads tries exceeded")
		}

		n, err := port.Read(innerBuf)

		if err != nil {
			return nil, err
		}

		totalRead += n
		buf = append(buf, innerBuf[:n]...)

		if totalRead < 6 {
			continue
		}
		if buf[2] != 0x0 && int(buf[2]) != len(buf) {
			continue
		}

		break
	}

	if buf[0] != StartCode || buf[1] != PeripheralAddress {
		return nil, fmt.Errorf("Response format invalid")
	}

	crc := binary.BigEndian.Uint16(buf[len(buf)-2:])

	buf = buf[:len(buf)-2]

	crc2 := crc16.ChecksumCCITT(buf)

	if crc != crc2 {
		return nil, fmt.Errorf("Response verification failed")
	}

	if buf[3] != 0x00 {
		return nil, nil // TODO Ack
	}

	if buf[3] != 0xFF {
		return nil, fmt.Errorf("Nack")
	}

	if buf[3] != 0x30 {
		return nil, fmt.Errorf("Illegal command")
	}

	buf = buf[3:]

	return buf, nil
}

func sendRequest(port *serial.Port, commandCode int16, bytesData ...[]byte) {
	buf := new(bytes.Buffer)

	length := 6

	for _, b := range bytesData {
		length += len(b)
	}

	binary.Write(buf, binary.BigEndian, StartCode)
	binary.Write(buf, binary.BigEndian, PeripheralAddress)
	binary.Write(buf, binary.BigEndian, byte(length))
	binary.Write(buf, binary.BigEndian, commandCode)

	for _, data := range bytesData {
		binary.Write(buf, binary.BigEndian, data)
	}

	crc := crc16.ChecksumCCITT(buf.Bytes())

	binary.Write(buf, binary.BigEndian, crc)

	port.Write(buf.Bytes())
}
