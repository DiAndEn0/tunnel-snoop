# Changelog

## [1.2.0](https://github.com/DiAndEn0/tunnel-snoop/compare/v1.1.0...v1.2.0) (2026-08-31)


### Features

* **procfs:** scope tunnel discovery to the invoking user's processes ([#10](https://github.com/DiAndEn0/tunnel-snoop/issues/10)) ([cb6e905](https://github.com/DiAndEn0/tunnel-snoop/commit/cb6e90512a8b4843ef9f2bdd7f1cbb1672d47de1))


### Bug Fixes

* **monitor:** scope client counts to the tunnel's local address ([#11](https://github.com/DiAndEn0/tunnel-snoop/issues/11)) ([440f37e](https://github.com/DiAndEn0/tunnel-snoop/commit/440f37e6c492f898bf5ae5ab2fa03aa32e61e1a3))
* **reaper:** re-verify process identity before SIGKILL escalation ([#9](https://github.com/DiAndEn0/tunnel-snoop/issues/9)) ([f4ae84b](https://github.com/DiAndEn0/tunnel-snoop/commit/f4ae84b236adcee5a3dbc12c6a9ea0defbfd873e))

## [1.1.0](https://github.com/DiAndEn0/tunnel-snoop/compare/v1.0.0...v1.1.0) (2026-08-31)


### Features

* **cli:** add -version flag ([6e5026e](https://github.com/DiAndEn0/tunnel-snoop/commit/6e5026e7d6e59675c66d8b1ee87e2a92ed8c3a47))
* **cli:** add -version flag ([65f6c55](https://github.com/DiAndEn0/tunnel-snoop/commit/65f6c55333c866c3a9086ba25207220798d38390))

## 1.0.0 (2026-08-31)


### Features

* **cli:** wire engine, reaper, and UI into tunnelsnoop binary ([613c6c0](https://github.com/DiAndEn0/tunnel-snoop/commit/613c6c0a7f4cf302827ebe07fc3b6a6b163512a9))
* **model:** define core tunnel and socket data models ([d0ceb7c](https://github.com/DiAndEn0/tunnel-snoop/commit/d0ceb7cc931dedda876cbb384198e35514a74440))
* **monitor:** implement state reconciliation and idle calculator ([00488ad](https://github.com/DiAndEn0/tunnel-snoop/commit/00488addfde1af6df91f585e561e7f81a0f9d3e6))
* **procfs:** add socket parser for IPv4 and IPv6 tables ([669e4f8](https://github.com/DiAndEn0/tunnel-snoop/commit/669e4f8e7c192667700a454f72a34406a88be627))
* **procfs:** correlate processes with socket inodes ([3fe17bf](https://github.com/DiAndEn0/tunnel-snoop/commit/3fe17bf2cc24ef9a4018b7f249c2a9837ec6c94c))
* **procfs:** implement process I/O accounting reader ([00f1157](https://github.com/DiAndEn0/tunnel-snoop/commit/00f11579a40fb34aeca98e7f1aa30e76f419cd70))
* **reaper:** implement safe process termination with signal escalation ([f7386de](https://github.com/DiAndEn0/tunnel-snoop/commit/f7386dea923a5189ab7a564a9c9542cde16c11e7))
* **ui:** implement ANSI table and JSON streaming renderers ([fdf6c85](https://github.com/DiAndEn0/tunnel-snoop/commit/fdf6c8532a5e1943d5f8d2958a03db41a9bf669a))


### Bug Fixes

* **procfs:** report unreadable socket tables instead of an empty view ([cace6eb](https://github.com/DiAndEn0/tunnel-snoop/commit/cace6ebfb3e4c2e59dac85c1423d03a4ead1194a))
* **review:** add reaper socket inode verification, protocol-scoped client counts, and fd deduplication ([e182435](https://github.com/DiAndEn0/tunnel-snoop/commit/e182435d4b3d1e4b6ef42c9b52ea3152d2f16309))
* **tests:** resolve go toolchain portably in integration test ([9da8d9d](https://github.com/DiAndEn0/tunnel-snoop/commit/9da8d9dc3438882fe817c9877f60d4e7581ac6f1))
* **tests:** resolve go toolchain portably in integration test ([f2836cf](https://github.com/DiAndEn0/tunnel-snoop/commit/f2836cf357ddeebe235292fee78b3cdc7c64136a))
