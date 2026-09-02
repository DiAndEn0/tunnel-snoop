# Changelog

## [1.3.0](https://github.com/DiAndEn0/tunnel-snoop/compare/v1.2.0...v1.3.0) (2026-09-02)


### Features

* add -fail-on-exposed audit gate exit code ([17b6865](https://github.com/DiAndEn0/tunnel-snoop/commit/17b68658c4079b2042e6c484c95f7b418a2f18b5))
* add result filters for port, process, exposure and idle time ([22bebd5](https://github.com/DiAndEn0/tunnel-snoop/commit/22bebd5d6b6850d0cbaee89cb4b18dda69837be2))
* classify tunnel exposure beyond literal wildcards ([8132734](https://github.com/DiAndEn0/tunnel-snoop/commit/8132734295d4ef1d22ee6e941ce7785f86f4ad3d))
* **cmd,monitor:** add result filters and fail-on-exposed CI audit gate ([f550b94](https://github.com/DiAndEn0/tunnel-snoop/commit/f550b94c7d2d73b65bbcf85fe3814bb372cf3fc5))
* **model,ui:** classify exposure tiers and derive idle activity from socket counters ([38a9ce0](https://github.com/DiAndEn0/tunnel-snoop/commit/38a9ce0c224c82be7c3213511ef3e46c91f3ac67))
* **monitor:** sync IsExposed with 4-tier exposure model ([aad8dcd](https://github.com/DiAndEn0/tunnel-snoop/commit/aad8dcd7cb9062efc5d769b91e82d3e92acb6b48))
* **monitor:** sync IsExposed with 4-tier exposure model ([202196e](https://github.com/DiAndEn0/tunnel-snoop/commit/202196e2ff71251ed4dbb9cc77d1b4e1c6729080))


### Bug Fixes

* **cmd:** exit with code 2 on scan failure to fail closed ([828cc56](https://github.com/DiAndEn0/tunnel-snoop/commit/828cc56518ff0522142d95c8da01bcea4450444e))
* derive tunnel activity from socket syscall counters ([23a2dcc](https://github.com/DiAndEn0/tunnel-snoop/commit/23a2dcc036e5a324975dc58e52b640d4b45ab3ad))
* **monitor:** ignore unreadable procfs io errors to prevent false idle resets ([fea7a70](https://github.com/DiAndEn0/tunnel-snoop/commit/fea7a7021825609cc97732f5935bd81fe5e74f29))
* **monitor:** key engine state by PID and socket inode composite ([36461ed](https://github.com/DiAndEn0/tunnel-snoop/commit/36461ede225bf86ed8d257bcc92de31d4894f8e6))
* **monitor:** key engine state by PID and socket inode composite ([5350e37](https://github.com/DiAndEn0/tunnel-snoop/commit/5350e37e724f43b2b3fe51bdb65b87fd2612dbcd))

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
