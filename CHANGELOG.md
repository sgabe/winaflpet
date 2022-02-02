# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Fixed
- Ignore unspecified or zero value for memory limit.

## [0.3.0] - 2021-12-18
### Added
- Support for using WinAFL as a pre-configured tool for DynamoRIO.
- Support for afl-fuzz environment variables and autoresume.

### Changed
- Update fuzzing job templates.

### Fixed
- Purging of crash recrods from the database.
- Handle error when Python path is incorrect.
- Use the randomly generated shared memory name.

## [0.2.0] - 2021-11-02
### Added
- Support for additional afl-fuzz options.

### Fixed
- Minor template issues.
- Check if process was found before killing it.

## [0.1.0] - 2021-10-10
### Added
- Support for sample delivery via shared memory.

### Changed
- Set coverage type values in job creation form.
- Update build constraints.
- Migrate structable from Masterminds to sgabe.

### Fixed
- Increase the width of the search box.
- Overflowing card header.
- Hide pager for single page.

## [0.0.7] - 2021-10-02
### Changed
- Gin version bumped to fix CVE-2020-28483.

## [0.0.6] - 2021-04-10
### Added
- Common template functions provided by Sprig.
- Pagination to navigate through the crashes.
- Search box to filter cards.

## [0.0.5] - 2021-04-01
### Fixed
- Prevent erroneous user profile update.
- jQuery AJAX used to download binary crash samples.

## [0.0.4] - 2021-03-22
### Added
- Flag to enable debug mode and non-secure session cookie.
- Show bitmap coverage information among overall results.

### Fixed
- Show target method when offset is not specified.
- Binding to command line host and port flags.
- Anonymous function as parameter to setTimeout().

### Changed
- Allow running up to 20 fuzzer instances simultaneously.
- Reload the page after successfully starting a job.
- Use goroutine to read process's standard output.
- More specific regex pattern to find crash samples.

## [0.0.3] - 2021-01-24
### Added
- Support additional command line arguments for target application.
- Support for absolute paths for input and output.

### Fixed
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
- Improve regex pattern to detect system errors.

### Removed
- Unused id attributes in the HTML templates.
- Unused CSS stylesheet.

## [0.0.2] - 2020-12-14
### Removed
- Unnecessary debug print.

## [0.0.1] - 2020-12-14
### Added
- Initial commit.

### Changed
- Redirect logged in users to jobs when page was not found.
- Improved template renderer to use layouts.

[Unreleased]: https://github.com/sgabe/winaflpet/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.3.0
[0.2.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.2.0
[0.1.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.1.0
[0.0.7]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.7
[0.0.6]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.6
[0.0.5]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.5
[0.0.4]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.4
[0.0.3]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.3
[0.0.2]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.2
[0.0.1]: https://github.com/sgabe/winaflpet/releases/tag/v0.0.1