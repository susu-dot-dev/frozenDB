# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Python 3.11, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., FastAPI, UIKit, LLVM or NEEDS CLARIFICATION]  
**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]  
**Testing**: [e.g., pytest, XCTest, cargo test or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Linux server, iOS 15+, WASM or NEEDS CLARIFICATION]  
**Project Type**: [single/web/mobile - determines source structure]  
**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]  

**GitHub Repository**: Obtain full repository path using `git remote get-url origin` for import statements in documentation and examples. Example: `github.com/user/repo` from `git@github.com:user/repo.git`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [ ] **Immutability First**: Design preserves append-only immutability with no delete/modify operations
- [ ] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included
- [ ] **Correctness Over Performance**: Any performance optimizations maintain data correctness
- [ ] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations
- [ ] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption
- [ ] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
- [ ] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Document Content Guidelines

### research.md (Phase 0 Output)
**Purpose**: Research findings that resolve technical unknowns from the specification.

**What to Include**:
- Analysis of existing codebase patterns and protocols
- Research on external libraries or technologies
- Decision rationale for technical choices with alternatives considered
- Existing function usage patterns and integration approaches
- Performance and constraint analysis from current architecture

**What to Exclude**:
- Prescriptive code examples for new functions that don't exist yet
- API specifications or method signatures (go in api.md)
- Implementation details that limit implementation flexibility
- Redundant documentation of existing codebase structure

### data-model.md (Phase 1 Output)
**Purpose**: New data entities, validation rules, and state changes introduced by the feature.

**What to Include**:
- New entity definitions and attributes
- Changes to existing data structures or relationships
- New validation rules specific to the feature
- State transitions and flow logic for new operations
- Error condition mappings for new error types
- Data flow relationships between components

**What to Exclude**:
- API specifications, method signatures, or implementation details
- Error handling patterns or usage examples (go in api.md)
- Existing codebase documentation or redundant information
- General project structure or integration details

### contracts/api.md (Phase 1 Output)
**Purpose**: Complete API specification for the feature.

**What to Include**:
- Method signatures and parameter descriptions
- Return values and error conditions
- Success/failure behavior documentation
- Basic usage examples without complex error handling
- Performance characteristics and thread safety information
- Integration notes and compatibility details

**What to Exclude**:
- Complex error handling patterns or implementation guidance
- Internal data model details or state transitions
- Redundant codebase documentation
- Prescriptive implementation details

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
