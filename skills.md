# Project Engineering Rules

These rules apply to **every change** in this repository. Their purpose is to keep the app production-ready at all times: every piece of code is tested, readable, secure, and observable. A change that breaks any **MUST** rule is not done.

- **MUST**: a hard requirement. CI enforces it and review blocks the change.
- **SHOULD**: the default. Any exception needs a written reason in the PR.

---

## 1. Definition of Done

A change is complete only when all of these are true:

1. [ ] Every new or changed line is covered by tests (see §2). Coverage stays at **100%**.
2. [ ] Lint, format check, type check, and tests all pass locally and in CI.
3. [ ] The change introduces no code smells from §3 (or the PR explains each one).
4. [ ] Errors are handled, logged, and turned into clear user-facing messages (§5).
5. [ ] It contains no secrets, no hard-coded environment values, no `console.log`/`print` debugging, and no commented-out code.
6. [ ] Public behaviour changes are reflected in docs (README, API docs, CHANGELOG).
7. [ ] The PR is small and focused (aim for under 400 changed lines) and does one thing.

---

## 2. Testing — 100% Coverage

### 2.1 Coverage thresholds (MUST, enforced in CI)

| Metric     | Threshold |
|------------|-----------|
| Lines      | 100%      |
| Statements | 100%      |
| Branches   | 100%      |
| Functions  | 100%      |

- CI **fails** if any metric drops below 100%. Nobody lowers a threshold to get a build to pass.
- Coverage exclusions are allowed only for:
  - generated code (ORM clients, protobuf, OpenAPI clients),
  - type-only files (interfaces, type declarations),
  - the entry-point bootstrap (`main`/`index` that only wires things together),
  - config files.
- Any inline ignore (`/* istanbul ignore next */`, `# pragma: no cover`, `c8 ignore`) **MUST** have a comment saying *why* the code cannot be tested. Review questions every one of them.
- 100% coverage is the minimum, not the goal. Every test **MUST** assert behaviour. A test that runs code without checking the result doesn't count as a test.

### 2.2 Test types

| Type        | Scope                                           | Speed    | Required for                             |
|-------------|-------------------------------------------------|----------|------------------------------------------|
| Unit        | One function/class, dependencies mocked         | < 50 ms  | All business logic, utilities, reducers  |
| Integration | Module + real DB/queue/HTTP layer (containers)  | < 2 s    | Repositories, API routes, external clients |
| End-to-end  | Full user flow through the running app          | seconds  | Critical paths: auth, core feature, payment |
| Contract    | API request/response shapes                     | fast     | Every public API endpoint                |

### 2.3 Test rules

- **MUST** follow Arrange–Act–Assert. Put one behaviour in each test.
- **MUST** name tests after behaviour: `returns 404 when video does not exist`, not `test getVideo 2`.
- **MUST** be deterministic. Tests never depend on real time, random values, network, test order, or shared mutable state. Inject clocks, random generators, and ID generators.
- **MUST** test every branch: the happy path, each error path, edge cases (empty, null/undefined, zero, negative, max length, unicode, duplicates), and boundaries (off-by-one).
- **MUST** test failure modes of external calls: timeout, 4xx, 5xx, malformed response, and retry exhaustion.
- **MUST NOT** use `sleep`/fixed waits. Use fake timers, or poll for a condition with a timeout.
- **MUST NOT** leave `.only`, `.skip`, `xit`, `fdescribe`, or `@pytest.mark.skip` in committed code, unless the skip has a linked ticket.
- **SHOULD** mock only at system boundaries (HTTP, DB, file system, clock). Don't mock the unit under test or its private helpers.
- **SHOULD** use factories/builders for test data instead of big inline object literals.
- **SHOULD** write the test first when fixing a bug. It must fail before the fix and pass after it.
- **SHOULD** run mutation testing (Stryker / mutmut) on core domain modules. Target a mutation score of at least 80%.
- Test files live next to the code (`foo.ts` → `foo.test.ts`) or in a mirrored `tests/` tree. Pick one layout and use it everywhere.

