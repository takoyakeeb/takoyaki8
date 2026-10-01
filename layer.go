package main

// レイヤー切り替えキー(MO など)の処理をまとめたファイル。
// keycode の値は tinykeemap の docs/protocol.md §2 と同じ(QMK の現行の値)。

// momentary[n] は、MO(n) のキーが、いくつ押されているか。
// 同じ MO(n) のキーを2つ押したとき、1つ離しただけで戻らないよう、数で数える。
var momentary [NumLayers]uint8

// momentaryLayer は、キーコードが MO(n) なら、レイヤー番号 n を返す。
// n がこのファームのレイヤー数以上のときは、MO として扱わない(ok=false)。
func momentaryLayer(code uint16) (layer int, ok bool) {
	if code&^QK_LAYER_MASK != QK_MOMENTARY {
		return 0, false
	}
	n := int(code & QK_LAYER_MASK)
	if n >= NumLayers {
		return 0, false
	}
	return n, true
}

// activeLayers は、いま有効なレイヤーを返す。ビットnが1なら、レイヤーnが有効。
// レイヤー0は常に有効。
func activeLayers() uint32 {
	active := uint32(1)
	for l := 1; l < NumLayers; l++ {
		if momentary[l] > 0 {
			active |= 1 << uint(l)
		}
	}
	return active
}
