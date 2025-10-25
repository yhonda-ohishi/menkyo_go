

from smartcard.util import toHexString
from smartcard.CardMonitoring import CardMonitor, CardObserver
from smartcard.CardConnectionObserver import ConsoleCardConnectionObserver
import cv2

from datetime import datetime, timedelta

from make_lib import database as db
from make_lib import reg_ic as reg_ic

GET_RESPONSE = [0XA0, 0XC0, 00, 00]  # IC用コード定義
check_menkyo = [0x00, 0xA4, 0x00, 0x00]
start = [0xff, 0xc2, 0x00, 0x00, 0x01, 0x81]
Start_trans = [0xff, 0xc2, 0x00, 0x00, 0x02, 0x84, 0x00]
Check_shaken = [0xFF, 0xCA, 0x01, 0x00, 0x00]
Switch_felica = [0xFF, 0xC2, 0x00, 0x02, 0x04, 0x8F, 0x02, 0x03, 0x04]
# SELECT_felica = [0xFF, 0x00, 0x50, 0x00, 0x02, 0x00, 0x03]
SELECT_felica = [0xFF, 0x00, 0x50, 0x00, 0x02, 0xff, 0xff]  # felica-lightに対応
GET_felica_uid = [0xFF, 0xCA, 0x00, 0x00, 0x00]
GET_shaken_uid = [0x00, 0xA4, 0x00, 0x00]
GET_shaken_sys = [0x0C, 0x0D, 0x00, 0x00]
GET_shaken_read = [0x06, 0x07, 0x00, 0x00]
GET_shaken_mf_select = [0x00, 0xa4, 0x00, 0x00, 0x02, 0x3f, 0x00]
GET_shaken_Data = [0x00, 0xB0, 0x00, 0x02, 0x01]
GET_shaken_nokori = [0x00, 0x20, 0x00, 0x80]
SELECT_end = [0xff, 0xc2, 0x00, 0x00, 0x02, 0x82, 0x00]


def init_printob():
    try:
        # 監視クラスの初期化
        cardmonitor = CardMonitor()
        cardobserver = PrintObserver()
        cardmonitor.addObserver(cardobserver)
    except KeyboardInterrupt:
        cardmonitor.deleteObserver(cardobserver)
        # cardmonitor.addObserver(cardobserver)


class PrintObserver(CardObserver):
    # スマートカードを検知すると呼び出される関数
    uid = ""
    uidd = ""
    pic_name = ""
    last_uuid=""
    # cam = cv2.VideoCapture(0)

    def reset(self):
        self.uid = ""
        self.uidd = ""
        self.pic_name = ""

    def __init__(self):
        self.observer = ConsoleCardConnectionObserver()
        self.uid = ""
        db.log_message("print observer init")
        # ret, img = self.cam.read()

        # self.cam_error = False
        # if (self.cam.isOpened() is False):
        #     self.cam_error = True

    def update(self, observable, actions):
        global register_ic
        self.card_type = ""
        self.card_due = ""
        self.card_due_count = ""
        try:
            (addedcards, removedcards) = actions
            for card in addedcards:
                card.connection = card.createConnection()
                card.connection.connect()
                card.connection.addObserver(self.observer)
                card.connection.transmit(start)
                print('start trans')
                
                db.log_message("start trans")
                # start_trans
                card.connection.transmit(Start_trans)
                print('check shaken')
                response, sw1, sw2 = card.connection.transmit(
                    Check_shaken)  # 車検証確認用
                print(f"card atr:{toHexString(card.atr)}")
                if (response == [6, 120, 119, 129, 2, 128]):
                    self.card_type = 'car_inspection'
                # elif (toHexString(card.atr) == "3B 88 80 01 00 00 00 00 91 81 C1 00 D8"):
                elif (toHexString(card.atr).startswith("3B 88 80 01 00 00 00")):
                    response, sw1, sw2 = card.connection.transmit(
                        GET_felica_uid)
                    response, sw1, sw2 = card.connection.transmit(  # select MF 残り回数
                        [0x00, 0xA4, 0x00, 0x00],)
                    response, sw1, sw2 = card.connection.transmit(  # 残り回数照会
                        [0x00, 0x20, 0x00, 0x81])
                    print('残り回数', oct(sw2)[4:])
                    self.card_due_count = oct(sw2)[4:]
                    response, sw1, sw2 = card.connection.transmit(  # 有効期限照会　MF
                        [0x00,
                         0xA4,
                         0x02,
                         0x0C,
                         0x02,
                         0x2F,
                         0x01])
                    response, sw1, sw2 = card.connection.transmit(  # 有効期限照会　DF
                        [0x00,
                         0xb0,
                         0x00,
                         0x00,
                         0x11,])  # 残り回数照会
                    if (sw1 == 0x90 and sw2 == 0x00):
                        self.card_due = toHexString(response).replace(' ', '')
                        self.card_type = 'driver_license'
                if (self.card_type == ""):
                    self.card_type = "other"
                db.log_message("to check response 1 remain")
                try:
                    db.log_message(toHexString(response1).replace(' ', ''))
                except:
                    pass
                response1=None
                # db.log_message(toHexString(response1).replace(' ', ''))
                response, sw1, sw2 = card.connection.transmit(SELECT_felica)
                response1, sw1, sw2 = card.connection.transmit(GET_felica_uid)
                if (self.card_type == "driver_license"):
                    db.log_message("driver license")
                    self.uidd = toHexString(card.atr).replace(
                        ' ', '') + self.card_due
                    print(f"driver license:{self.uidd}")
                else:
                    db.log_message("not driver license")
                    self.uidd = toHexString(response1).replace(' ', '')
                    # self.uid = response1

                td = timedelta(hours=9)
                ttd = datetime.now()+td
                print('reg ic start')
                print(self.card_due[10:26])
                if(self.last_uuid==self.uidd):#2回連続で削除
                    self.uidd="" 
                self.last_uuid=self.uidd
                db.log_message(self.uidd)
                reg_ic.ic.setup(ic=self.uidd, ic_type=self.card_type,
                                ic_detail=f"{self.card_due},{self.card_due_count}")
                db.log_message("enrolled ic in printOB")
                print('reg ic end')
            # cap_multi(self.uidd)
                self.uid = ""
                self.uuid = ""
                try:
                    response1, sw1, sw2 = card.connection.transmit(SELECT_end)#select end で　他の人のＩＤで登録される不具合を修正試み
                except:
                    pass
                db.log_message("select end")
            
                card.connection.disconnect()

            for card in removedcards:
                db.log_message("remove card")
                self.uid = ""
                self.uuid = ""
                
                card.connection.disconnect()
        except Exception as e:
            db.log_message("err enrolled ic in printOB")
            
            db.log_message(f"Unexpected {e=}, {type(e)=}")
            try:
                # print(e)
                db.log_message(f"{e=}")
                db.log_message(f"{type(e)=}")
            except:
                pass
            # print(e)
            
            self.uid = ""
            self.uuid = ""
            db.log_message("trying restart..")
            reg_ic.ic.sound("再起動を開始します")
            card.connection.disconnect()
            # self.__init__
            # init_printob()
            db.log_message("restarted")
            db.log_message("buser sound")
            reg_ic.ic.sound_busser("nc302124")
            reg_ic.ic.sound_busser("再起動が完了しました")
            reg_ic.ic.sound_busser("再度ＩＣをかざしてください")
            
