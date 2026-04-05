# Changelog

## [0.5.0](https://github.com/jpalczewski/qlx/compare/v0.4.0...v0.5.0) (2026-04-05)


### Features

* **inventory:** tokenized quick entry with [@container](https://github.com/container) and #tag tokens ([#105](https://github.com/jpalczewski/qlx/issues/105)) ([99b99f2](https://github.com/jpalczewski/qlx/commit/99b99f2e9fd8d386b88772dca38c94101929c64b))
* **stats:** add statistics page ([#104](https://github.com/jpalczewski/qlx/issues/104)) ([0493bc5](https://github.com/jpalczewski/qlx/commit/0493bc5dd4fab5796ac9716850edcc6b3818cf27))


### Bug Fixes

* correct Brother capabilities endpoint URL ([#97](https://github.com/jpalczewski/qlx/issues/97)) ([2c42633](https://github.com/jpalczewski/qlx/commit/2c42633265610cd29dc997cacf3c705eae5f58f7))
* **print:** correct capabilities endpoint URL ([#95](https://github.com/jpalczewski/qlx/issues/95)) ([7e9edf1](https://github.com/jpalczewski/qlx/commit/7e9edf18d80a5764323232ef82f08048659f6a5d))
* **tags:** show + button on items/containers with no tags ([#103](https://github.com/jpalczewski/qlx/issues/103)) ([78520a4](https://github.com/jpalczewski/qlx/commit/78520a457d766ffcadc5cfcaa5de8104e2b89ae3))


### Miscellaneous

* **print:** split event broker and handler ports ([#101](https://github.com/jpalczewski/qlx/issues/101)) ([cb75738](https://github.com/jpalczewski/qlx/commit/cb7573884e36da692a282a6892c894b807d15844))

## [0.4.0](https://github.com/jpalczewski/qlx/compare/v0.3.0...v0.4.0) (2026-03-28)


### Features

* container export modal with CSV/JSON/Markdown ([#20](https://github.com/jpalczewski/qlx/issues/20)) ([#81](https://github.com/jpalczewski/qlx/issues/81)) ([2eab2a2](https://github.com/jpalczewski/qlx/commit/2eab2a2c05e35f39ffbf843899a2d324918b9ada))
* **inventory:** move to container — keyboard shortcut m + UI buttons ([#94](https://github.com/jpalczewski/qlx/issues/94)) ([e6ed6ff](https://github.com/jpalczewski/qlx/commit/e6ed6fffa7b7bdc17dcc438bc919824d51f35ff1))
* Notes (sticky annotations) — full stack ([#84](https://github.com/jpalczewski/qlx/issues/84)) ([1e2ef73](https://github.com/jpalczewski/qlx/commit/1e2ef736e7507ce1f72db5f67a25995290db18e6))
* Phase 1a — template batch (fonts, container/ad-hoc print, calibration) ([#68](https://github.com/jpalczewski/qlx/issues/68)) ([a1de5bd](https://github.com/jpalczewski/qlx/commit/a1de5bddd4d94dde18c4a38d8d0169dfc62a542a))
* print options & renderer pipeline ([#92](https://github.com/jpalczewski/qlx/issues/92)) ([c3d9638](https://github.com/jpalczewski/qlx/commit/c3d9638b10214377df004ce6535146d4ab8e20c1))
* **print:** async printer startup — frontend available immediately ([#91](https://github.com/jpalczewski/qlx/issues/91)) ([5eab656](https://github.com/jpalczewski/qlx/commit/5eab6565d6e1e41890a92c8d6bd004a3b13d4519))
* **print:** Brother QL-700 bidirectional USB via gousb ([#71](https://github.com/jpalczewski/qlx/issues/71)) ([4783973](https://github.com/jpalczewski/qlx/commit/47839733154bc63005c51a93dada0ecafe0e5bee))
* **store:** migrate from JSON file store to SQLite ([#76](https://github.com/jpalczewski/qlx/issues/76)) ([c9b809d](https://github.com/jpalczewski/qlx/commit/c9b809d04b75f63224ac22c31e71da7fd06242f7))
* **ui:** keyboard shortcuts for power users ([#31](https://github.com/jpalczewski/qlx/issues/31)) ([#87](https://github.com/jpalczewski/qlx/issues/87)) ([3d27d7b](https://github.com/jpalczewski/qlx/commit/3d27d7b2b3a98a23d0bba08828f922c28afefb5a))
* **ui:** refactor print components and add label preview ([#80](https://github.com/jpalczewski/qlx/issues/80)) ([65f1acf](https://github.com/jpalczewski/qlx/commit/65f1acfb825fa3b79523f8e6a84ecb6e51378ea2))
* **ui:** style tags as clickable pills with icon and color ([#85](https://github.com/jpalczewski/qlx/issues/85)) ([0ebf5bb](https://github.com/jpalczewski/qlx/commit/0ebf5bb21c66f4491d2102c6e83cd7de9bbddac1))


### Bug Fixes

* **ci:** fix Claude Code Review workflow prompt and remove broken plugin config ([4117ec3](https://github.com/jpalczewski/qlx/commit/4117ec3b3485fa67e95900072ac1353db531c3dc))
* critical code quality issues ([#73](https://github.com/jpalczewski/qlx/issues/73)) ([cfddb9e](https://github.com/jpalczewski/qlx/commit/cfddb9e6d0920478005d3e55305300ccdc27b675))
* **i18n:** translate hardcoded English strings to i18n system ([#93](https://github.com/jpalczewski/qlx/issues/93)) ([43f7a39](https://github.com/jpalczewski/qlx/commit/43f7a397bbf1f68fec5480e7218ec377efc761b8))
* **store:** support map format in JSON migration ([#77](https://github.com/jpalczewski/qlx/issues/77)) ([538633a](https://github.com/jpalczewski/qlx/commit/538633ab08e4af0ec4024bad709aa9c42cae33a1))
* **ui:** correct drag-drop HTTP method and re-init after partial swaps ([#78](https://github.com/jpalczewski/qlx/issues/78)) ([b8d3079](https://github.com/jpalczewski/qlx/commit/b8d30794575a357b255482d226a1f4206244edb1))
* **ui:** tree picker search fix & reskin ([#88](https://github.com/jpalczewski/qlx/issues/88)) ([4892521](https://github.com/jpalczewski/qlx/commit/489252170a6ff71710c29ad35770af2dd5ee0880))


### Miscellaneous

* **handler:** auto-discover template subdirectories ([#89](https://github.com/jpalczewski/qlx/issues/89)) ([fb4c2d6](https://github.com/jpalczewski/qlx/commit/fb4c2d63a5c88cb8b9e780d171f9c80147c374bd)), closes [#44](https://github.com/jpalczewski/qlx/issues/44)

## [0.3.0](https://github.com/jpalczewski/qlx/compare/v0.2.0...v0.3.0) (2026-03-22)


### Features

* icons and colors for containers, items and tags ([#9](https://github.com/jpalczewski/qlx/issues/9), [#10](https://github.com/jpalczewski/qlx/issues/10)) ([#50](https://github.com/jpalczewski/qlx/issues/50)) ([ec542f2](https://github.com/jpalczewski/qlx/commit/ec542f2b5f27babc7762fea3eee52c2579e1b5bc))
* **label:** configurable JSON schemas with Terminus TTF font and Polish character support ([#51](https://github.com/jpalczewski/qlx/issues/51)) ([6dbbcdc](https://github.com/jpalczewski/qlx/commit/6dbbcdcb7decc894c860bce173b4b9ee13f65a6b))
* **label:** replace Terminus with Spleen fonts, add micro schema and Polish transliteration ([#52](https://github.com/jpalczewski/qlx/issues/52)) ([293dfe0](https://github.com/jpalczewski/qlx/commit/293dfe00931f96a809c64688ac8f3420773ecf10))
* **ui:** collapsible description in quick-entry forms ([#54](https://github.com/jpalczewski/qlx/issues/54)) ([ad0db8e](https://github.com/jpalczewski/qlx/commit/ad0db8e43d96f495038629e0e93311983be9d9f5))


### Bug Fixes

* **ui:** fix tag navigation, dropdown positioning, and list chip styling ([#53](https://github.com/jpalczewski/qlx/issues/53)) ([b173652](https://github.com/jpalczewski/qlx/commit/b1736522bac4bc83fe1fecccb6b9c067b451fad9))


### Miscellaneous

* deduplicate bulk types, add validation, partition store, complete edit UX ([#33](https://github.com/jpalczewski/qlx/issues/33), [#42](https://github.com/jpalczewski/qlx/issues/42), [#43](https://github.com/jpalczewski/qlx/issues/43), [#45](https://github.com/jpalczewski/qlx/issues/45)) ([#48](https://github.com/jpalczewski/qlx/issues/48)) ([f3c724a](https://github.com/jpalczewski/qlx/commit/f3c724ae37a9a8d5f39cf7151b1903cb78fb5df1))
* unify api/ and ui/ into handler/ with content negotiation ([#67](https://github.com/jpalczewski/qlx/issues/67)) ([dc7ac93](https://github.com/jpalczewski/qlx/commit/dc7ac93f9fa37d297ff1d11e1cd88b93be8fcba2))

## [0.2.0](https://github.com/jpalczewski/qlx/compare/v0.1.0...v0.2.0) (2026-03-21)


### Features

* add --trace flag for hex dump printer communication logging ([3616b16](https://github.com/jpalczewski/qlx/commit/3616b161be16a29fe93f2048a3ccbd22e97bec39))
* add Playwright E2E test suite with validation fixes ([#6](https://github.com/jpalczewski/qlx/issues/6)) ([79364f8](https://github.com/jpalczewski/qlx/commit/79364f8bfe02f602e8adc11f959bb77b226da201))
* add visual label template designer ([#3](https://github.com/jpalczewski/qlx/issues/3)) ([cd39222](https://github.com/jpalczewski/qlx/commit/cd39222c931cf6aecb252ef94e9db29d9f194a9f))
* **api:** add BLE scan endpoint and transport factory support ([180870e](https://github.com/jpalczewski/qlx/commit/180870e4004ce4e296405437f9f19252d90eeff3))
* **api:** add printer management and print endpoints ([af8313d](https://github.com/jpalczewski/qlx/commit/af8313d670325e0237990410bc01ab31ddbd7530))
* batch operations, tags, and search — design & plan ([#5](https://github.com/jpalczewski/qlx/issues/5)) ([6300f6f](https://github.com/jpalczewski/qlx/commit/6300f6fb567674dcdfab0f56b4a6f8e3afb3ff42))
* **ble:** add BLE transport and discovery via CoreBluetooth ([8d1c611](https://github.com/jpalczewski/qlx/commit/8d1c61167339428ab93ffbf356bdf4f83fd3bd04))
* **brother:** implement QL-700 raster encoder ([3f7ace4](https://github.com/jpalczewski/qlx/commit/3f7ace4067b8e7815ffd040b6793969378ac2c42))
* **build:** add BLE transport and scan stubs for non-BLE builds ([5fe902c](https://github.com/jpalczewski/qlx/commit/5fe902cbeb3194916919422029f551c74e526bb3))
* **build:** add minimal build tag to exclude serial transport ([ce1820d](https://github.com/jpalczewski/qlx/commit/ce1820d402fa1c55397e6dabd9344c5cffdc6489))
* **label:** add label renderer with 4 templates ([d6f5dd9](https://github.com/jpalczewski/qlx/commit/d6f5dd99dea4c9cd68044d28615b86c6dda7f701))
* **niimbot:** add 50x20mm/384pcs label barcode to offline db ([ea7f375](https://github.com/jpalczewski/qlx/commit/ea7f375ec8dc3fdbba2ee064f81250d29941ddf9))
* **niimbot:** implement B1 encoder with packet protocol ([851495d](https://github.com/jpalczewski/qlx/commit/851495daf5146ad75e33f28a25a09d6d8bbb7d19))
* **niimbot:** implement Heartbeat, RfidInfo, and Connect status queries ([f368d63](https://github.com/jpalczewski/qlx/commit/f368d63e2d70766fe92ae24217488dfffeb52f83))
* **niimbot:** implement packet format with checksum ([9c4d55b](https://github.com/jpalczewski/qlx/commit/9c4d55b6941c684beccef706be4d7a0eaa4aa509))
* **print:** add Encoder interface, Brother QL-700 and Niimbot B1 model definitions ([ef5ff2c](https://github.com/jpalczewski/qlx/commit/ef5ff2c17a36565eae25d170d6ee6c97f8a3ce62))
* **print:** add PrinterManager with persistent sessions, heartbeat, and SSE ([12d0823](https://github.com/jpalczewski/qlx/commit/12d0823959f7f308b40cd1f0e65ba49520cb8d68))
* **print:** add PrinterSession with persistent connection and heartbeat ([dd38582](https://github.com/jpalczewski/qlx/commit/dd385828dd36701546b3bc26eefe5f7e69a728db))
* **print:** add PrinterStatus model and StatusQuerier interface ([3b6b375](https://github.com/jpalczewski/qlx/commit/3b6b375e93acf569b7f19bb68f0c1c1d79ed1b0f))
* **print:** add PrintService orchestrating render, encode, transport ([54ae8a5](https://github.com/jpalczewski/qlx/commit/54ae8a52d01f536607612de58c00ba1af6d64a13))
* **print:** add Transport interface and MockTransport ([cc51f9d](https://github.com/jpalczewski/qlx/commit/cc51f9d023970e5379162821cb6bafe689a5475b))
* **print:** add USB, serial, and remote transports ([2641e1c](https://github.com/jpalczewski/qlx/commit/2641e1cb6785fed8e420b4e036b6b8456d912f59))
* show label size (mm) from RFID barcode offline database ([8dbf9d6](https://github.com/jpalczewski/qlx/commit/8dbf9d6320f7dfda22c773ad9309b0b4e5759661))
* **store:** add PrinterConfig persistence with CRUD ([1338970](https://github.com/jpalczewski/qlx/commit/13389704a04b4077bb0f33285b7ec431a6822b66))
* **ui/build:** add BLE scan UI and Mac/MIPS/dev Makefile targets ([253a10f](https://github.com/jpalczewski/qlx/commit/253a10fd8c26a1a73476aae989d6973eba364990))
* **ui:** add printer management page and print from item view ([018593f](https://github.com/jpalczewski/qlx/commit/018593f5d3110aaaca06aa9548fc81068b58db10))
* **ui:** add SSE live printer status in navbar and printer cards ([02a74f6](https://github.com/jpalczewski/qlx/commit/02a74f685bc246f3fb631d319912ce7bb55197c2))
* **ui:** show print width (mm) and DPI in printer status ([669fc9e](https://github.com/jpalczewski/qlx/commit/669fc9e369c93ae333e41e50369494f0248448e3))
* **ui:** use dynamic template list in item print section ([f9a64b6](https://github.com/jpalczewski/qlx/commit/f9a64b6aea6645bbc75d22ad57078382b4e04a76))
* **ui:** use dynamic template list in item print section ([6d1d1bd](https://github.com/jpalczewski/qlx/commit/6d1d1bd0bf9f69787f940cd4ac9ccef3f185ecec))
* wire PrinterManager into api/ui/app with SSE and status endpoints ([a10c523](https://github.com/jpalczewski/qlx/commit/a10c5236b64502054474a9736e54a09a566688f0))
* write trace log to data/trace.log with timestamps ([705a894](https://github.com/jpalczewski/qlx/commit/705a894551667895386ed177dccc4acec4e75267))


### Bug Fixes

* **ci:** migrate exclude-rules to linters.exclusions.rules (v2 schema) ([c1e3348](https://github.com/jpalczewski/qlx/commit/c1e33488c288d5b3db5073569e16f77c2b2a83c4))
* **ci:** pin golangci-lint v2.11.3 (built with Go 1.26, supports Go 1.25) ([ec2e073](https://github.com/jpalczewski/qlx/commit/ec2e073f09ba129440e880baa01171bee1b2989c))
* **ci:** upgrade to golangci-lint-action v8, checkout v5, setup-go v6 ([6cb07f6](https://github.com/jpalczewski/qlx/commit/6cb07f64e59b44255b823578e354ce80bca9882b))
* **ci:** use golangci-lint-action v7 for Go 1.25 compatibility ([2980898](https://github.com/jpalczewski/qlx/commit/2980898797cc21cb2d58123722edff86ef8a2e4f))
* **label:** auto-truncate barcode content when too wide for printhead ([4bed4d3](https://github.com/jpalczewski/qlx/commit/4bed4d3031bfc89fbe42d8eb14c8eb1a1dbfac3e))
* log error details on print failure and 500 responses ([203cb1a](https://github.com/jpalczewski/qlx/commit/203cb1ac95803eb4ac7eb958dea156f6c8c892b5))
* **niimbot:** add Connect step, packet sync, skip unsolicited responses ([e2c6613](https://github.com/jpalczewski/qlx/commit/e2c6613da1ba4ab24ef927bd4c42be29199ad8d7))
* resolve all golangci-lint findings ([bf137c7](https://github.com/jpalczewski/qlx/commit/bf137c7c09fcc8a8100bd5a42a27f6e477cce1ee))
* **ui:** expose showToast globally, fetch initial printer statuses ([0b7b7f0](https://github.com/jpalczewski/qlx/commit/0b7b7f0d97977953a1a71ea04fd9b10573127ea6))
