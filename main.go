package main

import (
	usbkbd "machine/usb/hid/keyboard"
	"time"
)

// true にすると、キーイベントをシリアルに出力する(動作確認用)
const debug = false

func main() {
	var scanner Scanner = NewMatrix(colPins, rowPins, debounceCount)
	kb := usbkbd.Port()

	// 押した時のキーコードを覚えておき、離す時に同じものを離す
	var held [NumKeys]uint16
	events := make([]KeyEvent, 0, NumKeys)

	for {
		events = scanner.Scan(events[:0])

		for _, ev := range events {
			if debug {
				println("key", ev.ID, ev.Pressed)
			}

			if ev.Pressed {
				code := keymap.Lookup(0, ev.ID)
				held[ev.ID] = code
				if code != 0 {
					kb.Down(usbkbd.Keycode(code))
				}
			} else {
				code := held[ev.ID]
				held[ev.ID] = 0
				if code != 0 {
					kb.Up(usbkbd.Keycode(code))
				}
			}
		}

		time.Sleep(time.Millisecond)
	}
}
