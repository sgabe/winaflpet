# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Onboarding guidance and refresh account pages.
- Selected file names and sizes on import and seed-upload forms.
- Keyboard navigation and a single tab stop to the fuzzer instance selector.
- Plot refresh controls and display refresh status and failures.
- Filtering by recorded function name to the crashes page.
- Setup guidance and saved connection context to agent forms.
- Responsive section sidebar to job configuration forms.
- Clear filters actions and page-specific no-match messages to agent, job and crash lists.
- Read-only saved configuration dialog to job cards.
- Confirmation dialogs before deleting agents, jobs, crash records or all crashes.
- Recovery guidance for missing job statistics and plot images, and per-image load failure messages.
- Validation summary with links to invalid fields on the job create and edit forms.
- File drop areas on the import and input-upload forms.
- Render instance plot data as SVG charts with legends and values tables, retaining generated image fallback.

### Changed
- Upgrade to Bootstrap 6.0.0-alpha.1 and jQuery 4.0.0.
- Redesign shared layouts and styles for Bootstrap 6 and jQuery 4.
- Redesign navigation and shared card-list controls.
- Redesign configuration forms and add section navigation.
- Redesign job cards and add status summaries and filters.
- Redesign instance statistics and plot layouts.
- Redesign agent and crash cards and add list filters.
- Improve import, seed-upload and crash-edit guidance and context.
- Merge card expand and collapse actions into one button beside the list filters.
- Refine the home onboarding layout and add a preparation checklist.
- Replace navigation menus with direct links, add account menu and skip-to-content link.
- Group settings into SMTP server and sender sections and add a validation summary.
- Move search from the navigation bar into the agent, job and crash list toolbars.
- Group crash-edit fields into Analysis, Source and Notes sections with field-specific guidance.
- Show job statistics as one divided panel per instance.
- Refine crash details, instance statistics, plot and form navigation, and sign-in feedback.
- Regroup job configuration sections and refine shared card controls, descriptions and switches.
- Refine onboarding, file selection and upload context, instance statistics and crash/profile editing feedback.
- Inline agent connection results and improve configuration summaries, dialog navigation, form context and action icon colors.
- Unify page headings and empty states, improve list action placement, and clarify required, optional and technical fields.
- Show per-page counts on agent, job and crash status filters and add a crash verification summary.
- Align shared body and form typography and link colours with the interface design, and dock form actions in a sticky panel.
- Unify list filters, search and folding in one responsive toolbar using shared HTML templates.
- Check saved agent connections inline without disabling sibling actions or losing unsaved edits.
- Show saved agent names and email alert state, and confirm input replacement before uploading.

### Fixed
- Prefill profile fields, so saving no longer clears the alert email, and explain password requirements.
- Submit the port from the agent edit form so saving keeps the configured port.
- Number fuzzer instance selector tiles by instance ID so sparse or non-sequential statistics are no longer mislabeled.

## [2.1.0] - 2026-09-12
### Added
- Settings page for SMTP and the email alert interval.

## [2.0.0] - 2026-09-09
### Added
- Crash downloads, HTML reports and a separate job Export action.

## [1.2.0] - 2026-09-08
### Added
- Seed uploads for idle jobs, with file replacement.

## [1.1.0] - 2026-09-07
### Changed
- Compact live job indicators, contextual actions and improved navigation.

## [1.0.0] - 2026-09-06
### Fixed
- Reliable job startup, retries, cancellation, shutdown and restart tracking.

## [0.8.0] - 2026-04-09
### Added
- Support for PageHeap‑enabled .ph harness during crash verification.
- Support for AFL postprocessor library.

### Changed
- Use the label "unavailable" for false positives.
- Make job startup to be async with per-instance status tracking.
- Update Docker base images to Go v1.26.1 and Alpine v3.23.

### Fixed
- Zombie child processes during crash verification using Windows job objects.
- Crash auto-increment counter not resetting after deletion.

## [0.7.0] - 2025-11-30
### Added
- Removal of output directory upon resuming if autoresume is disabled.
- Initial support for TinyInst.
- Active path indicator to navigation.

### Changed
- Copy hashed samples to a unified directory when collecting crashes.
- Use function address as function name until verification.
- Update dependencies.
- Upgrade to Bootstrap 5.
- Migrate card columns to Masonry.

### Fixed
- Agent build command to use correct working directory.

## [0.6.1] - 2025-04-20
### Changed
- Tidied go.mod.
- Update Docker base images.

## [0.6.0] - 2025-04-20
### Added
- Support for custom bug bucket feature.
- Support for 40 parallel fuzzer instances.
- Variable mode to allow for varying parameters.
- Sequential mode to allow execution of numbered target applications.

### Changed
- Infer binary directory from target architecture.
- Default risk set to none.

### Fixed
- Remove usage of deprecated io/ioutil package.

## [0.5.0] - 2022-06-08
### Added
- Ability to export/import jobs.
- Periodic email alerts of new crashes.
- Button to purge crash records from database.

### Changed
- Update Bootstrap icons to v1.8.3.
- Update button style for jobs.
- Update Docker base images.

## [0.4.0] - 2022-02-05
### Changed
- Type of fID changed from string to integer.

## [0.3.1] - 2022-02-02
### Changed
- More specific regex pattern for crash detection.

### Fixed
- Replace invalid execs_per_sec value for unmarshaling.
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

[Unreleased]: https://github.com/sgabe/winaflpet/compare/v2.1.0...HEAD
[2.1.0]: https://github.com/sgabe/winaflpet/releases/tag/v2.1.0
[2.0.0]: https://github.com/sgabe/winaflpet/releases/tag/v2.0.0
[1.2.0]: https://github.com/sgabe/winaflpet/releases/tag/v1.2.0
[1.1.0]: https://github.com/sgabe/winaflpet/releases/tag/v1.1.0
[1.0.0]: https://github.com/sgabe/winaflpet/releases/tag/v1.0.0
[0.8.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.8.0
[0.7.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.7.0
[0.6.1]: https://github.com/sgabe/winaflpet/releases/tag/v0.6.1
[0.6.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.6.0
[0.5.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.5.0
[0.4.0]: https://github.com/sgabe/winaflpet/releases/tag/v0.4.0
[0.3.1]: https://github.com/sgabe/winaflpet/releases/tag/v0.3.1
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