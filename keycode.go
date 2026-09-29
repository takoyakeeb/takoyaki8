package main

// キーコード。tinykeemap の docs/protocol.md と同じ値を使う。
// 下位8bitは USB HID Usage ID(QMKの基本キーコードと同じ)。
// 名前もQMKに合わせてある(KC_A など)。必要になったら足していく。
const (
	KC_NO          uint16 = 0x0000 // 何もしない
	KC_TRANSPARENT uint16 = 0x0001 // 透過(下のレイヤーの設定を使う)

	KC_A uint16 = 0x0004
	KC_B uint16 = 0x0005
	KC_C uint16 = 0x0006
	KC_D uint16 = 0x0007
	KC_E uint16 = 0x0008
	KC_F uint16 = 0x0009
	KC_G uint16 = 0x000A
	KC_H uint16 = 0x000B
)
