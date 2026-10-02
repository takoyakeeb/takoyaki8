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

	// LED(WS2812B)を使える状態にする(led.go)
	initLED()

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

	// 押した時に押した修飾キー(Shift など)を覚えておき、離す時に同じものを離す(0は「押していない」)。
	// ビット0〜7が、左Ctrl・左Shift・左Alt・左GUI・右Ctrl・右Shift・右Alt・右GUI(mods.go)。
	var heldMods [NumKeys]uint8

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
				} else if a, ok := decodeKey(code); ok {
					// 基本キー・修飾キー単体・修飾キー付き。修飾キーを先に押してから、基本キーを押す
					heldMods[ev.ID] = a.mods
					pressMods(kb, a.mods)
					if a.hasKey {
						held[ev.ID] = a.key
						kb.Down(a.key)
					}
				}
			} else {
				if m := heldMO[ev.ID]; m != 0 {
					heldMO[ev.ID] = 0
					momentary[m-1]--
				}
				// 基本キーを先に離してから、修飾キーを離す
				if kc := held[ev.ID]; kc != 0 {
					held[ev.ID] = 0
					kb.Up(kc)
				}
				if mods := heldMods[ev.ID]; mods != 0 {
					heldMods[ev.ID] = 0
					releaseMods(kb, mods)
				}
			}
		}

		// USBシリアルに届いたコマンド(INFO / GET / SET / DUMP / SAVE / LOAD / RESET)を処理する
		pollSerial()

		// LED の表示を、一定回数ごとに1回だけ更新する(led.go)。
		// 更新中は割り込みが止まる場合があるので、キー処理とシリアル処理のあとに置く。
		tickLED()

		time.Sleep(time.Millisecond)
	}
}