---

## 3. Code Smells — Rules & Limits

Linters enforce these limits wherever they can. Code over a limit must be refactored. Don't suppress the warning.

### 3.1 Size & complexity limits (MUST)

| Rule                                  | Limit |
|---------------------------------------|-------|
| Function/method length                | ≤ 40 lines |
| File length                           | ≤ 300 lines |
| Cyclomatic complexity per function    | ≤ 10 |
| Cognitive complexity per function     | ≤ 15 |
| Nesting depth                         | ≤ 3 levels |
| Parameters per function               | ≤ 4 (use an options object beyond that) |
| Class public methods                  | ≤ 10 |
| Line length                           | ≤ 120 chars |

### 3.2 Smells that MUST NOT appear

| Smell | Rule / Fix |
|-------|------------|
| **Duplicated code** | Don't copy a block of 5+ lines. Extract a function or module. Keep the duplication check (jscpd / SonarQube) under 3%. |
| **Long function / God class** | Split by responsibility. Each unit should have one reason to change (SRP). |
| **Magic numbers & strings** | Use named constants or enums. `0`, `1`, `-1`, and `''` in obvious contexts are fine. |
| **Deep nesting / arrow code** | Use guard clauses and early returns. |
| **Boolean flag parameters** | `render(true)` hides meaning. Split into two functions or pass a named option. |
| **Primitive obsession** | Wrap domain concepts (`VideoId`, `Email`, `Money`) in types or value objects. |
| **Feature envy** | If a function mostly uses another object's data, move it to that object. |
| **Shotgun surgery** | If one change forces edits in many files, the logic is scattered. Consolidate it. |
| **Dead code** | Delete unused functions, variables, imports, exports, and feature flags. Git keeps the history. |
| **Commented-out code** | Delete it. |
| **Speculative generality** | Don't add abstractions, params, or hooks "for later" (YAGNI). |
| **Swallowed errors** | An empty `catch`, or `catch` that only logs and continues silently, is forbidden (see §5). |
| **Mutable global state** | No module-level mutable singletons. Use dependency injection. |
| **Mixed abstraction levels** | A function either orchestrates or does low-level work, not both. |
| **Long parameter lists / data clumps** | Values that travel together become one type. |
| **Inconsistent naming** | Follow §3.3. |
| **`any` / untyped escapes** | `any`, `# type: ignore`, and `@ts-ignore` are forbidden without a justification comment. Prefer `unknown` + narrowing. |
| **TODO/FIXME without owner** | Write `TODO(#123): ...` with a ticket reference, or don't write it. |

### 3.3 Naming

- Names say *what* and *why*, not *how*: `activeSubscribers`, not `list2` or `data`.
- Functions are verbs (`fetchVideo`, `calculateWatchTime`). Booleans are questions (`isPublished`, `hasAccess`, `canEdit`).
- No abbreviations except widely known ones (`id`, `url`, `http`, `db`).
- Casing:
  - files: `kebab-case`
  - types/classes: `PascalCase`
  - variables/functions: `camelCase` (or `snake_case` in Python)
  - constants: `UPPER_SNAKE_CASE`

### 3.4 Comments

- Code explains *what*. Comments explain *why*: business rules, workarounds, non-obvious trade-offs.
- Public APIs and exported functions get doc comments (JSDoc / docstrings).
- Don't write comments that just restate the code.

---

## 4. Architecture & Design

