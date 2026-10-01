package main

import usbkbd "machine/usb/hid/keyboard"

// modifierKeys は、修飾キー単体(KC_LCTL〜KC_RGUI = 0x00E0〜0x00E7)に対応する、
// TinyGoの Keycode の表。並びは keycode.go の KC_LCTL〜KC_RGUI と同じ。
var modifierKeys = [...]usbkbd.Keycode{
	usbkbd.KeyLeftCtrl,
	usbkbd.KeyLeftShift,
	usbkbd.KeyLeftAlt,
	usbkbd.KeyLeftGUI,
	usbkbd.KeyRightCtrl,
	usbkbd.KeyRightShift,
	usbkbd.KeyRightAlt,
	usbkbd.KeyRightGUI,
}

// toHID は、キーマップのキーコード(keycode.go の値)を、
// TinyGoのキーボード機能が使う Keycode に変換する。
//
// TinyGoの Keycode は USB HID の値をそのままではなく、独自の形で持っている。
// そのため「キーマップにはHIDの値を保存し、キーボードへ送る直前に変換する」形にした。
// (キーマップがTinyGoの都合に縛られず、tinykeemapともそのまま値をやり取りできる)
//
// 変換できないもの(KC_NO・透過・まだ対応していないコード)は ok=false を返す。
func toHID(code uint16) (kc usbkbd.Keycode, ok bool) {
	// A(0x04)から基本キーの最後(0xA4)までは、KeyA からの差で求められる
	if code >= KC_A && code <= 0x00A4 {
		return usbkbd.KeyA + usbkbd.Keycode(code-KC_A), true
	}
	// 修飾キー単体(0xE0〜0xE7)は、表から引く
	if code >= KC_LCTL && code <= KC_RGUI {
		return modifierKeys[code-KC_LCTL], true
	}
	return 0, false
}

// keyAction は、1つのキーを押したときに送るもの。
type keyAction struct {
	// 押す修飾キー。ビット0〜7が、左Ctrl・左Shift・左Alt・左GUI・右Ctrl・右Shift・右Alt・右GUI。
	// (USB HID の修飾キーの並びと同じ。1つも押さないときは0)
	mods uint8
	// 押す基本キー(TinyGoの Keycode)
	key usbkbd.Keycode
	// 基本キーがあるか。修飾キー単体のときは false
	hasKey bool
}

// decodeKey は、キーマップのキーコードを、押す修飾キーと基本キーに分ける。
//   - 基本キー(0x0004〜0x00A4)             → 基本キーだけ
//   - 修飾キー単体(0x00E0〜0x00E7)         → 修飾キーだけ
//   - 修飾キー付き(0x0100〜0x1FFF)         → 修飾キー + 基本キー(例: Shift+A = 0x0204)
//
// 修飾キー付きの値は、下位8bitが基本キー(0x0004〜0x00A4)、ビット8〜11が Ctrl / Shift / Alt / GUI、
// ビット12(0x1000)が「右側」(立っていると、修飾キーはすべて右側になる)。
// 修飾キーが1つもない値(0x1004 など)や、基本キーでない値は、未定義として ok=false を返す。
// 「何もしない」「透過」「レイヤー切り替え」は、ここでは扱わない(ok=false)。
func decodeKey(code uint16) (a keyAction, ok bool) {
	// 修飾キー単体(0xE0〜0xE7)
	if code >= KC_LCTL && code <= KC_RGUI {
		return keyAction{mods: uint8(1) << (code - KC_LCTL)}, true
	}
	// 修飾キー付き(0x0100〜0x1FFF)
	if code >= 0x0100 && code <= 0x1FFF {
		base := code & 0x00FF
		if base < KC_A || base > 0x00A4 {
			return keyAction{}, false
		}
		mods := uint8(code>>8) & 0x0F // Ctrl / Shift / Alt / GUI(左側の並び)
		if mods == 0 {
			return keyAction{}, false
		}
		if code&0x1000 != 0 {
			mods <<= 4 // 右側の修飾キーのビットに移す
		}
		kc, found := toHID(base)
		if !found {
			return keyAction{}, false
		}
		return keyAction{mods: mods, key: kc, hasKey: true}, true
	}
	// 基本キー(0x0004〜0x00A4)
	if code >= KC_A && code <= 0x00A4 {
		kc, found := toHID(code)
		if !found {
			return keyAction{}, false
		}
		return keyAction{key: kc, hasKey: true}, true
	}
	return keyAction{}, false
}
