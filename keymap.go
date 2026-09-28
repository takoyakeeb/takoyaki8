package main

import usbkbd "machine/usb/hid/keyboard"

// Keymap[レイヤー][キー番号] = キーコード(0は「何もしない」)
// 固定サイズの配列なので、将来バイト列に変換して保存・転送しやすい。
type Keymap [NumLayers][NumKeys]uint16

var keymap = Keymap{
	{
		uint16(usbkbd.KeyA), uint16(usbkbd.KeyB), uint16(usbkbd.KeyC), uint16(usbkbd.KeyD),
		uint16(usbkbd.KeyE), uint16(usbkbd.KeyF), uint16(usbkbd.KeyG), uint16(usbkbd.KeyH),
	},
}

func (k *Keymap) Lookup(layer int, id uint16) uint16 {
	if layer < 0 || layer >= NumLayers || int(id) >= NumKeys {
		return 0
	}
	return k[layer][id]
}
