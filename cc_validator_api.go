package cc_validator_api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/tarm/serial"
)

const StartCode byte = 0x02
const PeripheralAddress byte = 0x03

type Baud int

const (
	Baud9600  Baud = 9600
	Baud19200 Baud = 19200
)

type Status byte

const (
	PowerUp                   Status = 0x10
	PowerUpWithBillValidator  Status = 0x11
	PowerUpWithBillStacker    Status = 0x12
	Initialize                Status = 0x13
	Idling                    Status = 0x14
	Accepting                 Status = 0x15
	Stacking                  Status = 0x17
	Returning                 Status = 0x18
	UnitDisabled              Status = 0x19
	Holding                   Status = 0x1A
	DeviceBusy                Status = 0x1B
	Rejecting                 Status = 0x1C
	DropCassetteFull          Status = 0x41
	DropCassetteOutOfPosition Status = 0x42
	ValidatorJammed           Status = 0x43
	DropCassetteJammed        Status = 0x44
	Cheated                   Status = 0x45
	GenericFailure            Status = 0x47
	EscrowPosition            Status = 0x80
	BillStacked               Status = 0x81
	BillReturned              Status = 0x82
)

//Rejecting Codes
const (
	DueToInsertion          byte = 0x60
	DueToMagnetic           byte = 0x61
	DueToRemainedBillInHead byte = 0x62
	DueToMultiplying        byte = 0x63
	DueToConveying          byte = 0x64
	DueToIdentification1    byte = 0x65
	DueToVerification       byte = 0x66
	DueToOptic              byte = 0x67
	DueToInhibit            byte = 0x68
	DueToCapacity           byte = 0x69
	DueToOperation          byte = 0x6A
	DueToLength             byte = 0x6C
)

//Failure Codes
const (
	StackMotorFailure            byte = 0x50
	TransportMotorSpeedFailure   byte = 0x51
	TransportMotorFailure        byte = 0x52
	AligningMotorFailure         byte = 0x53
	InitialCassetteStatusFailure byte = 0x54
	OpticCanalFailure            byte = 0x55
	MagneticCanalFailure         byte = 0x56
	CapacitanceCanalFailure      byte = 0x5F
)

type Identification struct {
	PartNumber   string
	SerialNumber string
	AssetNumber  []byte
}

type Bill struct {
	Denomination float64
	CountryCode  string
}

type CCValidator struct {
	config  *serial.Config
	port    *serial.Port
	logging bool
	open    bool
}

func NewConnection(path string, baud Baud, logging bool) (CCValidator, error) {
	c := &serial.Config{Name: path, Baud: int(baud), ReadTimeout: 5 * time.Second} // TODO
	o, err := serial.OpenPort(c)

	res := CCValidator{}

	if err != nil {
		return res, err
	}

	res.config = c
	res.port = o
	res.logging = logging
	res.open = true

	return res, nil
}

func (s *CCValidator) Open() error {
	if s.open {
		return errors.New("port already opened")
	}

	p, err := serial.OpenPort(s.config)

	if err != nil {
		return err
	}

	s.port = p
	s.open = true

	return nil
}

func (s *CCValidator) Close() error {
	if s.port == nil || !s.open {
		return errors.New("port not opened")
	}

	err := s.port.Close()
	s.open = false

	return err
}

