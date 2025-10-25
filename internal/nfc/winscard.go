// +build windows

package nfc

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winscard           = syscall.NewLazyDLL("winscard.dll")
	procEstablishCtx   = winscard.NewProc("SCardEstablishContext")
	procReleaseCtx     = winscard.NewProc("SCardReleaseContext")
	procListReaders    = winscard.NewProc("SCardListReadersW")
	procConnect        = winscard.NewProc("SCardConnectW")
	procDisconnect     = winscard.NewProc("SCardDisconnect")
	procTransmit       = winscard.NewProc("SCardTransmit")
	procGetStatusChange = winscard.NewProc("SCardGetStatusChangeW")
	procStatus         = winscard.NewProc("SCardStatusW")
)

const (
	SCARD_SCOPE_USER      = 0
	SCARD_SCOPE_TERMINAL  = 1
	SCARD_SCOPE_SYSTEM    = 2
	SCARD_SHARE_EXCLUSIVE = 1
	SCARD_SHARE_SHARED    = 2
	SCARD_LEAVE_CARD      = 0
	SCARD_RESET_CARD      = 1
	SCARD_UNPOWER_CARD    = 2
	SCARD_EJECT_CARD      = 3
	SCARD_PROTOCOL_T0     = 0x00000001
	SCARD_PROTOCOL_T1     = 0x00000002
	SCARD_PROTOCOL_RAW    = 0x00010000
	SCARD_PCI_T0          = 0
	SCARD_PCI_T1          = 1

	SCARD_STATE_UNAWARE   = 0x00000000
	SCARD_STATE_CHANGED   = 0x00000002
	SCARD_STATE_PRESENT   = 0x00000020
	SCARD_STATE_EMPTY     = 0x00000010

	INFINITE              = 0xFFFFFFFF
	MAX_ATR_SIZE          = 33
)

type SCARD_IO_REQUEST struct {
	Protocol  uint32
	PciLength uint32
}

var (
	g_rgSCardT0Pci = SCARD_IO_REQUEST{Protocol: SCARD_PROTOCOL_T0, PciLength: 8}
	g_rgSCardT1Pci = SCARD_IO_REQUEST{Protocol: SCARD_PROTOCOL_T1, PciLength: 8}
)

type ReaderState struct {
	Reader       *uint16
	UserData     uintptr
	CurrentState uint32
	EventState   uint32
	AtrLen       uint32
	Atr          [MAX_ATR_SIZE]byte
}

type Context struct {
	handle uintptr
}

type Card struct {
	handle         uintptr
	activeProtocol uint32
}

