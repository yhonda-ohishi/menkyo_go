package nfc

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// APDUコマンド定義（printobserver.pyから移植）
var (
	CMD_START            = []byte{0xff, 0xc2, 0x00, 0x00, 0x01, 0x81}
	CMD_START_TRANS      = []byte{0xff, 0xc2, 0x00, 0x00, 0x02, 0x84, 0x00}
	CMD_CHECK_SHAKEN     = []byte{0xFF, 0xCA, 0x01, 0x00, 0x00}
	CMD_SWITCH_FELICA    = []byte{0xFF, 0xC2, 0x00, 0x02, 0x04, 0x8F, 0x02, 0x03, 0x04}
	CMD_SELECT_FELICA    = []byte{0xFF, 0x00, 0x50, 0x00, 0x02, 0xff, 0xff} // felica-light対応
	CMD_GET_FELICA_UID   = []byte{0xFF, 0xCA, 0x00, 0x00, 0x00}
	CMD_SELECT_MF        = []byte{0x00, 0xA4, 0x00, 0x00}
	CMD_CHECK_REMAIN     = []byte{0x00, 0x20, 0x00, 0x81}
	CMD_SELECT_EXPIRE_MF = []byte{0x00, 0xA4, 0x02, 0x0C, 0x02, 0x2F, 0x01}
	CMD_READ_EXPIRE_DF   = []byte{0x00, 0xb0, 0x00, 0x00, 0x11}
	CMD_SELECT_END       = []byte{0xff, 0xc2, 0x00, 0x00, 0x02, 0x82, 0x00}
)

// 免許証ATRのプレフィックス
const DRIVER_LICENSE_ATR_PREFIX = "3B888001000000"

// カード種別
const (
	CardTypeDriverLicense  = "driver_license"
	CardTypeCarInspection = "car_inspection"
	CardTypeOther         = "other"
)

// LicenseData 免許証データ
type LicenseData struct {
	CardID          string
	CardType        string
	ATR             string
	ExpiryDate      string
	RemainCount     string
	FeliCaUID       string
	ReadTimestamp   time.Time
	ReaderName      string
}

// LicenseReader 免許証リーダー
type LicenseReader struct {
	context *Context
	logger  func(string)
}

// NewLicenseReader 新しいLicenseReaderを作成
func NewLicenseReader(logger func(string)) (*LicenseReader, error) {
	ctx, err := EstablishContext()
	if err != nil {
		return nil, fmt.Errorf("failed to establish context: %w", err)
	}

	return &LicenseReader{
		context: ctx,
		logger:  logger,
	}, nil
}

// Close リーダーを閉じる
func (lr *LicenseReader) Close() error {
	if lr.context != nil {
		return lr.context.Release()
	}
	return nil
}

// log ログを出力
func (lr *LicenseReader) log(msg string) {
	if lr.logger != nil {
		lr.logger(msg)
	}
}

// ListReaders 利用可能なリーダーをリスト
func (lr *LicenseReader) ListReaders() ([]string, error) {
	return lr.context.ListReaders()
}

