package main

// Keymap[レイヤー][キー番号] = キーコード(値の意味は keycode.go)
// 固定サイズの配列なので、将来バイト列に変換して保存・転送しやすい。
type Keymap [NumLayers][NumKeys]uint16

// defaultKeymap は、初期のキーマップを作って返す。
// レイヤー0だけ設定し、他のレイヤーは「透過」(何も設定していない = 下のレイヤーと同じ)にする。
// RESET コマンドや、Flashに有効なデータがないときに使う。
func defaultKeymap() Keymap {
	km := Keymap{
		{
			KC_A, KC_B, KC_C, KC_D,
			KC_E, KC_F, KC_G, KC_H,
		},
	}
	for l := 1; l < NumLayers; l++ {
		for k := range km[l] {
			km[l][k] = KC_TRANSPARENT
		}
	}
	return km
}

// 現在(RAM上)のキーマップ。起動時は初期キーマップ。Flashに有効なデータがあれば main で入れ替える。
var keymap = defaultKeymap()

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
