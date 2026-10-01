package main

// キーコード。tinykeemap の docs/protocol.md と同じ値を使う。
// 下位8bitは USB HID Usage ID(QMKの基本キーコードと同じ)。
// 名前もQMKに合わせてある(KC_A など)。必要になったら足していく。
const (
	KC_NO          uint16 = 0x0000 // 何もしない
	KC_TRANSPARENT uint16 = 0x0001 // 透過(下のレイヤーの設定を使う)
	KC_A           uint16 = 0x0004
	KC_B           uint16 = 0x0005
	KC_C           uint16 = 0x0006
	KC_D           uint16 = 0x0007
	KC_E           uint16 = 0x0008
	KC_F           uint16 = 0x0009
	KC_G           uint16 = 0x000A
	KC_H           uint16 = 0x000B

	// 修飾キー単体(USB HID Usage ID の 0xE0〜0xE7。並びは hid.go の modifierKeys と同じにすること)
	KC_LCTL uint16 = 0x00E0 // 左 Ctrl
	KC_LSFT uint16 = 0x00E1 // 左 Shift
	KC_LALT uint16 = 0x00E2 // 左 Alt
	KC_LGUI uint16 = 0x00E3 // 左 GUI(Windowsキー / Superキー)
	KC_RCTL uint16 = 0x00E4 // 右 Ctrl
	KC_RSFT uint16 = 0x00E5 // 右 Shift
	KC_RALT uint16 = 0x00E6 // 右 Alt
	KC_RGUI uint16 = 0x00E7 // 右 GUI
)