// ReadCard カードを読み取る
func (lr *LicenseReader) ReadCard(readerName string) (*LicenseData, error) {
	lr.log(fmt.Sprintf("Connecting to reader: %s", readerName))

	card, atr, err := lr.context.Connect(readerName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to card: %w", err)
	}
	defer card.Disconnect()

	lr.log(fmt.Sprintf("Card ATR: %s", hex.EncodeToString(atr)))

	data := &LicenseData{
		ATR:           hex.EncodeToString(atr),
		ReadTimestamp: time.Now(),
		ReaderName:    readerName,
	}

	// 初期化コマンド送信
	lr.log("Sending START command")
	_, _, _, err = card.Transmit(CMD_START)
	if err != nil {
		return nil, fmt.Errorf("START command failed: %w", err)
	}

	lr.log("Sending START_TRANS command")
	_, _, _, err = card.Transmit(CMD_START_TRANS)
	if err != nil {
		return nil, fmt.Errorf("START_TRANS command failed: %w", err)
	}

	// カード種別を判定
	data.CardType = lr.detectCardType(card, atr)
	lr.log(fmt.Sprintf("Card type detected: %s", data.CardType))

	// 免許証の場合、追加情報を取得
	if data.CardType == CardTypeDriverLicense {
		lr.log("Reading driver license data")
		err = lr.readDriverLicenseData(card, data)
		if err != nil {
			lr.log(fmt.Sprintf("Warning: failed to read license data: %v", err))
		}
	}

	// FeliCa UIDを取得
	lr.log("Reading FeliCa UID")
	_, _, _, _ = card.Transmit(CMD_SELECT_FELICA)
	uidResp, sw1, sw2, err := card.Transmit(CMD_GET_FELICA_UID)
	if err == nil && sw1 == 0x90 && sw2 == 0x00 {
		data.FeliCaUID = hex.EncodeToString(uidResp)
		lr.log(fmt.Sprintf("FeliCa UID: %s", data.FeliCaUID))
	}

	// CardIDを生成
	if data.CardType == CardTypeDriverLicense {
		data.CardID = strings.ToUpper(data.ATR + data.ExpiryDate)
	} else {
		data.CardID = strings.ToUpper(data.FeliCaUID)
	}

	// 終了コマンド送信
	lr.log("Sending SELECT_END command")
	card.Transmit(CMD_SELECT_END)

	return data, nil
}

// detectCardType カード種別を判定
func (lr *LicenseReader) detectCardType(card *Card, atr []byte) string {
	// 車検証チェック
	resp, _, _, err := card.Transmit(CMD_CHECK_SHAKEN)
	if err == nil && len(resp) == 6 {
		if resp[0] == 0x06 && resp[1] == 0x78 && resp[2] == 0x77 &&
			resp[3] == 0x81 && resp[4] == 0x02 && resp[5] == 0x80 {
			return CardTypeCarInspection
		}
	}

	// 免許証チェック（ATRプレフィックスで判定）
	atrHex := strings.ToUpper(hex.EncodeToString(atr))
	if strings.HasPrefix(atrHex, DRIVER_LICENSE_ATR_PREFIX) {
		return CardTypeDriverLicense
	}

	return CardTypeOther
}

// readDriverLicenseData 免許証データを読み取る
func (lr *LicenseReader) readDriverLicenseData(card *Card, data *LicenseData) error {
	// MF選択
	_, _, _, err := card.Transmit(CMD_SELECT_MF)
	if err != nil {
		return fmt.Errorf("SELECT MF failed: %w", err)
	}

	// 残り回数照会
	_, sw1, sw2, err := card.Transmit(CMD_CHECK_REMAIN)
	if err == nil {
		// sw2の下位4ビットが残り回数
		remainCount := sw2 & 0x0F
		data.RemainCount = fmt.Sprintf("%d", remainCount)
		lr.log(fmt.Sprintf("Remain count: %s", data.RemainCount))
	}

	// 有効期限照会 - MF選択
	_, sw1, sw2, err = card.Transmit(CMD_SELECT_EXPIRE_MF)
	if err != nil {
		return fmt.Errorf("SELECT EXPIRE MF failed: %w", err)
	}

	// 有効期限照会 - DF読み取り
	expireResp, sw1, sw2, err := card.Transmit(CMD_READ_EXPIRE_DF)
	if err != nil {
		return fmt.Errorf("READ EXPIRE DF failed: %w", err)
	}

	if sw1 == 0x90 && sw2 == 0x00 {
		data.ExpiryDate = hex.EncodeToString(expireResp)
		lr.log(fmt.Sprintf("Expiry date (hex): %s", data.ExpiryDate))
	}

	return nil
}

