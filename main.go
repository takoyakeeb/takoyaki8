package main

import (
	usbkbd "machine/usb/hid/keyboard"
	"time"
)

// true にすると、キーイベントをシリアルに出力する(動作確認用)。
// 出力は必ず "# " で始まる行にして、通信の応答(OK ... / ERR ...)と見分けられるようにする。
// ただし応答の間にデバッグ行が混ざるので、tinykeemap を使うときは false にしておく。
const debug = false

func main() {
	var scanner Scanner = NewMatrix(colPins[:], rowPins[:], debounceCount)

	kb := usbkbd.Port()

	// 起動時: Flashに有効なキーマップがあればそれを使う。無効(未保存・壊れている)なら初期キーマップのまま。
	if km, err := loadKeymapFromFlash(); err == nil {
		keymap = km
	}

	// 押した時に送ったキーを覚えておき、離す時に同じものを離す(0は「送っていない」)
	// SET でキーマップを書き換えても、押している最中のキーは元のキーを離すので、押しっぱなしにならない
	var held [NumKeys]usbkbd.Keycode

	// MO(n) のキーを押している間は「n+1」を覚えておく(0は「MOを押していない」)。
	// 離す時に、押した時と同じレイヤーを戻す。途中でキーマップやレイヤーが変わっても、戻し忘れない。
	var heldMO [NumKeys]uint8

	events := make([]KeyEvent, 0, NumKeys)

	for {
		events = scanner.Scan(events[:0])

		for _, ev := range events {
			if debug {
				println("# key", ev.ID, ev.Pressed)
			}

			if ev.Pressed {
				// いま有効なレイヤーで、押したキーの keycode を決める
				code := keymap.Lookup(activeLayers(), ev.ID)
				if kind, layer, ok := decodeLayerKey(code); ok {
					switch kind {
					case layerMO:
						// MO(n): 押している間だけ、レイヤー n を有効にする(離すときに戻す)
						heldMO[ev.ID] = uint8(layer) + 1
						momentary[layer]++
					case layerTG:
						// TG(n): 押すたびに、レイヤー n の有効・無効を切り替える(離すときは何もしない)
						toggleLayer(layer)
					case layerTO:
						// TO(n): 切り替えで有効にしているレイヤーを、n だけにする(離すときは何もしない)
						moveToLayer(layer)
					}
				} else if kc, ok := toHID(code); ok {
					held[ev.ID] = kc
					kb.Down(kc)
				}
			} else {
				if m := heldMO[ev.ID]; m != 0 {
					heldMO[ev.ID] = 0
					momentary[m-1]--
				}
				if kc := held[ev.ID]; kc != 0 {
					held[ev.ID] = 0
					kb.Up(kc)
				}
			}
		}

		// USBシリアルに届いたコマンド(INFO / GET / SET / DUMP / SAVE / LOAD / RESET)を処理する
		pollSerial()

		time.Sleep(time.Millisecond)
	}
}
