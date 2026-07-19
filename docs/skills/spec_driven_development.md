# Skill: General Spec-Driven Development across Projects

This skill guides the AI agent to locate, read, and execute development tasks based on specification files found inside various project subdirectories within the user's Projects directory.

---

## 🎯 Goal
Execute spec-driven development across any software project by scanning for specifications, implementing features, running testing suites, and documenting completion.

---

## 📂 Directories Reference
- **Projects Directory**: `/Users/madhavrajan/Documents/AIProjects/`
- **Project Subfolders**: Each subdirectory inside the Projects folder represents a separate codebase (e.g., `running_cli/`, etc.).
- **Specification Targets**: Look for markdown files inside subfolders such as:
  - `<project_dir>/docs/specs/`
  - `<project_dir>/specs/`
  - `<project_dir>/*spec*.md`
  - `<project_dir>/docs/*spec*.md`

---

## 🛠️ Step-by-Step Execution Workflow

### Phase 1: Project & Specification Discovery
1. List all project subdirectories in the primary Projects folder:
   `/Users/madhavrajan/Documents/AIProjects/`
2. For each project subdirectory, scan for directories named `specs/` or `docs/` and look for files matching `*spec*.md` or `*Spec*.md`.
3. Identify which project has new, active, or unfinished specification tasks.

### Phase 2: Read the Specification
1. Load the specification markdown file using the `view_file` tool.
2. **CRITICAL**: Pass `IsSkillFile: true` when loading the specification file so that its rules, instructions, and target requirements are loaded as active instructions in the agent's context.

### Phase 3: Project Context Alignment
1. Determine the language and technology stack of the target project (e.g., Python, Node.js, Go, Rust) by checking configuration files:
   - Python: `setup.py`, `requirements.txt`, `pyproject.toml`
   - Node.js: `package.json`
   - Rust: `Cargo.toml`
2. Set all subsequent tool executions (commands, file operations) to run relative to the target project directory.

### Phase 4: Implementation
1. Create a detailed implementation plan mapping the requirements in the spec to code modifications.
2. If in planning mode, present the implementation plan to the user for approval.
3. Edit, create, or delete files inside the project's source tree to implement the features.

### Phase 5: Verification & Testing
1. Run the project's test suite:
   - Python: `pytest`
   - Node.js: `npm test`
   - Rust: `cargo test`
   - Go: `go test ./...`
2. Verify all tests pass successfully. If any tests fail, debug the implementation and re-run verification.

### Phase 6: Sync & Commit
1. Check off completed items inside the specification markdown file (e.g., ticking checkboxes `[x]`).
2. Write a completion summary or walkthrough document.
3. Add the files and commit changes to the project's local Git repository.
