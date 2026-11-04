package main

// 「保存してあるか」の判定(INFO の saved)と、SAVE / LOAD の中身。
// キーマップ(flash.go)と設定(cfg.go)は、別のセクタに保存する。
//
// 「起動時に使われるもの」の控えを、RAM に持つ。
//   - 起動したとき: Flash が有効なら Flash の内容、無効なら初期値。
//   - SAVE したあと・LOAD したあと: その時点の内容。
// いまの RAM が、この控えと同じなら saved=1(電源を入れ直しても、同じ状態で起動する)。
// TG / TO のレイヤーの状態は、比べない(保存しないものなので)。

var (
	keymapBoot Keymap     // 起動時に使われるキーマップの控え
	cfgBoot    []cfgEntry // 起動時に使われる設定の控え
)

// markBoot は、いまの RAM の内容を、「起動時に使われるもの」の控えにする。
// 起動の直後(main)と、LOAD のあとに呼ぶ。
func markBoot() {
	keymapBoot = keymap
	cfgBoot = append(cfgBoot[:0], cfgNow...)
}

// isSaved は、RAM のキーマップと設定が、どちらも控えと同じなら true。
func isSaved() bool {
	return keymap == keymapBoot && cfgEqual(cfgNow, cfgBoot)
}

// savedFlag は、INFO の saved の値("1" か "0")を返す。
func savedFlag() string {
	if isSaved() {
		return "1"
	}
	return "0"
}

// doSave は、SAVE の中身。変わったほうだけ、Flash に書く(書き換え回数を増やさないため)。
// どちらも変わっていなければ、何も書かずに OK を返す。
func doSave() string {
	if keymap != keymapBoot {
		if err := saveKeymapToFlash(&keymap); err != nil {
			return "ERR FLASH " + err.Error()
		}
		keymapBoot = keymap
	}
	if !cfgEqual(cfgNow, cfgBoot) {
		if err := cfgSaveToFlash(cfgNow); err != nil {
			return "ERR FLASH cfg " + err.Error()
		}
		cfgBoot = append(cfgBoot[:0], cfgNow...)
	}
	return "OK"
}

// doLoad は、LOAD の中身。キーマップと設定を、「起動したときと同じ状態」に戻す。
// Flash が有効ならその内容、無効(未保存・壊れている)なら初期値にする。未保存の変更は捨てる。
// Flash が空でも、エラーにはしない(起動時と同じ扱い)。
func doLoad() string {
	if km, err := loadKeymapFromFlash(); err == nil {
		keymap = km
	} else {
		keymap = defaultKeymap()
	}
	if list, err := cfgLoadFromFlash(); err == nil {
		cfgNow = cfgBuild(list)
	} else {
		cfgNow = cfgBuild(nil)
	}
	markBoot()
	resetLayerState() // キーマップが入れ替わったので、TG / TO の状態は解除する(protocol.md §3)
	return "OK"
}
