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

// baseLayer は、「土台のレイヤー」の番号を返す。TG / TO で有効にしているレイヤーのうち、いちばん番号が大きいもの。
// 1つもなければ 0。押している間の MO(momentary)は数えない。次・前のレイヤーは、ここから数える。
func baseLayer() int {
	for l := NumLayers - 1; l > 0; l-- {
		if toggled&(1<<uint(l)) != 0 {
			return l
		}
	}
	return 0
}

// layerIsUsed は、レイヤー l が「使われている」かどうかを返す。
// レイヤー0は、いつも使われているものとして扱う(飛ばさない)。
// それ以外は、「透過」でないキーが1つでもあれば使われている。すべて透過なら、使われていない。
func layerIsUsed(l int) bool {
	if l == 0 {
		return true
	}
	for _, code := range keymap[l] {
		if code != KC_TRANSPARENT {
			return true
		}
	}
	return false
}

// stepLayer は、次のレイヤー(dir = 1)・前のレイヤー(dir = -1)の動き。
// 土台のレイヤーから、dir の向きに1つずつ見て、最初に見つかった「使われている」レイヤーへ、TO(n) と同じように移る。
// 最後の「次」は 0 に戻り、0 の「前」は最後に移る(ぐるっと回る)。
// 移れるレイヤーがほかになければ(使われているのが L0 だけのとき)、何も起きない。
func stepLayer(dir int) {
	base := baseLayer()
	for i := 1; i < NumLayers; i++ {
		// (base + dir*i) を 0〜NumLayers-1 の範囲に収める(マイナスになっても、足し直してから % で割る)
		l := ((base+dir*i)%NumLayers + NumLayers) % NumLayers
		if layerIsUsed(l) {
			moveToLayer(l)
			return
		}
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

// currentLayer は、いま「いちばん上」の有効なレイヤーの番号を返す。
// keymap.Lookup が最初に見るレイヤーと同じ決め方(番号の大きい有効なレイヤー)。
// OLED の表示に使う。
func currentLayer() int {
	active := activeLayers()
	for l := NumLayers - 1; l > 0; l-- {
		if active&(1<<uint(l)) != 0 {
			return l
		}
	}
	return 0
}
