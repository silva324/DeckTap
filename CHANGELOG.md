# Changelog

## [0.11.1](https://github.com/silva324/DeckTap/compare/v0.11.0...v0.11.1) (2026-10-22)


### Bug Fixes

* **test:** wait for deck readiness before pairing in tests ([#53](https://github.com/silva324/DeckTap/issues/53)) ([235b6db](https://github.com/silva324/DeckTap/commit/235b6db97b965ac569f7a2c7f660f3b336cf815a))

## [0.11.0](https://github.com/silva324/DeckTap/compare/v0.10.1...v0.11.0) (2026-10-22)


### Features

* **app:** add native local Wi-Fi LAN mode for offline and corporate presentations ([#52](https://github.com/silva324/DeckTap/issues/52)) ([71801d2](https://github.com/silva324/DeckTap/commit/71801d2fbe561d7ee35ef0eaa9221a061896f445))
* **tunnel:** add ngrok port 443 fallback for corporate networks ([#50](https://github.com/silva324/DeckTap/issues/50)) ([526e1b2](https://github.com/silva324/DeckTap/commit/526e1b27c82b0daed83bbeea744d1ddffcc7193d))

## [0.10.1](https://github.com/silva324/DeckTap/compare/v0.10.0...v0.10.1) (2026-10-22)


### Bug Fixes

* **session:** ensure monotonic host seq and add pure-js qr scanner fallback for ios ([#48](https://github.com/silva324/DeckTap/issues/48)) ([12d6a11](https://github.com/silva324/DeckTap/commit/12d6a1133c79a7c50ce0030d13ff27fcf09a8038))

## [0.10.0](https://github.com/silva324/DeckTap/compare/v0.9.0...v0.10.0) (2026-10-22)


### Features

* **updater:** check updates hourly with ETag caching and add on-demand update button ([#43](https://github.com/silva324/DeckTap/issues/43)) ([82e18d1](https://github.com/silva324/DeckTap/commit/82e18d16468e2d8cf8fc0f410d03d1a4f0322fab))


### Bug Fixes

* **ci:** grant pages and id-token write permissions for release workflows ([#42](https://github.com/silva324/DeckTap/issues/42)) ([e3f35b0](https://github.com/silva324/DeckTap/commit/e3f35b08a39a42ba4e8020ae9f1c42e412bca959))
* **ci:** grant write permissions and pass token for release actions ([#47](https://github.com/silva324/DeckTap/issues/47)) ([a9606f2](https://github.com/silva324/DeckTap/commit/a9606f20ec62426a26a5f54c9c290aa7eb0bd810))
* **sync:** live synchronize slide speaker notes and add on-demand refresh ([#45](https://github.com/silva324/DeckTap/issues/45)) ([1489402](https://github.com/silva324/DeckTap/commit/1489402def7ba49c54b8a20d3ec9fe614b6d3937))


### Documentation

* **agents:** add multi-agent concurrency and git isolation guide ([#44](https://github.com/silva324/DeckTap/issues/44)) ([148b94f](https://github.com/silva324/DeckTap/commit/148b94f52d0042c178a7605058c7d3883aab1e7c))

## [0.9.0](https://github.com/silva324/DeckTap/compare/v0.8.0...v0.9.0) (2026-10-22)


### Features

* **packaging:** add homebrew tap automation and installation docs ([#35](https://github.com/silva324/DeckTap/issues/35)) ([06b6f26](https://github.com/silva324/DeckTap/commit/06b6f2691276725a11f1c28104cbdc3100e83532))


### Bug Fixes

* **ci:** avoid MSYS path conversion for Inno Setup compiler flag ([#38](https://github.com/silva324/DeckTap/issues/38)) ([c3921c9](https://github.com/silva324/DeckTap/commit/c3921c9c97d3bb9c778a4d3f58f1dd843121a373))
* **ci:** fix Windows release package zip command on Windows runners ([#37](https://github.com/silva324/DeckTap/issues/37)) ([5739395](https://github.com/silva324/DeckTap/commit/5739395b209730ed9ae7dcd3274c8ce8341ae442))
* **windows:** honor MyAppVersion define from command line in installer script ([#39](https://github.com/silva324/DeckTap/issues/39)) ([0bd863a](https://github.com/silva324/DeckTap/commit/0bd863a73b274771c754f70e291fb423a1e4d8c9))


### Documentation

* **readme:** streamline features and roadmap, add quick start guide, and move protocol spec to docs ([#40](https://github.com/silva324/DeckTap/issues/40)) ([48b45e4](https://github.com/silva324/DeckTap/commit/48b45e44e6cdf22b58e0b6e1aab3a69fe5e24194))

## [0.8.0](https://github.com/silva324/DeckTap/compare/v0.7.1...v0.8.0) (2026-10-21)


### Features

* **app:** add version display and about menu across desktop and mobile ([#33](https://github.com/silva324/DeckTap/issues/33)) ([a3a7655](https://github.com/silva324/DeckTap/commit/a3a76555b748427340a49a5057ab0cb1f9fab5c5))

## [0.7.1](https://github.com/silva324/DeckTap/compare/v0.7.0...v0.7.1) (2026-10-21)


### Bug Fixes

* **presenter:** enable show with presenter on multi-display setups ([#32](https://github.com/silva324/DeckTap/issues/32)) ([248ce7a](https://github.com/silva324/DeckTap/commit/248ce7a43691c221acf46b740647cc484e794f01))


### Documentation

* **guide:** add docs directory with presenter user guide and architecture spec ([#30](https://github.com/silva324/DeckTap/issues/30)) ([63520fc](https://github.com/silva324/DeckTap/commit/63520fcbcbdc25103075207217c8357bc760ba65))

## [0.7.0](https://github.com/silva324/DeckTap/compare/v0.6.2...v0.7.0) (2026-10-21)


### Features

* **webui:** add smart stage cues and touch pinch-to-zoom for speaker notes ([#28](https://github.com/silva324/DeckTap/issues/28)) ([3ab39d0](https://github.com/silva324/DeckTap/commit/3ab39d0f739db6a6ae9853657db73b95bea01fae))

## [0.6.2](https://github.com/silva324/DeckTap/compare/v0.6.1...v0.6.2) (2026-10-21)


### Bug Fixes

* **presenter:** smart title extraction and missing value filtering across mac, windows, and webui ([#26](https://github.com/silva324/DeckTap/issues/26)) ([de5dad1](https://github.com/silva324/DeckTap/commit/de5dad17c48ecab32b7e7f3ef7eddfa873ddbcd3))

## [0.6.1](https://github.com/silva324/DeckTap/compare/v0.6.0...v0.6.1) (2026-10-21)


### Bug Fixes

* **session:** allow null and webclip origins for ios safari and improve mobile touch handling ([#24](https://github.com/silva324/DeckTap/issues/24)) ([0410713](https://github.com/silva324/DeckTap/commit/04107139af8cf3f503c5ab3d6716aebcc78e66f9))

## [0.6.0](https://github.com/silva324/DeckTap/compare/v0.5.0...v0.6.0) (2026-10-21)


### Features

* **webui:** allow scanning new QR code when reconnecting and enhance PWA launch handling ([#21](https://github.com/silva324/DeckTap/issues/21)) ([7131845](https://github.com/silva324/DeckTap/commit/71318457322c2361383395e7bdd664aa48585823))


### Bug Fixes

* **updater:** prevent html release page download and validate executable headers before replacement ([#23](https://github.com/silva324/DeckTap/issues/23)) ([512ff67](https://github.com/silva324/DeckTap/commit/512ff67aac4649063ec4b90902741cd4b7f9940e))

## [0.5.0](https://github.com/silva324/DeckTap/compare/v0.4.0...v0.5.0) (2026-10-21)


### Features

* **app:** add quit/exit option to desktop UI and native window with Cmd+Q support ([#18](https://github.com/silva324/DeckTap/issues/18)) ([8a2efe9](https://github.com/silva324/DeckTap/commit/8a2efe92476b3c02c52946954ea8e2de813fe2a1))
* **pointer:** query slide show window bounds directly to respect user presentation screen on multi-displays ([#20](https://github.com/silva324/DeckTap/issues/20)) ([a08da80](https://github.com/silva324/DeckTap/commit/a08da8029039a8fdc69d851f2ba9f259956ca623))

## [0.4.0](https://github.com/silva324/DeckTap/compare/v0.3.1...v0.4.0) (2026-10-21)


### Features

* **pointer:** disable IMU tilt by default with toggle and target slide show screen on multi-displays ([#17](https://github.com/silva324/DeckTap/issues/17)) ([b06ba69](https://github.com/silva324/DeckTap/commit/b06ba695dd10322cc1d978dc5acd52750fe25c35))
* **updater:** implement seamless automated in-app self-updater ([#16](https://github.com/silva324/DeckTap/issues/16)) ([dbdc86a](https://github.com/silva324/DeckTap/commit/dbdc86aed97636142b29cb453543726a0a8c77dc))


### Miscellaneous Chores

* **branding:** standardize application naming strictly to DeckTap ([#14](https://github.com/silva324/DeckTap/issues/14)) ([9575462](https://github.com/silva324/DeckTap/commit/9575462eaf05c9b292d24641d0ea2562da1e0bd2))

## [0.3.1](https://github.com/silva324/DeckTap/compare/v0.3.0...v0.3.1) (2026-10-21)


### Miscellaneous Chores

* **ci:** configure pages setup step and trigger deployment ([#12](https://github.com/silva324/DeckTap/issues/12)) ([694026b](https://github.com/silva324/DeckTap/commit/694026b8f50e0b4712868f5320c556f327157ab7))

## [0.3.0](https://github.com/silva324/DeckTap/compare/v0.2.0...v0.3.0) (2026-10-21)


### Features

* **pwa:** canonical GitHub Pages entry point and connection manager ([#10](https://github.com/silva324/DeckTap/issues/10)) ([fcef3c5](https://github.com/silva324/DeckTap/commit/fcef3c5130a3fc2b40a384ee7a6b04072df64714))

## [0.2.0](https://github.com/silva324/DeckTap/compare/v0.1.0...v0.2.0) (2026-10-21)


### Features

* **agents:** add AGENTS.md, agent verification suite, and makefile target ([#5](https://github.com/silva324/DeckTap/issues/5)) ([2c41c15](https://github.com/silva324/DeckTap/commit/2c41c15377503f65cda9957a0104c90d9551c9fc))


### Bug Fixes

* **pwa:** guarantee fixed bottom controls bar visibility with visualViewport sync and overscroll boundary locking ([#7](https://github.com/silva324/DeckTap/issues/7)) ([20aeb57](https://github.com/silva324/DeckTap/commit/20aeb57119a823ab199c953514b4e6df0eb5176d))
* **webui:** rename scroll touch variable to avoid duplicate declaration ([#8](https://github.com/silva324/DeckTap/issues/8)) ([e94d73c](https://github.com/silva324/DeckTap/commit/e94d73c01f94f866d8c37bd446a17d34e27fd60b))


### Documentation

* clarify exploratory roadmap with no-commitment horizons ([#3](https://github.com/silva324/DeckTap/issues/3)) ([82e14fe](https://github.com/silva324/DeckTap/commit/82e14fe7d0d4776f83411067bea4fdaa1834c52f))
* **macos:** add first-launch Gatekeeper instructions to README and DMG package ([#6](https://github.com/silva324/DeckTap/issues/6)) ([6171f4a](https://github.com/silva324/DeckTap/commit/6171f4a722458229cfc357cf70ea573cca70fe7f))


### Miscellaneous Chores

* **agents:** enforce linter and add pre-commit verification hook ([#9](https://github.com/silva324/DeckTap/issues/9)) ([efa4c6a](https://github.com/silva324/DeckTap/commit/efa4c6a32ad8c57760d0eefeeffdedf4e4d307e2))

## [0.1.0](https://github.com/silva324/DeckTap/compare/v0.0.1...v0.1.0) (2026-10-21)


### Features

* initial public release of DeckTap ([de71663](https://github.com/silva324/DeckTap/commit/de716639b9503443cdd1d4f244384cf0229df2bc))
