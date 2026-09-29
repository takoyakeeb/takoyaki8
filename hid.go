package main

import usbkbd "machine/usb/hid/keyboard"

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
	// 修飾キー(Ctrlなど)は、値の定義を決めてから対応する
	return 0, false
}
