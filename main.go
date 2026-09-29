package main

import (
	usbkbd "machine/usb/hid/keyboard"
	"time"
)

// true にすると、キーイベントをシリアルに出力する(動作確認用)
const debug = false

func main() {
	var scanner Scanner = NewMatrix(colPins[:], rowPins[:], debounceCount)
	kb := usbkbd.Port()

	// 今は常にレイヤー0を使う(レイヤーの切り替えは後で作る)
	activeLayer := 0

	// 押した時に送ったキーを覚えておき、離す時に同じものを離す(0は「送っていない」)
	var held [NumKeys]usbkbd.Keycode
	events := make([]KeyEvent, 0, NumKeys)

	for {
		events = scanner.Scan(events[:0])

		for _, ev := range events {
			if debug {
				println("key", ev.ID, ev.Pressed)
			}

			if ev.Pressed {
				code := keymap.Lookup(activeLayer, ev.ID)
				if kc, ok := toHID(code); ok {
					held[ev.ID] = kc
					kb.Down(kc)
				}
			} else {
				if kc := held[ev.ID]; kc != 0 {
					held[ev.ID] = 0
					kb.Up(kc)
				}
			}
		}

		time.Sleep(time.Millisecond)
	}
}
