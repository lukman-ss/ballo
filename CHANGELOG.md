# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.1] - 2026-11-05

### Fixed
- Error handling audit: internal errors from handlers/middleware never leak to clients. Full details logged server-side; clients receive generic "internal server error" response.
- `writeInternalError` consistently logs error via `log.Printf` before sending safe 500 response.
- `RecoveryMiddleware` logs panic details server-side; clients receive generic 500 response.

### Added
- Test middleware error propagation in `TestEngineErrorDoesNotLeakDetails`.
- Test `TestEngineErrorDoesNotLeakDetails` verifies internal errors are logged server-side and clients receive safe generic 500 response for regular routes, 404, and 405 handlers.
- Regression test for `JSON` response encoding error handling (`TestEngineJSONMarshalError`).
- Concurrency regression test for concurrent route registration and request serving (`TestEngineConcurrentRegisterAndServe`).
- Fallback handling and regression test for handlers returning `nil` response (`TestEngineNilResponseHandling`).

### Changed
- Clarified thread-safety documentation in `README.md` to accurately document internal RWMutex route protection while recommending startup-time registration.