func (s *CCValidator) Reset() error {
	err := sendRequest(s, 0x30, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) GetStatus() ([]uint, []uint, error) {
	err := sendRequest(s, 0x31, []byte{})

	if err != nil {
		return nil, nil, err
	}

	response, err := readResponse(s)

	if err != nil {
		return nil, nil, err
	}

	var enabledBills []uint
	var securityBills []uint

	for b, v := range response[:3] {
		shift := uint(16 - b*8)
		for i := uint(0); i < 8; i++ {
			if v&(1<<i) != 0 {
				enabledBills = append(enabledBills, shift+7-i)
			}
		}
	}

	for b, v := range response[4:] {
		shift := uint(16 - b*8)
		for i := uint(0); i < 8; i++ {
			if v&(1<<i) != 0 {
				securityBills = append(securityBills, shift+7-i)
			}
		}
	}

	return enabledBills, securityBills, nil
}

func (s *CCValidator) SetSecurity(security []byte) error {
	securityBytes := []byte{0, 0, 0}

	for _, t := range security {
		pos := 23 - t
		securityBytes[pos/8] |= 1 << (7 - pos + pos/8*8)
	}

	err := sendRequest(s, 0x32, securityBytes)

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) Poll() (Status, byte, error) {
	err := sendRequest(s, 0x33, []byte{})

	if err != nil {
		return 0, 0, err
	}

	response, err := readResponse(s)

	if err != nil {
		return 0, 0, err
	}

	param := byte(0)
	if len(response) > 1 {
		param = response[1]
	}

	return Status(response[0]), param, err
}

func (s *CCValidator) Identification() (Identification, error) {
	err := sendRequest(s, 0x37, []byte{})

	if err != nil {
		return Identification{}, err
	}

	response, err := readResponse(s)

	if err != nil {
		return Identification{}, err
	}

	return Identification{
		PartNumber:   string(response[:15]),
		SerialNumber: string(response[16:27]),
		AssetNumber:  response[28:34],
	}, nil
}

func (s *CCValidator) GetBillTable() ([]Bill, error) {
	err := sendRequest(s, 0x41, []byte{})

	if err != nil {
		return nil, err
	}

	response, err := readResponse(s)

	if err != nil {
		return nil, err
	}

	var bills []Bill

	for i := 0; i < 24; i++ {
		first := response[i*5]
		countryCode := string(response[i*5+1 : i*5+4])
		secondByte := response[i*5+4]

		second := 0
		if secondByte > 0x80 {
			second = -int(secondByte - 0x80)
		} else {
			second = int(secondByte)
		}

		bill := Bill{Denomination: float64(first) * math.Pow(10, float64(second)), CountryCode: countryCode}
		bills = append(bills, bill)
	}

	return bills, nil
}

func (s *CCValidator) EnableBillTypes(enabled []uint, escrow []uint) error {
	enabledBytes := []byte{0, 0, 0}
	escrowBytes := []byte{0, 0, 0}

	for _, t := range enabled {
		pos := 23 - t
		enabledBytes[pos/8] |= 1 << (7 - pos + pos/8*8)
	}

	for _, t := range escrow {
		pos := 23 - t
		escrowBytes[pos/8] |= 1 << (7 - pos + pos/8*8)
	}

	err := sendRequest(s, 0x34, append(enabledBytes, escrowBytes...))

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) Stack() error {
	err := sendRequest(s, 0x35, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) Return() error {
	err := sendRequest(s, 0x36, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) Hold() error {
	err := sendRequest(s, 0x38, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) GetCRC32() ([]byte, error) {
	err := sendRequest(s, 0x51, []byte{})

	if err != nil {
		return nil, err
	}

	return readResponse(s)
}

func (s *CCValidator) SetBarcodeParameters(format byte, numberOfCharacters byte) error {
	err := sendRequest(s, 0x3A, []byte{format, numberOfCharacters})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func (s *CCValidator) ExtractBarcodeData() ([]byte, error) {
	err := sendRequest(s, 0x3A, []byte{})

	if err != nil {
		return nil, err
	}

	return readResponse(s)
}

func (s *CCValidator) Ack() error {
	err := sendRequest(s, 0x00, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func Ack(s *CCValidator) {
	_ = sendRequest(s, 0x00, []byte{})
}

func (s *CCValidator) Nack() error {
	err := sendRequest(s, 0xFF, []byte{})

	if err != nil {
		return err
	}

	_, err = readResponse(s)
	return err
}

func readResponse(v *CCValidator) ([]byte, error) {
	var buf []byte
	innerBuf := make([]byte, 256)

	totalRead := 0
	readTriesCount := 0
	maxReadCount := 1050

	for ; ; {
		readTriesCount += 1

		if readTriesCount >= maxReadCount {
			return nil, fmt.Errorf("Reads tries exceeded")
		}

		n, err := v.port.Read(innerBuf)

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

	crc := binary.LittleEndian.Uint16(buf[len(buf)-2:])

	buf = buf[:len(buf)-2]

	crc2 := GetCRC16(buf)

	if crc != crc2 {
		return nil, fmt.Errorf("Response verification failed")
	}

	if len(buf) == 4 && buf[3] == 0x00 {
		if v.logging {
			fmt.Printf("validator[%v]: <- %X\n", v.config.Name, buf)
		}
		return nil, nil // TODO Ack
	}

	if len(buf) == 4 && buf[3] == 0xFF {
		return nil, fmt.Errorf("Nack")
	}

	if len(buf) == 4 && buf[3] == 0x30 {
		return nil, fmt.Errorf("Illegal command")
	}

	buf = buf[3:]

	if v.logging {
		fmt.Printf("validator[%v]: <- %X\n", v.config.Name, buf)
	}

	Ack(v)

	return buf, nil
}

func sendRequest(v *CCValidator, commandCode byte, bytesData ...[]byte) error {
	if !v.open {
		return errors.New("serial port is closed")
	}

	buf := new(bytes.Buffer)

	length := 6

	for _, b := range bytesData {
		length += len(b)
	}

	_ = binary.Write(buf, binary.LittleEndian, StartCode)
	_ = binary.Write(buf, binary.LittleEndian, PeripheralAddress)
	_ = binary.Write(buf, binary.LittleEndian, byte(length))
	_ = binary.Write(buf, binary.LittleEndian, commandCode)

	for _, data := range bytesData {
		_ = binary.Write(buf, binary.LittleEndian, data)
	}

	crc := GetCRC16(buf.Bytes())

	_ = binary.Write(buf, binary.LittleEndian, crc)

	if v.logging {
		fmt.Printf("validator[%v]:-> %X\n", v.config.Name, buf.Bytes())
	}

	_, err := v.port.Write(buf.Bytes())
	return err
}

func GetCRC16(bufData []byte) uint16 {
	CRC := uint16(0)
	for i := 0; i < len(bufData); i++ {
		TmpCRC := CRC ^ uint16(bufData[i])
		for j := 0; j < 8; j++ {
			if (TmpCRC & 0x0001) > 0 {
				TmpCRC >>= 1
				TmpCRC ^= 0x08408
			} else {
				TmpCRC >>= 1
			}
		}
		CRC = TmpCRC
	}
	return CRC
}
