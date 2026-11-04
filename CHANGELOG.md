# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Regression test for `JSON` response encoding error handling (`TestEngineJSONMarshalError`).
- Concurrency regression test for concurrent route registration and request serving (`TestEngineConcurrentRegisterAndServe`).
- Fallback handling and regression test for handlers returning `nil` response (`TestEngineNilResponseHandling`).

### Fixed
- Fixed `JSON(status, data)` silently returning `nil` when encoding fails; it now returns an HTTP 500 error response.
- Prevented race conditions between route registration and request serving by holding read lock across route tree matching in `ServeHTTP`.
- Handled `nil` responses returned by custom route, 404, or 405 handlers by writing a standardized fallback error response.

### Changed
- Clarified thread-safety documentation in `README.md` to accurately document internal RWMutex route protection while recommending startup-time registration.
