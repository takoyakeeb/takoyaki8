package main

// Keymap[レイヤー][キー番号] = キーコード(値の意味は keycode.go)
// 固定サイズの配列なので、将来バイト列に変換して保存・転送しやすい。
type Keymap [NumLayers][NumKeys]uint16

// 初期のキーマップ(defaultKeymap)は、機種ごとに違うので、model_takoyaki8.go にある。

// 現在(RAM上)のキーマップ。起動時は初期キーマップ。Flashに有効なデータがあれば main で入れ替える。
var keymap = defaultKeymap()

// Lookup は、有効なレイヤー active で、キー番号idに割り当てられたキーコードを返す。
// active は、ビットnが1ならレイヤーnが有効(layer.go の activeLayers)。レイヤー0は常に有効として扱う。
// 番号の大きい有効なレイヤーから順に見て、最初に見つかった「透過でない」キーコードを返す。
// 有効なレイヤーがすべて透過なら KC_NO。
func (k *Keymap) Lookup(active uint32, id uint16) uint16 {
	if int(id) >= NumKeys {
		return KC_NO
	}
	active |= 1 // レイヤー0は常に有効
	for l := NumLayers - 1; l >= 0; l-- {
		if active&(1<<uint(l)) == 0 {
			continue // 有効でないレイヤーは飛ばす
		}
		if code := k[l][id]; code != KC_TRANSPARENT {
			return code
		}
	}
	return KC_NO
}
