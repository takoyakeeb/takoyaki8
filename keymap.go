package main

// Keymap[レイヤー][キー番号] = キーコード(値の意味は keycode.go)
// 固定サイズの配列なので、将来バイト列に変換して保存・転送しやすい。
type Keymap [NumLayers][NumKeys]uint16

// defaultKeymap は、初期のキーマップを作って返す。
// レイヤー0(MacOS の操作)とレイヤー1(Google Meet)だけ設定し、
// レイヤー2・3は「透過」(何も設定していない = 下のレイヤーと同じ)にする。
// キー番号は、左上から右へ 0〜3、下の行が 4〜7(基板の SW1〜SW8 と同じ並び)。
// どちらのレイヤーも、キー0(左上)がもう一方のレイヤーへ移るキー。
// RESET コマンドや、Flashに有効なデータがないときに使う。
func defaultKeymap() Keymap {
	km := Keymap{
		// レイヤー0: MacOS(ウィンドウ・デスクトップの操作)
		{
			QK_TO | 1,                    // 0: レイヤー1(Meet)へ移る
			MOD_LGUI | MOD_LSFT | KC_TAB, // 1: ⌘+Shift+Tab(アプリを逆順で切り替え)
			MOD_LGUI | KC_TAB,            // 2: ⌘+Tab(アプリを切り替え)
			MOD_LCTL | KC_UP,             // 3: Ctrl+↑(Mission Control)
			KC_F11,                       // 4: F11(デスクトップを表示)
			MOD_LCTL | KC_LEFT,           // 5: Ctrl+←(前のデスクトップ)
			MOD_LCTL | KC_RIGHT,          // 6: Ctrl+→(次のデスクトップ)
			MOD_LCTL | KC_DOWN,           // 7: Ctrl+↓(アプリのウィンドウ一覧)
		},
		// レイヤー1: Google Meet(Mac)
		{
			QK_TO | 0,                  // 0: レイヤー0(MacOS)へ戻る
			MOD_LCTL | MOD_LGUI | KC_H, // 1: Ctrl+⌘+H(挙手)
			MOD_LGUI | KC_D,            // 2: ⌘+D(マイクのミュート切り替え)
			MOD_LGUI | KC_E,            // 3: ⌘+E(カメラのオン・オフ)
			MOD_LCTL | MOD_LGUI | KC_C, // 4: Ctrl+⌘+C(チャット)
			MOD_LCTL | MOD_LGUI | KC_P, // 5: Ctrl+⌘+P(参加者)
			MOD_LCTL | MOD_LGUI | KC_M, // 6: Ctrl+⌘+M(自分の動画を最小化・拡大)
			MOD_LSFT | KC_M,            // 7: Shift+M(ピクチャインピクチャ)
		},
	}
	for l := 2; l < NumLayers; l++ {
		for k := range km[l] {
			km[l][k] = KC_TRANSPARENT
		}
	}
	return km
}

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
