package main

import "machine"

// このファイルは、機種 takoyaki8(基板 rev1.0)の値をまとめた場所。
// 機種がコアに渡すもの: 機種名、行と列のピン、レイヤー数、初期キーマップ、レイヤー名の初期値、
// LED のピン・個数・明るさの上限、OLED のピン・アドレス・画面の大きさ。
// 行数・列数・キー数は、ピンの数から自動で決まる(config.go)。
// ピン配置と配線表は docs/wiring.md。配線を変えたら、このファイルも同じコミットで直す。

// ===== 機種名(INFOコマンドの name) =====
const deviceName = "takoyaki8"

// ===== ピン割り当て(COL2ROW) =====
// 配列(要素数が決まった並び)で書くと、行数・列数をピンの数から自動で決められる。
var (
	colPins = [...]machine.Pin{machine.GPIO2, machine.GPIO3, machine.GPIO4, machine.GPIO5}
	rowPins = [...]machine.Pin{machine.GPIO0, machine.GPIO1}
)

// ===== レイヤー数 =====
const NumLayers = 4

// ===== LED(WS2812B) =====
const (
	pinLED  = machine.GPIO26
	NumLEDs = 4 // つないだ LED の数(基板 rev1.0 は4個)

	// 設定 led が 100% のときの、赤・緑・青それぞれの値(0〜255)。255 が最大。
	// 段階8aで、128 から 255 に上げた(暗い側の段を増やすため)。
	// 4個を白で全灯すると、128 で約120mA、255 で約240mA(目安)。USB の 500mA に収まる。
	// ちらつきと発熱は、段階8aの実機で確認する。
	ledMaxLevel = 255
)

// ===== OLED(SSD1306、128x32、I2C1) =====
// I2C の番号(I2C1)は、まだ oled.go に直接書いてある(GPIO14/15 は I2C1)。
const (
	pinSDA = machine.GPIO14
	pinSCL = machine.GPIO15

	// I2C アドレス(7ビット)。スキャンで 0x3C と確認済み。
	oledAddress = 0x3C

	oledWidth  = 128 // 画面の幅(ドット。実際の画面)
	oledHeight = 32  // 画面の高さ(ドット。実際の画面)
)

// ===== 予備のピン(未使用。シリーズ共通ルールの記録用) =====
const (
	pinSpare1 = machine.GPIO27
	pinSpare2 = machine.GPIO28
)

// ===== 初期キーマップとレイヤー名 =====

// defaultKeymap は、初期のキーマップを作って返す。
// レイヤー0(MacOS の操作)とレイヤー1(編集とスクリーンショット)だけ設定し、
// レイヤー2・3は「透過」(何も設定していない = 下のレイヤーと同じ)にする。
// キー番号は、左上から右へ 0〜3、下の行が 4〜7(基板の SW1〜SW8 と同じ並び)。
// どちらのレイヤーも、キー0(左上)がもう一方のレイヤーへ移るキー。
// RESET コマンドや、Flashに有効なデータがないときに使う。
// OLED に出すレイヤーの名前の初期値は、下の lnameDefaults(ここを変えたら、名前も合わせる)。
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
// 上の初期キーマップに合わせた名前。空の名前は、OLED に名前を出さない。
var lnameDefaults = [NumLayers]string{"MAC", "EDIT", "", ""}
