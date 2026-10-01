package main

// レイヤー切り替えキー(MO / TG / TO)の処理をまとめたファイル。
// keycode の値は tinykeemap の docs/protocol.md §2 と同じ(QMK の現行の値)。
// 動きの決まりは protocol.md §3 の「レイヤーの動き」。

// layerKind は、レイヤー切り替えキーの種類。
type layerKind uint8

const (
	// MO(n): 押している間だけ、レイヤー n を有効にする
	layerMO layerKind = iota + 1
	// TG(n): 押すたびに、レイヤー n の有効・無効を切り替える
	layerTG
	// TO(n): 切り替えで有効にしているレイヤーを、n だけにする
	layerTO
)

// momentary[n] は、MO(n) のキーが、いくつ押されているか。
// 同じ MO(n) のキーを2つ押したとき、1つ離しただけで戻らないよう、数で数える。
var momentary [NumLayers]uint8

// toggled は、TG / TO で有効にしているレイヤー。ビットnが1なら、レイヤーnが有効。
// レイヤー0は常に有効なので、ビット0は使わない。保存はしない(起動時は0)。
var toggled uint32

// decodeLayerKey は、キーコードが MO(n) / TG(n) / TO(n) なら、その種類とレイヤー番号 n を返す。
// n がこのファームのレイヤー数以上のときは、レイヤー切り替えとして扱わない(ok=false)。
func decodeLayerKey(code uint16) (kind layerKind, layer int, ok bool) {
	switch code &^ QK_LAYER_MASK {
	case QK_MOMENTARY:
		kind = layerMO
	case QK_TOGGLE_LAYER:
		kind = layerTG
	case QK_TO:
		kind = layerTO
	default:
		return 0, 0, false
	}
	n := int(code & QK_LAYER_MASK)
	if n >= NumLayers {
		return 0, 0, false
	}
	return kind, n, true
}

// toggleLayer は、TG(n) の動き。レイヤー n の有効・無効を切り替える。n = 0 は何もしない。
func toggleLayer(n int) {
	if n > 0 {
		toggled ^= 1 << uint(n)
	}
}

// moveToLayer は、TO(n) の動き。切り替えで有効にしているレイヤーを、n だけにする。
// n = 0 なら、切り替えをすべて解除する(土台のレイヤー0だけに戻る)。
// 押している間の MO(momentary)には影響しない。
func moveToLayer(n int) {
	toggled = 0
	if n > 0 {
		toggled = 1 << uint(n)
	}
}

// resetLayerState は、TG / TO の状態を解除して、レイヤー0だけに戻す。
// RESET と、成功した LOAD から呼ぶ。押している間の MO(momentary)はそのまま。
func resetLayerState() {
	toggled = 0
}

// activeLayers は、いま有効なレイヤーを返す。ビットnが1なら、レイヤーnが有効。
// レイヤー0は常に有効。TG / TO で有効にしたものと、MO を押している間のものが加わる。
func activeLayers() uint32 {
	active := uint32(1) | toggled
	for l := 1; l < NumLayers; l++ {
		if momentary[l] > 0 {
			active |= 1 << uint(l)
		}
	}
	return active
}