- **MUST** layer the code and point dependencies inward: `routes/controllers → services (business logic) → repositories/clients (I/O)`. Business logic never imports framework or HTTP objects.
- **MUST** use dependency injection for anything with I/O (DB, HTTP, cache, clock, logger) so it can be tested.
- **MUST** validate all external input at the boundary (request bodies, query params, env vars, third-party API responses) with a schema library (Zod / Pydantic / Joi). Trust the data only after that.
- **SHOULD** prefer pure functions and immutability. Isolate side effects.
- **SHOULD** prefer composition over inheritance.
- **SHOULD** keep modules organised by feature (`/videos`, `/channels`, `/auth`) rather than by technical type.
- Circular dependencies are forbidden (enforced by `madge` / `import-linter`).

---

## 5. Error Handling

- **MUST** use typed/custom error classes (`NotFoundError`, `ValidationError`, `ExternalServiceError`). Never throw strings.
- **MUST** handle errors at the right level: rethrow with context (`cause`) or convert to a domain error. Never swallow them.
- **MUST** have a single global error handler that maps errors to HTTP responses. Unknown errors become a generic 500. Responses never include stack traces or internal details.
- **MUST** put a timeout on every external call. Retries use exponential backoff + jitter and apply only to idempotent operations.
- **MUST** handle unhandled promise rejections / uncaught exceptions by logging them and shutting down gracefully.
- **SHOULD** use a circuit breaker for flaky third-party APIs (e.g. YouTube Data API quota / 5xx).

---

## 6. Security (MUST)

- No secrets in code, tests, logs, or git history. Load them from environment / secret manager. Commit only `.env.example`, never `.env`.
- Validate and sanitise all input. Use parameterised queries only, never string-built SQL.
- Encode output to prevent XSS. Set security headers (CSP, HSTS, X-Content-Type-Options, frame-ancestors).
- AuthN/AuthZ is checked on the server for every protected route. Never rely on the client alone.
- Rate-limit public and auth endpoints.
- Hash passwords with argon2/bcrypt, never a fast hash.
- Configure CORS explicitly. No `*` with credentials.
- Dependencies:
  - Pin versions with a lockfile and commit it.
  - Run `npm audit` / `pip-audit` / Dependabot in CI.
  - Block merges on high/critical vulnerabilities.
- Run secret scanning (gitleaks) in pre-commit and CI.
- Follow OWASP Top 10 and least privilege for DB users, API keys, and cloud IAM.

---

## 7. Observability

- **MUST** use structured (JSON) logging through a shared logger with levels (`debug`, `info`, `warn`, `error`). No `console.log`/`print` in app code.
- **MUST** attach a request/correlation ID to every log line for a request.
- **MUST NOT** log secrets, tokens, passwords, or PII. Redact them in the logger config.
- **MUST** expose `/health` (liveness) and `/ready` (readiness: checks DB and dependencies).
- **SHOULD** emit metrics (request rate, latency p50/p95/p99, error rate) and send errors to a tracker (e.g. Sentry).

---

## 8. Performance & Reliability

- **MUST** paginate every list endpoint. No unbounded queries.
- **MUST** avoid N+1 queries. Add indexes for filtered/sorted columns.
- **SHOULD** cache expensive or rate-limited external calls (e.g. YouTube API) with an explicit TTL and invalidation strategy.
- **SHOULD** run long work (transcoding, bulk imports) in background jobs, not in the request cycle.
- **MUST** support graceful shutdown: stop accepting requests, drain in-flight work, close connections.

---

## 9. Configuration & Environments

- All config comes from environment variables, validated at startup. The app fails fast on missing or invalid config.
- The same build artifact runs in dev, staging, and prod. Only config differs.
- Feature flags have an owner and a removal date.
- DB schema changes go through versioned migrations. Migrations are backward compatible (expand → migrate → contract) and reversible.

---

## 10. Git & Code Review

