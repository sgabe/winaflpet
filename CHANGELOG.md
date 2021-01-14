# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
## Fixed
- Fix regex pattern used to extract crash location from BugId output.
- Return early on invalid number of PIDs provided for checking a job.
- Missing instrumentation option to set target_offset.

### Changed
- Use smaller font size in footer for mobile screens.
- Allow crash analysis when page heap is not enabled.
- Allow running up to 8 fuzzer instances simultaneously.
- Sort crashes in descending order by internal ID.
- Update crash file paths when resuming aborted jobs.
- Increase request timeout to avoid errors when starting jobs.
- Increase database query limit to display more crashes.
- Refactor crash template.

### Removed
- Unused id attributes in the HTML templates.

## [0.0.2] - 2020-12-14
### Removed
- Unnecessary debug print.

## [0.0.1] - 2020-12-14
### Added
- Initial commit.

### Changed
- Redirect logged in users to jobs when page was not found.
- Improved template renderer to use layouts.

[Unreleased]: https://github.com/sgabe/winaflpet/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.2
[0.0.1]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.1