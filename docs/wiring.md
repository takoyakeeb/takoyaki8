# takoyaki8 配線表

配線を変更したら、このファイルと `config.go` のピン定義を同じコミットで更新する。

## 概要
- マイコン: Waveshare RP2040-Zero
- スイッチ: 8キー(2行×4列のマトリクス)
- ダイオード: 各スイッチに1本。COL2ROW(カソード=黒線側をRow側に向ける)
- LED(WS2812/SK6812)とOLEDは未配線(ピンのみ予約)

## ピン割り当て
シリーズ共通ルール: マトリクスはGPIO0から順に使う。周辺機器のピンは固定。

| 役割 | GPIO |
|---|---|
| Row0 / Row1 | GPIO0 / GPIO1 |
| Col0 / Col1 / Col2 / Col3 | GPIO2 / GPIO3 / GPIO4 / GPIO5 |
| LEDデータ(予約・未配線) | GPIO26 |
| OLED SDA / SCL(予約・未配線) | GPIO14 / GPIO15 |
| 予備 | GPIO27 / GPIO28 |
| 使用不可 | GPIO29(基板から引き出されていない)、GPIO16(オンボードLED専用) |

コード上のピン定義は `config.go` にある。

## 配線表
形式: Col GPIO ---SW--- D(アノード→カソード) --- Row GPIO

| キー | 位置 | キー番号(ID) | 配線 |
|---|---|---|---|
| SW1 | R0C0 | 0 | GPIO2 ---SW1--- D1 --- GPIO0 |
| SW2 | R0C1 | 1 | GPIO3 ---SW2--- D2 --- GPIO0 |
| SW3 | R0C2 | 2 | GPIO4 ---SW3--- D3 --- GPIO0 |
| SW4 | R0C3 | 3 | GPIO5 ---SW4--- D4 --- GPIO0 |
| SW5 | R1C0 | 4 | GPIO2 ---SW5--- D5 --- GPIO1 |
| SW6 | R1C1 | 5 | GPIO3 ---SW6--- D6 --- GPIO1 |
| SW7 | R1C2 | 6 | GPIO4 ---SW7--- D7 --- GPIO1 |
| SW8 | R1C3 | 7 | GPIO5 ---SW8--- D8 --- GPIO1 |

キー番号は行優先:

```
Row0:  0  1  2  3
Row1:  4  5  6  7
```

## 共通線のまとめ方
- Col0(GPIO2): SW1とSW5の片足
- Col1(GPIO3): SW2とSW6
- Col2(GPIO4): SW3とSW7
- Col3(GPIO5): SW4とSW8
- Row0(GPIO0): D1〜D4のカソード(黒線側)
- Row1(GPIO1): D5〜D8のカソード(黒線側)

## 部品の向き
- ダイオードは黒線側がカソード。電流はアノード(黒線と反対側)からカソードへ流れる
- COL2ROWなので、黒線側をすべてRow側に向ける
- スイッチの足は2本。どちらをColにつないでも構わない

## 動作確認済みの内容
- 8キーすべてが反応し、左上から順に `abcdefgh` の順で出力(仮キー配置)
- 隣り合う2キーの同時押しで、余計な入力なし
- L字3キー同時押し(a b e / c d h)で、ゴースト(f / g)なし