- **Branches**: `feat/…`, `fix/…`, `chore/…`, `refactor/…`, `test/…`, `docs/…`. Never commit directly to `main`.
- **Commits**: follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat: add video search`). Each commit is atomic and builds on its own.
- **PRs**:
  - at least 1 approving review
  - all CI checks green
  - the description covers what, why, and how it was tested
  - screenshots for UI changes
- `main` is protected: no force-push, linear history, required status checks.

---

## 11. CI/CD Pipeline (MUST)

Every push and PR runs these steps, in this order. Any failure blocks the merge:

1. Install with a frozen lockfile
2. Format check
3. Lint (including complexity/smell rules from §3)
4. Type check
5. Unit + integration tests with the **100% coverage gate**
6. Duplicate-code check + static analysis (SonarQube/SonarCloud quality gate: 0 new bugs, 0 new vulnerabilities, 0 new code smells, 100% coverage on new code)
7. Security: dependency audit + secret scan
8. Build the production artifact / Docker image
9. E2E tests against the built artifact
10. Deploy to staging → smoke tests → manual/automatic promotion to production

A pre-commit hook (husky + lint-staged / pre-commit) runs format, lint, and related tests on staged files.

---

## 12. Frontend-Specific (if applicable)

- Keep components small (≤ 150 lines). Split presentational and container/logic code.
- No business logic in components. Put it in hooks/services that have their own unit tests.
- Accessibility **MUST** pass (axe checks in tests): semantic HTML, labels, keyboard navigation, colour contrast.
- Test with Testing Library by user behaviour (roles, text), not implementation details.
- Handle loading, empty, and error states for every async view.
- Performance budget: Lighthouse score ≥ 90, lazy-load routes and heavy media, set width/height on images and embeds.

---

## 13. Tooling Reference

Once the stack is chosen, fill in the matching tools and delete the rest.

| Concern         | TypeScript / Node                                      | Python                                   |
|-----------------|--------------------------------------------------------|------------------------------------------|
| Test runner     | Vitest / Jest                                          | pytest                                   |
| Coverage        | `@vitest/coverage-v8` / `c8` with `thresholds: 100`    | `pytest-cov --cov-fail-under=100 --cov-branch` |
| E2E             | Playwright                                             | Playwright / pytest-playwright           |
| Lint + smells   | ESLint + `eslint-plugin-sonarjs` + `complexity`, `max-lines`, `max-depth`, `max-params` rules | Ruff (with `C90`, `PL`, `SIM`, `ERA` rules) |
| Format          | Prettier                                               | Ruff format / Black                      |
| Types           | `tsc --noEmit` with `strict: true`                     | mypy `--strict` / pyright                |
| Duplication     | jscpd                                                  | jscpd / pylint `duplicate-code`          |
| Mutation        | Stryker                                                | mutmut                                   |
| Validation      | Zod                                                    | Pydantic                                 |
| Security        | `npm audit`, gitleaks, Dependabot                      | `pip-audit`, bandit, gitleaks            |
| Static analysis | SonarCloud                                             | SonarCloud                               |

### Example coverage gate (Vitest)

```ts
// vitest.config.ts
export default defineConfig({
  test: {
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov', 'html'],
      thresholds: { lines: 100, branches: 100, functions: 100, statements: 100 },
      exclude: ['**/*.d.ts', '**/*.config.*', 'src/main.ts', 'src/generated/**'],
    },
  },
});
```

### Example coverage gate (pytest)

```toml
# pyproject.toml
[tool.pytest.ini_options]
addopts = "--cov=src --cov-branch --cov-report=term-missing --cov-fail-under=100"
```

---

## 14. Instructions for AI Assistants

When generating or modifying code in this repo:

1. Write or update tests **in the same change** as the code, reaching 100% line and branch coverage for the touched code.
2. Run lint, type check, and the full test suite with coverage before saying the work is done. Report any failure honestly.
3. Stay within the limits in §3.1. If a function would exceed them, refactor it instead of suppressing the rule.
4. Never add coverage-ignore pragmas, lint suppressions, or `any` without a justification comment.
5. Never commit secrets, debug logs, or commented-out code.
6. Follow the existing patterns and naming in the codebase before introducing new ones.
