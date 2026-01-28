---
description: Execute the implementation planning workflow using the plan template to generate design artifacts.
handoffs: 
  - label: Create Tasks
    agent: speckit.tasks
    prompt: Break the plan into tasks
    send: true
  - label: Create Checklist
    agent: speckit.checklist
    prompt: Create a checklist for the following domain...
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

1. **Setup**: Run `.specify/scripts/bash/setup-plan.sh --json` from repo root and parse JSON for FEATURE_SPEC, IMPL_PLAN, SPECS_DIR, BRANCH. For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

2. **Load context**: Read FEATURE_SPEC and `.specify/memory/constitution.md`. Load IMPL_PLAN template (already copied).

3. **Execute plan workflow**: Follow the structure in IMPL_PLAN template to:
   - Fill Technical Context (mark unknowns as "NEEDS CLARIFICATION")
   - Fill Constitution Check section from constitution
   - Evaluate gates (ERROR if violations unjustified)
   - Phase 0: Generate research.md (resolve all NEEDS CLARIFICATION)
   - Phase 1: Generate data-model.md, contracts/
   - Re-evaluate Constitution Check post-design

4. **Stop and report**: Command ends after Phase 2 planning. Report branch, IMPL_PLAN path, and generated artifacts.

## Phases

### Phase 0: Outline & Research

1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:

   ```text
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

### Phase 1: Design & Contracts

**Prerequisites:** `research.md` complete

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable
   - Do not include code algorithms

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

**Output**: data-model.md, /contracts/*

## Document Content Guidelines

Use these when generating Phase 0 and Phase 1 artifacts.

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

## Key rules

- Use absolute paths
- ERROR on gate failures or unresolved clarifications
