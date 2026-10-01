package main

import usbkbd "machine/usb/hid/keyboard"

// keySender は、キーを押す・離すことができるもの(TinyGoの usbkbd.Port() で得られるもの)。
type keySender interface {
	Down(c usbkbd.Keycode) error
	Up(c usbkbd.Keycode) error
}

// modCount[i] は、修飾キー i(modifierKeys の並び。0=左Ctrl … 7=右GUI)を、いくつのキーが押しているか。
// 同じ修飾キーを複数のキーが押しているとき、1つ離しただけでは離さないようにするための数。
// (例: 左Shiftを押したまま Shift+A のキーを押して、Shift+A のキーだけ離しても、左Shiftは押したまま)
var modCount [8]uint8

// pressMods は、mods(ビット i が修飾キー i)の修飾キーを押す。
// 数が 0 から 1 になった修飾キーだけ、実際に押す。
func pressMods(kb keySender, mods uint8) {
	for i := 0; i < len(modCount); i++ {
		if mods&(uint8(1)<<uint(i)) == 0 {
			continue
		}
		modCount[i]++
		if modCount[i] == 1 {
			kb.Down(modifierKeys[i])
		}
	}
}

// releaseMods は、pressMods で押した修飾キーを離す。
// 数が 1 から 0 になった修飾キーだけ、実際に離す。
func releaseMods(kb keySender, mods uint8) {
	for i := 0; i < len(modCount); i++ {
		if mods&(uint8(1)<<uint(i)) == 0 {
			continue
		}
		if modCount[i] > 0 {
			modCount[i]--
			if modCount[i] == 0 {
				kb.Up(modifierKeys[i])
			}
		}
	}
}
