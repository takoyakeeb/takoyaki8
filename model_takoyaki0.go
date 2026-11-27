//go:build takoyaki0

package main

import "machine"

// このファイルは、機種 takoyaki0(entry。基板 rev1.0)の値をまとめた場所。
// ビルドするときに -tags takoyaki0 を付けたときだけ使われる(付けないときは model_takoyaki8.go)。
//   tinygo build -target=waveshare-rp2040-zero -tags takoyaki0 -o <出力ファイル> .
// takoyaki0 は、ピンと行列が takoyaki8 と同じ。違いは、機種名と、使う部品(LED は RP2040-Zero の1個だけ、OLED なし)。
// 機種がコアに渡すもの: 機種名、行と列のピン、レイヤー数、初期キーマップ、レイヤー名の初期値、
// 使う部品(LED のピン・個数・明るさの上限、OLED の I2C・ピン・アドレス・画面の大きさ)。
// 行数・列数・キー数は、ピンの数から自動で決まる(config.go)。
// 回路図は takoyakeeb/takoyaki0 の hardware/。配線を変えたら、このファイルも直す。

// ===== 機種名(INFOコマンドの name) =====
const deviceName = "takoyaki0"

// ===== ピン割り当て(COL2ROW) =====
// 配列(要素数が決まった並び)で書くと、行数・列数をピンの数から自動で決められる。
// takoyaki8 と同じ(GP0=ROW0、GP1=ROW1、GP2〜GP5=COL0〜COL3)。
var (
	colPins = [...]machine.Pin{machine.GPIO2, machine.GPIO3, machine.GPIO4, machine.GPIO5}
	rowPins = [...]machine.Pin{machine.GPIO0, machine.GPIO1}
)

// ===== レイヤー数 =====
const NumLayers = 4

// ===== 使う部品 =====
// コア(main.go)が、起動時・ループごと・キーを押したときに、並べた順に呼ぶ。
var parts = []Part{
	// LED(WS2812)。RP2040-Zero に載っている1個(GPIO16)だけを使う。基板に LED は付けない。
	// maxLevel は、takoyaki8 と同じ 255(仮)。明るさと、色の並び(赤と緑が入れ替わらないか)は、実機で確認が必要。
	&ledPart{pin: machine.GPIO16, count: 1, maxLevel: 255},

	// OLED はない。並べないので、設定の oled・rotate は CFG LIST に出ない。
	// OLED の部品のコードは、使われないのでリンクで消えるはず(サイズで確認する)。
}

// ===== キーの並び(LAYOUT コマンドで返す。tinykeemap が画面に描く) =====
// KLE(keyboard-layout-editor.com)の JSON を1行にしたもの。決まり:
// ・改行・空白を入れない。ASCII だけ。回転(r, rx, ry)と装飾(色・文字の大きさなど)は入れない
// ・キーの文字は「行,列」(tinykeemap が index = 行 × 列数 + 列 に対応づけ、範囲外や重なりを検査する)
// ・幅・高さが 1u でないキーだけ、{"w":1.5} のように前に書く
// いまは takoyaki8 と同じ 2×4 の格子(RP2040-Zero の場所などの飾りは入れない)。並びがない機種は "" にする(INFO に layout=1 が出ない)。
// キーの並びを変えたら、ここも同じコミットで直す。
const layoutKLE = `[["0,0","0,1","0,2","0,3"],["1,0","1,1","1,2","1,3"]]`

// ===== 初期キーマップとレイヤー名 =====
// いまは takoyaki8 と同じ(2026-11-25に決定。入門用の中身は、あとで決める)。

// defaultKeymap は、初期のキーマップを作って返す。
// レイヤー0(MacOS の操作)とレイヤー1(編集とスクリーンショット)だけ設定し、
// レイヤー2・3は「透過」(何も設定していない = 下のレイヤーと同じ)にする。
// キー番号は、左上から右へ 0〜3、下の行が 4〜7(基板の SW1〜SW8 と同じ並び)。
// どちらのレイヤーも、キー0(左上)がもう一方のレイヤーへ移るキー。
// RESET コマンドや、Flashに有効なデータがないときに使う。
// レイヤーの名前の初期値は、下の lnameDefaults(ここを変えたら、名前も合わせる。tinykeemap のタブ名に使う)。
func defaultKeymap() Keymap {
	km := Keymap{
		// レイヤー0: MacOS(上の行 = ウィンドウの切り替え、下の行 = デスクトップの切り替え)
		{
			QK_TO | 1,                    // 0: レイヤー1(編集・スクリーンショット)へ移る
			MOD_LGUI | MOD_LSFT | KC_TAB, // 1: ⌘+Shift+Tab(アプリを逆順で切り替え)
			MOD_LGUI | KC_TAB,            // 2: ⌘+Tab(アプリを切り替え)
			MOD_LCTL | KC_DOWN,           // 3: Ctrl+↓(アプリのウィンドウ一覧)
			KC_F11,                       // 4: F11(デスクトップを表示)
			MOD_LCTL | KC_LEFT,           // 5: Ctrl+←(前のデスクトップ)
			MOD_LCTL | KC_RIGHT,          // 6: Ctrl+→(次のデスクトップ)
			MOD_LCTL | KC_UP,             // 7: Ctrl+↑(Mission Control)
		},
		// レイヤー1: 編集とスクリーンショット(Mac)。上の行 = スクリーンショット、下の行 = 編集
		{
			QK_TO | 0,                  // 0: レイヤー0(MacOS)へ戻る
			MOD_LGUI | MOD_LSFT | KC_3, // 1: ⌘+Shift+3(画面全体のスクリーンショット)
			MOD_LGUI | MOD_LSFT | KC_4, // 2: ⌘+Shift+4(範囲を選んでスクリーンショット)
			MOD_LGUI | MOD_LSFT | KC_5, // 3: ⌘+Shift+5(スクリーンショットのメニュー)
			MOD_LGUI | KC_Z,            // 4: ⌘+Z(元に戻す)
			MOD_LGUI | KC_X,            // 5: ⌘+X(切り取り)
			MOD_LGUI | KC_C,            // 6: ⌘+C(コピー)
			MOD_LGUI | KC_V,            // 7: ⌘+V(ペースト)
		},
	}
	for l := 2; l < NumLayers; l++ {
		for k := range km[l] {
			km[l][k] = KC_TRANSPARENT
		}
	}
	return km
}

// レイヤー名の初期値(L0 = MAC、L1 = EDIT。それ以外は空)。
// 上の初期キーマップに合わせた名前。
var lnameDefaults = [NumLayers]string{"MAC", "EDIT", "", ""}
