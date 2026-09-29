package main

// Keymap[レイヤー][キー番号] = キーコード(値の意味は keycode.go)
// 固定サイズの配列なので、将来バイト列に変換して保存・転送しやすい。
type Keymap [NumLayers][NumKeys]uint16

// 初期のキーマップ。レイヤー0だけ設定し、他のレイヤーは init で「透過」にする。
var keymap = Keymap{
	{
		KC_A, KC_B, KC_C, KC_D,
		KC_E, KC_F, KC_G, KC_H,
	},
}

// レイヤー1以降は「透過」で埋める(何も設定していない = 下のレイヤーと同じ)。
func init() {
	for l := 1; l < NumLayers; l++ {
		for k := range keymap[l] {
			keymap[l][k] = KC_TRANSPARENT
		}
	}
}

// Lookup は、指定したレイヤーでキー番号idに割り当てられたキーコードを返す。
// 「透過」なら1つ下のレイヤーを見に行く(レイヤー0まで行っても透過なら KC_NO)。
func (k *Keymap) Lookup(layer int, id uint16) uint16 {
	if layer < 0 || layer >= NumLayers || int(id) >= NumKeys {
		return KC_NO
	}
	for l := layer; l >= 0; l-- {
		if code := k[l][id]; code != KC_TRANSPARENT {
			return code
		}
	}
	return KC_NO
}