// MonitorCards カード挿入を監視
func (lr *LicenseReader) MonitorCards(callback func(*LicenseData, error)) error {
	readers, err := lr.ListReaders()
	if err != nil {
		return fmt.Errorf("failed to list readers: %w", err)
	}

	if len(readers) == 0 {
		return fmt.Errorf("no readers found")
	}

	lr.log(fmt.Sprintf("Monitoring %d reader(s): %v", len(readers), readers))

	lastCardID := ""
	cardProcessed := make(map[string]bool) // 処理済みカードを記録

	for {
		// カード状態変化を待機（1秒タイムアウト）
		states, err := lr.context.WaitForCardChange(readers, 1000)
		if err != nil {
			lr.log(fmt.Sprintf("WaitForCardChange error: %v", err))
			time.Sleep(1 * time.Second)
			continue
		}

		for i, state := range states {
			// カードが存在する場合
			if state.EventState&SCARD_STATE_PRESENT != 0 {
				readerName := readers[i]

				// ATRを取得してカードIDを作成（簡易的な識別用）
				atrHex := hex.EncodeToString(state.Atr[:state.AtrLen])

				// 同じカードが既に処理済みの場合はスキップ
				if cardProcessed[atrHex] {
					states[i].CurrentState = state.EventState
					continue
				}

				lr.log(fmt.Sprintf("Card detected on reader: %s", readerName))

				// 最大3回リトライ
				var data *LicenseData
				var readErr error
				maxRetries := 3
				successRead := false

				for retry := 0; retry < maxRetries; retry++ {
					if retry > 0 {
						lr.log(fmt.Sprintf("Retry %d/%d", retry, maxRetries-1))
						time.Sleep(500 * time.Millisecond)

						// リトライ前にカードがまだ存在するか確認
						checkStates, checkErr := lr.context.WaitForCardChange(readers, 100)
						if checkErr == nil && len(checkStates) > i {
							if checkStates[i].EventState&SCARD_STATE_EMPTY != 0 {
								lr.log("Card removed during retry, aborting")
								readErr = fmt.Errorf("card removed during retry")
								break
							}
							// 状態を更新
							states[i].CurrentState = checkStates[i].EventState
						}
					}

					data, readErr = lr.ReadCard(readerName)

					// 成功判定：エラーがなく、免許証の場合はExpiryDateがある
					if readErr == nil {
						if data.CardType == CardTypeDriverLicense {
							// 免許証の場合、ExpiryDateが取得できたら成功
							if data.ExpiryDate != "" {
								lr.log(fmt.Sprintf("Successfully read license with expiry date (attempt %d)", retry+1))
								successRead = true
								break
							}
						} else {
							// 免許証以外の場合、エラーがなければ成功
							lr.log(fmt.Sprintf("Successfully read card (attempt %d)", retry+1))
							successRead = true
							break
						}
					}

					if retry < maxRetries-1 {
						lr.log(fmt.Sprintf("Read failed or incomplete, retrying... Error: %v", readErr))
					} else {
						lr.log(fmt.Sprintf("All %d attempts failed", maxRetries))
					}
				}

				// 成功したか、3回リトライしたら処理済みとしてマーク
				cardProcessed[atrHex] = true

				if data != nil {
					lastCardID = data.CardID
				}

				// 最終結果をコールバック
				if !successRead && readErr == nil {
					readErr = fmt.Errorf("failed to read complete data after %d attempts", maxRetries)
				}
				callback(data, readErr)

				// 状態をリセット
				states[i].CurrentState = state.EventState
			} else if state.EventState&SCARD_STATE_EMPTY != 0 {
				// カードが取り除かれた
				if lastCardID != "" {
					lr.log("Card removed")
					lastCardID = ""
					// 処理済みフラグをクリア
					cardProcessed = make(map[string]bool)
				}
				states[i].CurrentState = state.EventState
			}
		}
	}
}
