# hardware/ — takoyaki8 の設計データ

takoyaki8 の基板とケースの設計データです。

Copyright (c) 2026 takoyakeeb
SPDX-License-Identifier: CERN-OHL-P-2.0

## ライセンス

- この `hardware/` の中のファイルは、下の「他人のファイル」を除いて、CERN-OHL-P-2.0(CERN Open Hardware Licence Version 2 - Permissive)です。ケースの図面を作るスクリプトも、ここに含まれます。
- 全文は、リポジトリのルートにある `LICENSE-HARDWARE` を見てください。
- ルートの `LICENSE`(MIT)は、ファームウェアと文書のライセンスです。

## 中身

| 場所 | 内容 |
|---|---|
| `takoyaki8.kicad_pro`、`takoyaki8.kicad_sch`、`takoyaki8.kicad_pcb` | KiCad のプロジェクト、回路図、基板。KiCad 10.0 で作成しています |
| `fp-lib-table`、`sym-lib-table` | ライブラリの表。自作ライブラリ(`lib/`)を指します |
| `lib/` | 自作のシンボル(RP2040-Zero)とフットプリント。出典と、寸法の確認は、`lib/README.md` を見てください |
| `case/` | ケース(アクリル)の寸法メモと、入稿用の図面(SVG)を作るスクリプト。くわしくは、`case/README.md` を見てください |
| `gerber/` | KiCad から書き出した Gerber の置き場所。リポジトリには含めていません(`.gitignore`)。配布は、GitHub のリリースで行う予定です(準備中) |

## 基板

- 2層、板厚 1.6mm。メイン基板とトッププレートを、1枚のパネルにしています。マウスバイトで切り離します。
- KiCad の DRC には、マウスバイトの穴の警告が24件出ます。意図した設計です。
- 発注に使った版には、タグ `hw-rev1.0-order1` が付いています。

## 他人のファイル

- **Choc v2 のホットスワップソケットのフットプリント**(`lib/takoyaki8.pretty/SW_choc_v2_HS_CPG135001S30_1u.kicad_mod`): marbastlib の `marbastlib-xp-choc`(ebastler/marbastlib、CERN-OHL-P-2.0)から、2026-10-05 にコピーしました。寸法は、Kailh の資料と、実物で確認しています(`lib/README.md`)。3D モデル(STEP)は、パスで参照しているだけで、このリポジトリには含めていません。
- **KiCad 標準ライブラリの部品**(Capacitor_SMD、Diode_SMD、LED_SMD、Resistor_SMD): 回路図と基板に、部品として含まれています。KiCad のライブラリは CC-BY-SA 4.0 ですが、例外があり、ライブラリを設計に使うだけなら、設計データを CC-BY-SA にする必要はありません(<https://www.kicad.org/libraries/license/>)。ライブラリのファイルそのものは、このリポジトリに含めていません。
- RP2040-Zero のシンボルとフットプリントは、自作です。ピン配置は、Waveshare 公式の図を見て作りました(図そのものは含めていません)。
- ロゴ(`lib/takoyaki8.pretty/TakoyaKeeb_logo.kicad_mod`、`case/art/`)は、自作です。
