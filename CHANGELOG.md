# Changelog

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
