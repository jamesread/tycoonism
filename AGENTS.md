# Coding style

- Use comments to explain edge cases or design decisions only. Don't use them to describe what the code is doing.
- Use the languages' standard style guides for formatting and naming conventions.
- Make function and method names descriptive of their behavior.
- Cyclomic complexity should be kept to 4 or less. 

# TDD 

- Write tests for all new features and bug fixes.
- If a test breaks due to a code change, prompt the user to decide whether to update the test or fix the code.

# Project layout

- Make and makefile targets should be used to build, test, lint, and run the project. Make files should call other Makefiles for subdifectories like `make -wC frontend`.
- Running `make` without arguments should build the project and all subdirectories.
- Backend code should be in the service/ directory.
- Frontend code (js, vue, vite, etc.) should be in the frontend/ directory.
- Integration tests should be in the inegration-tests/ directory.
- Protocol code should use connectrpc amd buf, and should be in the protocol/ directory.
- Documentation should be in the docs/ directory, and use mkdocs.

# repo health

- The command line tool `repohealth` reports issues with the repo. 

# Frontend

- Use a component-based architecture with Vue + Vite.
- For routing, use Vue Router.
- Use the npm library `picocrank` that provides some common components and utilities.
- Use as few CSS rules as possible in yhe components, the library femtocrank which is a dependency of picocrank provides most of the needed styling.

# Backend

- Use Go modules for dependency management.
- Use the standard library as much as possible.
- Use the `logrus` library for logging.
- Use the `koanf` library for configuration management.
- Use the `jamesread/golure` library for utility functions.
- Use the `jamesread/httpauthshim` library for HTTP authentication.
- Use the `stretchr/testify` library for testing.
- Use the `connectrpc` library for gRPC services.

# Protocol

- Use Protocol Buffers version 3 syntax.
- Use `connectrpc` for gRPC services.
- Use `buf` for managing Protocol Buffers files and generating code.
- Follow best practices for designing Protocol Buffers messages and services.
- Keep Protocol Buffers files organized and modular.
- Document Protocol Buffers messages and services with comments.

# Testing

- Use unit tests for individual functions and methods.
- Use integration tests for testing interactions between components.
- Use end-to-end tests for testing the entire system.
- Use mocking and stubbing to isolate components during testing.
- integration tests should be implemented using mocha and selenium-webdriver.
- integration tests should be located in the integration-tests/tests/ directory, and include the JS tests, and comfig.yaml for the backend.
- integration tests should start and stop the backend service and set the -configdir arg as needed.