// コンテキストを確立
func EstablishContext() (*Context, error) {
	var handle uintptr
	ret, _, _ := procEstablishCtx.Call(
		uintptr(SCARD_SCOPE_USER),
		0,
		0,
		uintptr(unsafe.Pointer(&handle)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("SCardEstablishContext failed: 0x%X", ret)
	}

	return &Context{handle: handle}, nil
}

// コンテキストを解放
func (c *Context) Release() error {
	ret, _, _ := procReleaseCtx.Call(c.handle)
	if ret != 0 {
		return fmt.Errorf("SCardReleaseContext failed: 0x%X", ret)
	}
	return nil
}

// リーダーをリスト
func (c *Context) ListReaders() ([]string, error) {
	var readersLen uint32

	// まず必要なバッファサイズを取得
	ret, _, _ := procListReaders.Call(
		c.handle,
		0,
		0,
		uintptr(unsafe.Pointer(&readersLen)),
	)

	if ret != 0 && ret != 0x8010002E { // SCARD_E_NO_READERS_AVAILABLE
		return nil, fmt.Errorf("SCardListReaders (size) failed: 0x%X", ret)
	}

	if readersLen == 0 {
		return []string{}, nil
	}

	// バッファを確保してリーダー名を取得
	readers := make([]uint16, readersLen)
	ret, _, _ = procListReaders.Call(
		c.handle,
		0,
		uintptr(unsafe.Pointer(&readers[0])),
		uintptr(unsafe.Pointer(&readersLen)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("SCardListReaders failed: 0x%X", ret)
	}

	// マルチ文字列を分割
	readerList := []string{}
	start := 0
	for i := 0; i < int(readersLen); i++ {
		if readers[i] == 0 {
			if i > start {
				readerList = append(readerList, syscall.UTF16ToString(readers[start:i]))
			}
			start = i + 1
		}
	}

	return readerList, nil
}

// カードに接続
func (c *Context) Connect(reader string) (*Card, []byte, error) {
	readerName, err := syscall.UTF16PtrFromString(reader)
	if err != nil {
		return nil, nil, err
	}

	var handle uintptr
	var activeProtocol uint32

	ret, _, _ := procConnect.Call(
		c.handle,
		uintptr(unsafe.Pointer(readerName)),
		uintptr(SCARD_SHARE_SHARED),
		uintptr(SCARD_PROTOCOL_T0|SCARD_PROTOCOL_T1),
		uintptr(unsafe.Pointer(&handle)),
		uintptr(unsafe.Pointer(&activeProtocol)),
	)

	if ret != 0 {
		return nil, nil, fmt.Errorf("SCardConnect failed: 0x%X", ret)
	}

	card := &Card{
		handle:         handle,
		activeProtocol: activeProtocol,
	}

	// ATRを取得
	atr, err := card.GetATR()
	if err != nil {
		card.Disconnect()
		return nil, nil, err
	}

	return card, atr, nil
}

// カード状態を取得してATRを取得
func (c *Card) GetATR() ([]byte, error) {
	var readerLen uint32 = 256
	reader := make([]uint16, readerLen)
	var state, protocol uint32
	var atr [MAX_ATR_SIZE]byte
	var atrLen uint32 = MAX_ATR_SIZE

	ret, _, _ := procStatus.Call(
		c.handle,
		uintptr(unsafe.Pointer(&reader[0])),
		uintptr(unsafe.Pointer(&readerLen)),
		uintptr(unsafe.Pointer(&state)),
		uintptr(unsafe.Pointer(&protocol)),
		uintptr(unsafe.Pointer(&atr[0])),
		uintptr(unsafe.Pointer(&atrLen)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("SCardStatus failed: 0x%X", ret)
	}

	return atr[:atrLen], nil
}

// カードから切断
func (c *Card) Disconnect() error {
	ret, _, _ := procDisconnect.Call(c.handle, uintptr(SCARD_LEAVE_CARD))
	if ret != 0 {
		return fmt.Errorf("SCardDisconnect failed: 0x%X", ret)
	}
	return nil
}

// APDUコマンドを送信
func (c *Card) Transmit(apdu []byte) ([]byte, byte, byte, error) {
	sendBuf := apdu
	recvBuf := make([]byte, 258)
	recvLen := uint32(len(recvBuf))

	var pioSendPci *SCARD_IO_REQUEST
	if c.activeProtocol == SCARD_PROTOCOL_T0 {
		pioSendPci = &g_rgSCardT0Pci
	} else {
		pioSendPci = &g_rgSCardT1Pci
	}

	ret, _, _ := procTransmit.Call(
		c.handle,
		uintptr(unsafe.Pointer(pioSendPci)),
		uintptr(unsafe.Pointer(&sendBuf[0])),
		uintptr(len(sendBuf)),
		0,
		uintptr(unsafe.Pointer(&recvBuf[0])),
		uintptr(unsafe.Pointer(&recvLen)),
	)

	if ret != 0 {
		return nil, 0, 0, fmt.Errorf("SCardTransmit failed: 0x%X", ret)
	}

	if recvLen < 2 {
		return nil, 0, 0, fmt.Errorf("response too short: %d bytes", recvLen)
	}

	sw1 := recvBuf[recvLen-2]
	sw2 := recvBuf[recvLen-1]
	data := recvBuf[:recvLen-2]

	return data, sw1, sw2, nil
}

// カード状態変化を監視
func (c *Context) WaitForCardChange(readers []string, timeout uint32) ([]ReaderState, error) {
	states := make([]ReaderState, len(readers))

	for i, reader := range readers {
		readerPtr, err := syscall.UTF16PtrFromString(reader)
		if err != nil {
			return nil, err
		}
		states[i].Reader = readerPtr
		states[i].CurrentState = SCARD_STATE_UNAWARE
	}

	ret, _, _ := procGetStatusChange.Call(
		c.handle,
		uintptr(timeout),
		uintptr(unsafe.Pointer(&states[0])),
		uintptr(len(states)),
	)

	if ret != 0 && ret != 0x8010000A { // SCARD_E_TIMEOUT
		return nil, fmt.Errorf("SCardGetStatusChange failed: 0x%X", ret)
	}

	return states, nil
}
