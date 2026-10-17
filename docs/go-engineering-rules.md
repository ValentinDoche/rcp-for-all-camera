# Rules for a scalable, maintainable and secure Go application

A synthesis of two guides, adapted to Go:

- **[DDH]** [Domain-Driven Hexagon](https://github.com/Sairyss/domain-driven-hexagon) (Sairyss):
  architecture (DDD, hexagonal / ports and adapters, Clean Architecture, Secure by Design). Its
  examples are in TypeScript/NestJS; they are rewritten here with Go idioms.
- **[LGWT]** [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests) (Chris James):
  TDD, test-driven design, Go idioms, acceptance tests, fakes and contracts.
- **[Added]**: a Go addition that comes from neither guide, marked as such.

Every rule has an id (`A3`, `T7`...) so it can be quoted in a code review.

> **Rule zero.** These are recommendations, not dogma [DDH]. Before applying a pattern, check that
> its benefit is worth the complexity it adds (YAGNI, KISS). To break a rule, first understand it
> and know which trade-off you make [LGWT, *IO and sorting*]. But "it is easier to refactor
> over-design than no design at all" [DDH].

---

## Contents

1. [General principles](#1-general-principles)
2. [Architecture: layers and dependency direction](#2-architecture-layers-and-dependency-direction)
3. [Modules and Go project layout](#3-modules-and-go-project-layout)
4. [Domain layer](#4-domain-layer)
5. [Application layer (use cases)](#5-application-layer-use-cases)
6. [Inbound adapters (HTTP, CLI, GUI, messages)](#6-inbound-adapters-http-cli-gui-messages)
7. [Infrastructure (outbound adapters)](#7-infrastructure-outbound-adapters)
8. [Errors](#8-errors)
9. [Security (Secure by Design)](#9-security-secure-by-design)
10. [Interfaces, dependency injection, public API](#10-interfaces-dependency-injection-public-api)
11. [Concurrency, `context`, time](#11-concurrency-context-time)
12. [TDD process](#12-tdd-process)
13. [Test strategy: levels and test doubles](#13-test-strategy-levels-and-test-doubles)
14. [Testing techniques in Go](#14-testing-techniques-in-go)
15. [Refactoring](#15-refactoring)
16. [Performance](#16-performance)
17. [Tooling and discipline](#17-tooling-and-discipline)
18. [Enforcing the architecture](#18-enforcing-the-architecture)
19. [When to go lighter (small projects)](#19-when-to-go-lighter-small-projects)
20. [How this repository applies them](#20-how-this-repository-applies-them)
21. [Review checklist](#21-review-checklist)

---

## 1. General principles

- **G1. Software must be able to change.** A system in use has to evolve or it becomes useless
  (Lehman's law of continuing change). Its complexity grows unless work is done to reduce it (law
  of increasing complexity). Do not ship feature after feature without refactoring [LGWT, *Why*].
- **G2. Do not try to predict the future.** No "perfectly extensible" system designed up front:
  keep the code malleable with tests and constant refactoring [LGWT, *Why*].
- **G3. Small composable pieces.** Modules, functions, types and services stay small enough to be
  rewritten in a few days [DDH, *Modules*].
- **G4. Separate essential complexity (the business) from accidental complexity (network, disk,
  protocols, frameworks).** That is the goal of all the architecture below [LGWT, *Scaling
  acceptance tests*].
- **G5. One responsibility per component** (single responsibility principle), where it makes
  sense: repositories for data access, entities for business logic, and so on [DDH, *Diagram*].
- **G6. Simple is not easy.** Adding a field to a test table or copy-pasting a loop is easy, not
  necessarily simple. Do not confuse unfamiliar with complex either [LGWT, *Anti-patterns*,
  *Revisiting arrays with generics*].
- **G7. Move in thin vertical slices.** Make a small feature work end to end, covered by tests,
  then iterate. Avoid the "big bang" and unvalidated bottom-up design [LGWT, *Mocking*, *Scaling
  acceptance tests*, *Reading files*].
- **G8. "Make it work, make it right, make it fast"**, in that order. No premature optimisation
  [LGWT, *Concurrency*].
- **G9. Names matter.** Use the business language (*ubiquitous language*) [DDH] and the
  established names of patterns (`Map`, `Reduce`, `Find`, adapter, decorator) rather than invented
  ones [LGWT, *Revisiting arrays with generics*].

## 2. Architecture: layers and dependency direction

Flow of a request [DDH, *Diagram*]:

```
Inbound adapter (HTTP / CLI / GUI / message)
   │  raw DTO → validation → Command / Query
   ▼
Application layer (use case / service)
   │  orchestrates; calls the domain; uses the ports (interfaces)
   ▼
Domain layer (entities, aggregates, value objects, domain services)
   ▲
   │  implements the ports
Infrastructure (repositories, API clients, drivers, broker, logger...)
```

- **A1. Dependencies point inwards.** Outer layers may import inner layers, never the reverse.
  The core (domain and application) imports no framework, no driver and no `net/http`
  [DDH, *Ports*].
- **A2. The core reaches the outside world through ports.** In Go, a port is an **interface
  declared on the side that uses it** (the core). The implementation lives in the infrastructure
  and is injected [DDH, *Ports*] [LGWT, *Dependency injection*].
- **A3. Design a port for the domain's need,** not as a copy of the tool's API: a `CameraStore`
  port, not an interface that copies the SQL client [DDH, *Ports*].
- **A4. Do not create needless abstractions.** Depending on a concrete type is often fine. An
  interface is justified by several implementations, an external dependency to replace in tests,
  or a real need for decoupling [DDH, *Ports*, *Repositories*] [LGWT, *Anti-patterns*].
- **A5. The core does not know who calls it.** It must serve an HTTP handler, a CLI, a GUI or a
  message consumer without change [DDH, *Domain errors*, *Resolvers*].
- **A6. `main` only assembles.** It builds the components and wires them together; no logic in it
  [LGWT, *Scaling acceptance tests*, *Command line*].

## 3. Modules and Go project layout

- **M1. Split by business module** (bounded context), each in its own folder, not by technical
  kind (`controllers/`, `services/`, `repositories/`), which scatters one feature over several
  folders and leads to spaghetti code [DDH, *Folder structure*].
- **M2. Keep together what changes together** (Common Closure Principle, vertical slicing). A use
  case groups its handler, service, DTOs and command [DDH, *Folder structure*].
- **M3. Set apart what several components share** (entities, ports, common DTOs)
  [DDH, *Folder structure*].
- **M4. A module is a small application.** Its internals are private. Modules cooperate through a
  public facade, a mediator or messages, not by importing each other's details [DDH, *Modules*].
  In Go, the `internal/` folder and unexported identifiers make the compiler enforce this [Added].
- **M5. No dependency between use cases or between modules.** If two components share logic,
  extract it into a third one that both depend on [DDH, *Modules*].
- **M6. No catch-all `domain/` folder** with hundreds of unrelated types. Group ideas that belong
  together in packages named after the business [LGWT, *Scaling acceptance tests*].
- **M7. Add structure when it becomes necessary, not before.** The standard library is very flat.
  Conventions: `cmd/<app>/main.go` for each binary, the importable code beside it, adapters grouped
  coherently [LGWT, *Command line*, *Scaling acceptance tests*].
- **M8. Avoid deep folder nesting,** and move files around until the structure "feels right"
  [DDH, *Folder structure*].
- **M9. Explicit file names** for fuzzy search: `create_user.go`, `user_store.go`,
  `camera_handler.go` (DDH suggests `*.service.ts`; in Go, prefer underscores and meaningful
  suffixes) [DDH, *File names*].
- **M10. Ship the library with the program,** and the other way round: an API that can also be run
  from the command line is easier to learn [LGWT, *Maths*].

## 4. Domain layer

### Entities

- **D1. Entities carry the business logic.** Business logic that ends up in services gives an
  anaemic model. Exception: domain services, for logic that spans several entities
  [DDH, *Entities*].
- **D2. An entity has a stable identity**, and equality is on the identifier [DDH].
- **D3. An entity is always valid.** It checks its invariants at creation and fails fast on the
  first violation. In Go: **unexported fields plus a `NewX(...) (X, error)` constructor**, no empty
  construction [DDH, *Entities*].
- **D4. No public setters.** State changes through business methods (`Deposit`, `Withdraw`,
  `AssignTo`) that check the invariants again [DDH, *Entities*] [LGWT, *Pointers & errors*].
- **D5. Make immutable what must not change** (`id`, `createdAt`): no method changes them [DDH].
- **D6. Model the domain for the business logic, not for the database schema** [DDH].
- **D7. The domain does not know the upper layers** (services, handlers) [DDH].
- **D8. A module may hold several entities,** as long as their logic is related. Not one module
  per entity [DDH, *Entities*].

### Aggregates

- **D9. An aggregate is a group of objects treated as one unit.** Every reference from outside
  goes through the **root** [DDH, *Aggregates*].
- **D10. Operations on an aggregate are transactional** (all or nothing), and all its invariants
  hold after every change (consistency boundary) [DDH].
- **D11. Aggregates refer to each other by identifier,** not by pointer [DDH].
- **D12. Only roots are loaded directly from persistence** [DDH].
- **D13. Avoid large aggregates**: they cost performance and maintenance [DDH].

### Value objects and domain primitives

- **D14. Replace primitives that have rules with domain types** (against primitive obsession):
  `Email`, `IPAddress`, `FStop`, `Kelvin`, `Bitcoin`. In Go, a named type (`type Bitcoin int`) or a
  struct with an unexported field and a validating constructor [DDH, *Value objects*]
  [LGWT, *Pointers & errors*].
- **D15. A value object has no identity** (structural equality) and is **immutable**: in Go, value
  receivers, and methods return a new value [DDH].
- **D16. The value object holds its invariant and its behaviour** (validation, formatting,
  comparison). Entities no longer have to [DDH, quoting *Secure by Design*].
- **D17. The zero value always exists in Go.** Design the type so that its zero value is valid, or
  detectably invalid (`IsZero()`), since anyone can write `X{}` [Added].
- **D18. Only create a type for primitives that have rules or behaviour.** One type per field is
  too much in a small project [DDH, *Value objects*, notes].
- **D19. A value object does not leave the process as is** (DTOs, events, persistence): it is
  converted to primitives [DDH, *Value objects*, *Warning*].

### Making illegal states unrepresentable

- **D20. Let the compiler carry as many rules as possible**: typed constants (`type Mode int` plus
  `const`), distinct types, closed interfaces (an unexported method) to emulate a union,
  *typestate* (one type per state) [DDH, *Make illegal states unrepresentable*].
- **D21. What cannot be checked at compile time is checked at run time, in two places**: at the
  entry (DTO validation), then in the domain (invariant guards) [DDH, *Validation at runtime*].

### Validation or guard

- **D22. Validation filters the outside**: invalid input is nothing exceptional; reject it with
  clear messages, preferably all at once (*Notification* pattern) [DDH].
- **D23. Guards protect the domain**: an invariant violation inside the domain reveals a bug. In
  Go, the constructor returns an error; an "impossible" state detected internally may justify a
  `panic` (fail fast), never for user input [DDH, *Guarding vs validating*] [Added for the Go
  idiom].
- **D24. No generic validation library in the domain.** Keep it at the boundary. A very
  specialised library tied to one or two value objects is acceptable. Beware of home-made regular
  expressions that contradict those of the input validation [DDH].

### Domain services and events

- **D25. A domain service holds logic that spans several entities** and naturally belongs to none.
  It handles domain types only [DDH, *Domain services*].
- **D26. Domain events**: propagate a change to other aggregates of the same process without
  coupling them (in Go: in-memory subscribers, callbacks or a channel). They also feed an audit log
  [DDH, *Domain events*].
- **D27. Do not use only events for a long workflow**: a chain of events becomes unreadable. An
  orchestrator keeps the whole flow in one place [DDH].
- **D28. Integration events** (to other processes): publish them **after** persistence. In a
  distributed system: transactional outbox, sagas, eventual consistency [DDH, *Integration events*].

### Libraries in the core

- **D29. The core imports no library that** does I/O (HTTP, database), brings technology unrelated
  to the business (ORM, logger, framework), introduces randomness (UUID, `time.Now`), or changes
  often or pulls in many dependencies. For those: an adapter or a facade, that is, an
  anti-corruption layer [DDH, *Using libraries*].
- **D30. Inject the clock and the identifier generator** (`func() time.Time`, an `IDGenerator`
  interface) for deterministic tests. Go cannot replace them on the fly the way JavaScript can
  [DDH] [LGWT, *Mocking*, *Time*].
- **D31. Frameworks stay outside.** You must be able to extract the domain and rebuild an
  infrastructure around it without breaking the business logic [DDH].

## 5. Application layer (use cases)

- **U1. One application service per use case** (*interactor*). It orchestrates: loads the
  aggregates through the ports, calls their methods, persists, emits the external effects. **It
  holds no business logic** [DDH, *Application services*].
- **U2. It receives scalar types and converts them to domain types** [DDH].
- **U3. An application service does not depend on another application service** (risk of cycles)
  [DDH]. In Go, an import cycle does not even compile [Added].
- **U4. Separate commands and queries (CQS).** A command changes state and returns no business
  data (at most an identifier or a status). A query reads and changes nothing
  [DDH, *Commands and Queries*].
- **U5. A command is an intent struct** (`CreateUserCommand`), distinct from the input DTO
  [DDH, *DTOs*].
- **U6. No command triggers another command.** Chain them through an event (command → event →
  command) [DDH].
- **U7. A query may bypass the domain and the repositories** and read a read model directly
  [DDH, *Queries*].
- **U8. CQS from the start**: splitting reads and writes later (CQRS, separate databases) then
  needs no rewrite. It is a **scalability** lever [DDH].
- **U9. A command or query bus is only useful to decouple the caller from the executor.** In Go, a
  method call on an interface is usually enough [DDH] [Added].

## 6. Inbound adapters (HTTP, CLI, GUI, messages)

- **H1. A handler has three responsibilities, no more**: 1) parse and validate the request, 2) call
  the application service, 3) translate the result into a response (status, body). No business
  rule, no database connection, no e-mail sending [LGWT, *Revisiting HTTP handlers*]
  [DDH, *Controllers*].
- **H2. An `http.HandlerFunc` is just a function: test it by calling it** with
  `httptest.NewRecorder()` and `httptest.NewRequest` [LGWT, *HTTP server*].
- **H3. One adapter per kind of trigger** (HTTP, CLI, message), all plugged into the same use case:
  only the adapter changes from one transport to another [DDH, *Controllers*] [LGWT, *Scaling
  acceptance tests*].
- **H4. Request DTOs**: the contract the client must respect. Flat structs with `json` tags
  **only there**, never on domain types [DDH, *DTOs*].
- **H5. Allow-listed response DTOs**: expose only the intended fields, never the entity itself, so
  a sensitive field added later cannot leak [DDH, *Additional recommendations*].
- **H6. DTOs shield clients from internal changes.** To break a contract, version it (`/v2/...`)
  [DDH].
- **H7. A local DTO (between modules) is justified to keep one module's entities from leaking into
  another**, at the cost of mapping [DDH, *Local DTOs*].
- **H8. Translate domain errors into transport codes at the boundary** (`ErrNotFound` → 404,
  `ErrAlreadyExists` → 409). A 500 for every error makes a very poor API
  [LGWT, *Revisiting HTTP handlers*] [DDH, *Domain errors*].
- **H9. Announce the response format** (`Content-Type: application/json`) [LGWT, *JSON*].
- **H10. Routing with an `http.ServeMux` built once in the constructor,** not on every request
  [LGWT, *JSON*].
- **H11. HTML: `html/template`** (escaping against injection), templates parsed once and embedded
  with `embed`. `template.HTML` is for trusted content only [LGWT, *HTML templates*].
- **H12. Logic-free templates**: prepare a *view model* tested separately rather than functions in
  the template [LGWT, *HTML templates*].
- **H13. Graceful shutdown**: listen for `SIGTERM` (`os/signal`), stop accepting requests, let
  running ones finish (`http.Server.Shutdown`) [LGWT, *Intro to acceptance tests*].

## 7. Infrastructure (outbound adapters)

- **I1. The infrastructure is the most volatile layer**: keep it away from the domain, replaceable
  [DDH, *Infrastructure*].
- **I2. An adapter implements a port,** and is only called through that port [DDH, *Adapters*].
- **I3. An adapter has** its port, a *mapper* to and from the domain, a DTO for the data it
  receives, and **validation of incoming data**: an API or database answer is not trustworthy
  [DDH, *Adapters*, *Validation at runtime*].
- **I4. An adapter is an anti-corruption layer** in front of a legacy system or a badly designed
  API [DDH].
- **I5. A repository is a collection of aggregates**: it takes and returns domain types and hides
  where and how data is stored [DDH, *Repositories*].
- **I6. Abstracting the database is not always useful**: databases are rarely changed, and the
  abstraction keeps you from using their full power. Decide case by case [DDH, *Repositories*].
- **I7. Separate the domain model from the persistence model,** with a mapper: you can then
  normalise or denormalise without touching the domain. Overkill for a small application
  [DDH, *Persistence models*].
- **I8. For high-load data, prefer explicit SQL queries to an ORM** [DDH].
- **I9. HTTP client: never `http.DefaultClient`.** Inject an `*http.Client` configured with a
  timeout [LGWT, *Scaling acceptance tests*].
- **I10. Always close `resp.Body`,** or file descriptors leak [LGWT, *Select*].
- **I11. Do not create a connection on every call** (`sync.Once`, or build it once)
  [LGWT, *Scaling acceptance tests*].
- **I12. Use the standard interfaces** (`io.Reader`, `io.Writer`, `io.ReadWriteSeeker`, `fs.FS`):
  the code becomes reusable (file, network, `embed.FS`, test buffer) [LGWT, *Reading files*, *IO*].
- **I13. File writes replace the whole content**: truncate the file (`Truncate`) before writing
  from the start [LGWT, *IO*]. For an atomic write: a temporary file, then a rename [Added].
- **I14. Logger, scheduled jobs, framework code** are infrastructure too [DDH, *Other things*].

## 8. Errors

- **E1. The core never returns an HTTP status or a transport-specific error.** It returns domain
  errors [DDH, *Domain errors*].
- **E2. Expected business errors are explicit returned values** (the equivalent of DDH's `Result`
  is simply `(T, error)` in Go). Two forms:
  - **sentinel error**: `var ErrInsufficientFunds = errors.New("cannot withdraw, insufficient
    funds")`, tested with `errors.Is`;
  - **error type** with data (`BadStatusError{URL, Status}`), extracted with `errors.As`
    [DDH] [LGWT, *Pointers & errors*, *Error types*, *Maps*].
- **E3. Never compare the text of an error** (neither in code nor in tests): it is brittle for you
  and for your callers [LGWT, *Error types*].
- **E4. Add context by wrapping with `%w`**: `fmt.Errorf("withdraw from %s: %w", account, err)`. A
  bare `return err` at every layer loses the origin. `if err != nil { return err }` everywhere is
  **not** idiomatic [LGWT, *Pointers & errors*, *IO*].
- **E5. One precise error per useful case** (`ErrNotFound`, `ErrWordDoesNotExist`): the caller can
  react differently [LGWT, *Maps*].
- **E6. Constant errors**: `type DictionaryErr string` with its `Error()` method makes sentinels
  immutable [LGWT, *Maps*].
- **E7. Tell the expected from the unrecoverable.** A technical failure (lost connection, memory)
  that is ignored would leave the program inconsistent: propagate it to a layer that can stop, log
  or retry, and never hide it. An internal invariant violation may be a `panic` [DDH, *Domain
  errors*, *Warning*] [Added for the idiom].
- **E8. Leave no error unchecked** (`errcheck`, `go vet`) [LGWT, *Pointers & errors*].
- **E9. Check for `nil` before dereferencing** a returned pointer or interface, and never write to
  a `nil` map: initialise it (`map[K]V{}` or `make`) [LGWT, *Pointers*, *Maps*].
- **E10. Do not test `return err` for its own sake**: a test that only checks that an error is
  propagated adds little. Test where you **do** something with the error [LGWT, *Scaling
  acceptance tests*, *Reading files*].

## 9. Security (Secure by Design)

- **S1. Security comes from design.** Domain types that are always valid give input and output
  validation in **every** function, with no extra effort [DDH, quoting *Secure by Design*, ch. 5].
- **S2. Trust no data**: user, external API, database, configuration file, or programming error
  [DDH, *Validation at runtime*].
- **S3. Two lines of defence**: validation at the boundary (DTOs), then invariant guards in the
  domain [DDH].
- **S4. Allow-listed responses** (H5): no leak of a field added later [DDH].
- **S5. The core has no access to external resources** (no I/O in the domain): a smaller attack
  surface and testable behaviour [DDH, *Using libraries*].
- **S6. Few dependencies.** Each dependency is a possible source of bugs and vulnerabilities.
  Strike a balance: do not rewrite a proven library, but do not import one for three lines
  [DDH, *Using libraries*]. Check known vulnerabilities with `govulncheck` [Added].
- **S7. Encapsulation**: export only what is needed. Do not embed a concrete type whose public
  methods would become yours (an embedded `sync.Mutex` exposes `Lock`/`Unlock`, an embedded
  `http.ServeMux` lets a caller add routes) [LGWT, *Sync*, *JSON*].
- **S8. HTML escaped by default** (`html/template`), `template.HTML` only for trusted content or
  content passed through a reliable sanitiser [LGWT, *HTML templates*].
- **S9. `context.Value` informs, it does not control**: a trace id, yes; data needed to work, no.
  That goes in typed parameters [LGWT, *Context*].
- **S10. Timeouts everywhere**: HTTP client, `select` with `time.After`, `context.WithTimeout`. An
  endless wait is a self-inflicted denial of service [LGWT, *Select*, *Context*].
- **S11. No "test mode" in production code** (`GetData(mode string)`): inject the dependency
  instead [LGWT, *OS exec*].
- **S12. Secrets outside the code and outside versioned configuration,** in files with restricted
  rights [Added; applied here: passwords are kept apart from `rig.json`, encrypted with DPAPI].

## 10. Interfaces, dependency injection, public API

- **C1. Dependency injection is just passing the dependency as a parameter.** No framework needed,
  and it does not complicate the design [LGWT, *Dependency injection*].
- **C2. Accept interfaces, return structs.** Take the smallest interface that is enough
  (`io.Writer` rather than `*bytes.Buffer`, `io.Reader` rather than `fs.File`, `string` rather than
  `fs.DirEntry` when only the name matters) [LGWT, *DI*, *Reading files*].
- **C3. The bigger the interface, the weaker the abstraction.** An interface with many methods
  betrays a leaky abstraction. Aim for one method [LGWT, *Anti-patterns*].
- **C4. Avoid interface pollution**: expose a struct with its methods, and let each client declare
  the interface of the subset it needs. Expose an interface only if it is small and coherent,
  **and** decoupling is needed (several implementations, an external dependency to replace)
  [LGWT, *Anti-patterns*] [DDH, *Ports* (interface segregation)].
- **C5. For a one-method interface, provide the function adapter type**
  (`type BlindAlerterFunc func(...)`), like `http.HandlerFunc` [LGWT, *Time*].
- **C6. No global state** (connection pool, configuration in package variables): it makes code
  untestable and hides responsibilities. With injection, the list of dependencies documents the
  intent [LGWT, *DI*, *Time*].
- **C7. A growing list of dependencies is a signal**: from 4 on, the type probably does too much.
  Group them into a business collaborator (`Game`) [LGWT, *Time*, *Anti-patterns*].
- **C8. `NewX(...)` constructors** whenever the zero value is not usable or private fields must be
  filled; they tell the reader not to initialise the type by hand [LGWT, *Sync*, *Command line*].
- **C9. Compute in the constructor what does not depend on the method's arguments** (base URL,
  parsed template, encoder) [LGWT, *Refactoring checklist*, *IO*, *HTML templates*].
- **C10. Consistent receivers**: if one method needs a pointer (mutation, mutex), all the methods
  of the type use a pointer [LGWT, *Pointers*].
- **C11. Composition over inheritance**: no deep inheritance chains. Go has no inheritance at all;
  embedding is composition, but it widens your public API (S7) [DDH, *Prevent massive inheritance
  chains*] [LGWT, *JSON*].
- **C12. To add behaviour without changing the code** (logging, telemetry, retries, cancellation),
  use the *decorator* or *delegation* pattern: a struct that wraps the interface
  [LGWT, *Context-aware reader*, *Working without mocks*].
- **C13. Avoid `interface{}` / `any` and reflection**: you lose type safety and push errors to run
  time. Prefer a real interface or generics [LGWT, *Reflection*, *Generics*].
- **C14. Generics: only after seeing the same code about three times,** starting from tested
  concrete implementations. A little duplication is better than coupling to a bad abstraction
  [LGWT, *Generics*].
- **C15. Compile errors are your friends, run-time errors your enemies**: listen to the compiler
  and express constraints in types [LGWT, *Arrays*, *Generics*].
- **C16. Document exported APIs** (doc comments) and add testable `Example`s: the documentation
  stays right because it is compiled and run [LGWT, *Integers*].

## 11. Concurrency, `context`, time

- **K1. Always run the tests with `-race`.** An unsynchronised concurrent access is a bug even if
  it "works" [LGWT, *Concurrency*, *Revisiting time with synctest*].
- **K2. Channels to hand over ownership of data, mutexes to protect shared state** (Go wiki,
  *MutexOrChannel*). Do not overuse goroutines and channels just because you can [LGWT, *Sync*].
- **K3. One shared value: `sync/atomic`** (`atomic.Int64`, `atomic.Bool`); several fields tied by
  an invariant: `sync.Mutex` [LGWT, *Sync*].
- **K4. A mutex is not copied** (`go vet` detects it): pass a pointer, provide a constructor. Do
  not embed it (S7) [LGWT, *Sync*].
- **K5. Do not communicate by sharing memory; share memory by communicating.** A goroutine that
  produces a result sends it on a channel rather than writing a shared variable
  [LGWT, *Revisiting time with synctest*].
- **K6. Create channels with `make`**: a `nil` channel blocks forever. For a mere signal, use
  `chan struct{}` and `close` [LGWT, *Select*].
- **K7. Every `select` that can block has a way out**: `ctx.Done()` or `time.After`
  [LGWT, *Select*].
- **K8. `context.Context` as the first parameter of every function on the path between an inbound
  request and an outbound call.** Propagate it, derive it (`WithCancel`, `WithTimeout`), honour
  `ctx.Done()`, and return `ctx.Err()` so the caller knows the cause. Each component handles its
  own cancellation rather than a caller "cancelling" its dependencies by hand
  [LGWT, *Context*, *Context-aware reader*].
- **K9. A loop variable captured by a goroutine**: safe since Go 1.22, only if `go.mod` declares
  `go >= 1.22` [LGWT, *Concurrency*].
- **K10. Time is a dependency**: inject a clock or a scheduler (`BlindAlerter`), or test the real
  code with `testing/synctest` (Go 1.25+) [LGWT, *Time*, *Revisiting time*].
- **K11. Sleeping does not synchronise**: a "long enough" `time.Sleep` is still a race.
  Synchronise for real (channel, `WaitGroup`, `synctest.Wait()`) [LGWT, *Revisiting time*].
- **K12. A `time.AfterFunc` callback runs in its own goroutine**: the state it touches must be
  protected [LGWT, *Revisiting time*].
- **K13. `sync.WaitGroup` to wait for a group of goroutines** [LGWT, *Sync*].
- **K14. Maps are not safe for concurrent writes** (fatal error `concurrent map writes`)
  [LGWT, *Concurrency*].
- **K15. Transactions** guarantee consistency but become a bottleneck when many users change the
  same row. Beyond that, move to eventual consistency (sagas, outbox) [DDH, *Domain events*]. It is
  the key **scalability** rule on the data side.

## 12. TDD process

- **P1. The cycle** [LGWT, *Hello, World*]:
  1. write a small test for a small behaviour;
  2. make it compile (listen to the compiler, write just the signatures);
  3. **run it and see it fail, with a clear message**;
  4. write the minimum to make it pass ("sins" are allowed);
  5. refactor, production code **and** test code;
  6. commit.
- **P2. Seeing the test fail is mandatory**: it is the only way to avoid tests that can never fail
  and useless failure messages (`false was not equal to true`) [LGWT, *Anti-patterns*].
- **P3. Get out of the red as fast as possible.** The more you change while it does not compile or
  tests fail, the more problems pile up. Priority: make it compile, then make the tests pass
  [LGWT, *HTTP server*].
- **P4. The minimum to pass reveals holes in the tests** (a 404 returned to everyone passes the
  tests → an assertion on the 200 is missing) [LGWT, *HTTP server*].
- **P5. TDD is first a source of design feedback.** If a test is painful to write, the code will
  be painful to use: the test is its first user. **Listen to your tests** [LGWT, *Anti-patterns*].
- **P6. Start from the outside (top-down)**: an acceptance test sets the course, then unit tests
  bring out the units. Bottom-up produces code that does not integrate or does useless things
  [LGWT, *Scaling acceptance tests*, *Reading files*].
- **P7. Write the test you would like to see**, from the consumer's point of view: the *what* and
  the *why*, not the *how* [LGWT, *Reading files*].
- **P8. Skeleton first** ("iteration 0"): a minimal program, built and tested end to end, before
  the features [LGWT, *Scaling acceptance tests*, *Command line*].
- **P9. Integrate early**: plug the code into `main` quickly. Using it from another package is how
  you discover private fields that cannot be filled [LGWT, *Command line*].
- **P10. TDD is not a religion**: when you know where you are going, take bigger steps, but always
  write the test first [LGWT, *Maths*]. To explore, make a throwaway spike (after a commit), then
  go back and drive the real solution with a test [LGWT, *WebSockets*].
- **P11. "Write tests until fear is transformed into boredom"** (often attributed to Kent Beck)
  [LGWT, *Maths*].

## 13. Test strategy: levels and test doubles

### What to test

- **Q1. Test behaviour, not implementation.** Tests coupled to details (the inner triangles of a
  square) block refactoring instead of enabling it [LGWT, *Why*].
- **Q2. A "unit" is a coherent component with a public API,** not a function or a class. Test its
  public API; underneath, refactor freely [LGWT, *Why*].
- **Q3. Do not test private functions** (except as an assumed exception for a precise risk, see
  `tape` in [LGWT, *IO*]). Do not make a function public just to test it
  [LGWT, *Anti-patterns*, *Mocking*].
- **Q4. Do not test other people's code**: neither the output of `encoding/json` nor a third-party
  library [LGWT, *JSON*, *HTML templates*].
- **Q5. Every test has a cost.** The goal is confidence, not the number of tests nor 100 %
  coverage. Coverage helps find holes; delete redundant tests [LGWT, *Arrays*, *Pointers*].
- **Q6. Tests are not throwaway code**: a good test outlives the code it tests. Take care of it
  like production code [LGWT, *Maths*].

### Pyramid

- **Q7. Many fast unit tests, a few acceptance tests** [LGWT, *Intro to acceptance tests*,
  *Scaling acceptance tests*].
- **Q8. Acceptance test** = black box, from the user's point of view, only through the system's
  public interface. It is verified documentation. Cost: slow, expensive, imprecise about the cause
  of a failure, blind to internal quality [LGWT].
- **Q9. When to write an acceptance test?** For a user journey, or something non-technical people
  talk about a lot. An edge case is a unit test [LGWT, *Scaling acceptance tests*].
- **Q10. Structure acceptance tests as Specification → DSL → Driver → System** (Dave Farley). The
  specification expresses only the business and changes only if the behaviour changes. The driver
  (HTTP, gRPC, browser, direct domain call) absorbs technical changes. The same specification can
  serve as a unit test of the domain through an adapter [LGWT, *Scaling acceptance tests*].
- **Q11. Configurable drivers**: the same acceptance tests run locally, in staging and ideally in
  production [LGWT].
- **Q12. Legacy code without tests: write acceptance tests first, refactor afterwards**
  [LGWT, *Scaling acceptance tests*].
- **Q13. Integration tests in between**: several units together (service plus fakes, server plus
  store) without starting the whole system [LGWT, *Working without mocks*, *HTTP server*].
- **Q14. A non-technical person who knows the business must understand your acceptance tests**
  (Dave Farley) [LGWT]. It is also the spirit of behavioural tests and Gherkin [DDH, *Behavioral
  testing*].

### Test doubles

- **Q15. Choose the double on purpose** [LGWT, *Working without mocks*, *Anti-patterns*]:
  - *dummy*: fills a parameter list, never used; name it `dummyX`;
  - *stub*: returns fixed data; **prefer it to a mock**;
  - *spy*: records calls; use it only if the order or number of calls **is** the wanted
    behaviour;
  - *mock*: programmed expectations; powerful, so easy to misuse;
  - *fake*: a real simplified implementation (in-memory store); it keeps state.
- **Q16. More than 3 doubles in a test: red alert** about the design. The code has too many
  dependencies or too fine-grained ones [LGWT, *Mocking*].
- **Q17. Complicated interactions with mocks mean code that interacts too much**: simplify the
  collaboration [LGWT, *Anti-patterns*].
- **Q18. Fakes plus contracts for important dependencies**: a contract is a test suite written
  against the **interface** and run both on the fake and on the real implementation. Assumptions
  about the dependency are then codified and checked, instead of scattered in ad hoc stubs
  [LGWT, *Working without mocks*].
- **Q19. When your knowledge of a dependency changes**: update the contract, check it against the
  real system, bring the fake into line, then run the whole suite. Other tests that break are
  **good** news [LGWT].
- **Q20. With a fake, check the final state, not the calls** ("which records does the fake
  hold?") [LGWT].
- **Q21. To make a fake fail, wrap it in a decorator** with overridable `XxxFunc` fields, or keep a
  stub at unit level for the error paths [LGWT].
- **Q22. Do not mock the database driver nor check the SQL sent**: it is an implementation detail.
  In-memory fakes and contracts; the real database only to validate the contract
  [LGWT, *Working without mocks*].
- **Q23. Mock generators**: acceptable in a team for consistency, only if they generate from an
  interface, with no "magic" that dictates the shape of the tests [LGWT, *Mocking*].
- **Q24. Exported doubles**: providing stubs and helpers in a `testing.go` of the package helps its
  users (a HashiCorp practice). It is a deliberate choice that widens the API
  [LGWT, *Command line*].

## 14. Testing techniques in Go

- **T1. Write tests in an external `xxx_test` package** by default: the compiler then keeps you
  from coupling to the internals, and you get a real user's experience [LGWT, *Anti-patterns*,
  *Command line*, *Reading files*].
- **T2. Test helpers**: take `testing.TB` (good for tests and benchmarks) and call `t.Helper()` so
  that a failure points at the caller's line [LGWT, *Hello, World*].
- **T3. `t.Fatal` when the rest of the test no longer makes sense** (unexpected nil error, failed
  decoding), `t.Error` otherwise [LGWT, *Pointers & errors*, *JSON*].
- **T4. Useful failure messages**: `got %q want %q`, `%#v` for a struct, the inputs shown. For
  floats: `%g` [LGWT, *Hello*, *Arrays*, *Structs*].
- **T5. Subtests `t.Run("case", ...)`** to group and name scenarios; run just one with
  `go test -run TestX/case` [LGWT, *Hello*, *Structs*].
- **T6. Aim for one assertion per test**: subtests separate the assertions, which keeps the test
  readable and precise when it fails [LGWT, *Anti-patterns*].
- **T7. Table tests when only the input varies**: named fields, case names, `t.Run`. **Leave the
  table** as soon as it would need booleans or control fields [LGWT, *Structs*, *Anti-patterns*].
- **T8. Assert only what matters to you**: not a whole struct when one field counts
  [LGWT, *Anti-patterns*].
- **T9. Structured output (JSON, XML, SVG): parse it and compare data,** do not compare strings:
  otherwise the test is both brittle and not precise enough [LGWT, *JSON*, *Maths*].
- **T10. Large output (HTML, reports): approval tests** ("golden files"). Approved files go into
  git [LGWT, *HTML templates*].
- **T11. Floats: compare with a tolerance** (or rearrange the computation to keep precision)
  [LGWT, *Structs*, *Maths*].
- **T12. Unspecified order (maps): do not assert on order,** but still check length and content
  [LGWT, *Reflection*].
- **T13. Property-based tests** (`testing/quick`) for domain rules, for example
  `FromRoman(ToRoman(n)) == n`. Bound the domain of generated inputs [LGWT, *Roman numerals*].
- **T14. Standard library tools**: `httptest` (server and recorder), `fstest.MapFS`,
  `strings.NewReader`, `bytes.Buffer`, `io.Discard`, `os.CreateTemp` with clean-up
  [LGWT, *Select*, *Reading files*, *IO*].
- **T15. No real network call in a unit test**: it is slow, flaky, and edge cases cannot be
  provoked [LGWT, *Select*].
- **T16. No arbitrary `time.Sleep` in tests.** For asynchronous code: `within(timeout, assert)`,
  `retryUntil`, or `synctest`. **A test must never block forever** [LGWT, *WebSockets*,
  *Revisiting time*].
- **T17. Configurable delays** (`ConfigurableRacer`) so that the timeout test takes milliseconds,
  with a default function for users [LGWT, *Select*].
- **T18. `testing/synctest`**: a bubble with a fake clock. Time only advances when every goroutine
  is durably blocked. Every goroutine must end, and real I/O is not durably blocking (use
  `net.Pipe` instead) [LGWT, *Revisiting time*].
- **T19. `go test ./...` must run without configuration** (apart from the compiler, perhaps
  Docker). Slow acceptance tests skip with `if testing.Short() { t.Skip() }` [LGWT, *Scaling
  acceptance tests*].
- **T20. Benchmarks** (`b.Loop()`, `-bench=. -benchmem`) before and after an optimisation
  [LGWT, *Iteration*, *Concurrency*].
- **T21. Testable `Example`s** for the documentation (`// Output:`) [LGWT, *Integers*].
- **T22. Break the code on purpose** (after a commit) to check that a test catches it
  [LGWT, *Mocking*].
- **T23. Be patient with error messages and stack traces**: everything you need to know is written
  there [LGWT, *Concurrency*, *Structs*].

## 15. Refactoring

- **R1. Refactoring is changing the code without changing its behaviour.** If you have to change
  the tests, you are not refactoring, you are **designing**. Then drive the change with a test
  [LGWT, *Why*, *Refactoring checklist*].
- **R2. When the tests are green, anything goes except changing behaviour**: new types,
  interfaces, private functions [LGWT, *Anti-patterns*].
- **R3. Small steps, tests at each step** (a few milliseconds), commit if it is good, revert
  otherwise [LGWT, *Refactoring checklist*].
- **R4. Do not change production code and tests at the same time**: first the production code
  without touching its interface, then the tests [LGWT, *Time*].
- **R5. Automatic renames by the IDE count as refactoring** even when they touch the tests: they
  are safe [LGWT, *Refactoring checklist*].
- **R6. Checklist at every cycle** [LGWT, *Refactoring checklist*]:
  - inline a variable that adds no meaning;
  - extract a variable or a constant for a repeated or magic value, knowing that everything that
    shares it becomes coupled;
  - DRY on the **idea**, not on the lines; if the parameter list gets confusing, it is bad DRY;
  - public functions readable at a glance: they say **what**, private functions say **how**;
  - move the creation of argument-independent values into the constructor;
  - replace a comment with a well-named function.
- **R7. First make the change easy, then make the easy change** (Kent Beck) [LGWT, *Scaling
  acceptance tests*].
- **R8. Refactoring is not asked for, it is part of the job.** "No time to refactor" means
  precisely that you must refactor [LGWT, *Refactoring checklist*].
- **R9. Data rather than `switch`es**: a table of rules (`allRomanNumerals`) rather than logic
  hidden in branches [LGWT, *Roman numerals*].
- **R10. A failed abstraction gets undone**: go back without drama, that is what the tests are for
  [LGWT, *Reflection*].

## 16. Performance

- **F1. Measure before optimising** (benchmarks, `-benchmem`) [LGWT, *Iteration*].
- **F2. Concurrency to wait in parallel** for independent I/O, keeping sequential what must be
  (collect results through a channel) [LGWT, *Concurrency*].
- **F3. Do once what can be done once**: parse the templates, build the router, the JSON encoder,
  the dialer, read the file at start-up rather than on every call [LGWT, *HTML templates*, *JSON*,
  *IO*, *Scaling acceptance tests*]. Exception: a stateful object that cannot be reused (the
  gomarkdown parser) is created every time [LGWT, *HTML templates*].
- **F4. `strings.Builder` to concatenate in a loop** [LGWT, *Iteration*].
- **F5. Avoid large aggregates and overly wide transactions** (D13, K15) [DDH].
- **F6. CQS/CQRS**: read models optimised separately from writes when the load requires it (U8)
  [DDH].
- **F7. Modules split along bounded contexts**: a module can be extracted into a microservice
  without touching the domain. That is **organisational and technical scalability** [DDH,
  *Architecture*, *Modules*].
- **F8. Sorting**: where it is most efficient (in the database with `ORDER BY` if there is one,
  otherwise `sort.Slice` in the store) [LGWT, *IO*].

## 17. Tooling and discipline

- **O1. `gofmt`, `go vet` (mutex copies and more), `errcheck`, `go test -race ./...`** on every
  change [LGWT, *Sync*, *Pointers & errors*, *Concurrency*].
- **O2. Your unit suite should run in under a second**: it is what lets you refactor boldly
  [LGWT, *Why*].
- **O3. A button in the editor to run the tests and refactor** (extract, inline, rename) in one
  shortcut, or you will do it less [LGWT, *Refactoring checklist*].
- **O4. Commit as soon as the tests pass, before refactoring**: you can always go back to the state
  that works [LGWT, *Hello*, *Revisiting arrays with generics*].
- **O5. Study the standard library**: `io`, `fs`, `context`, `sync`, `time`, `httptest`, `embed`,
  `slices`. Every standard interface you know makes your code reusable [LGWT, *DI*, *Reading
  files*].
- **O6. `embed` for the files the binary needs** (templates, assets): no fragile relative path that
  depends on the launch folder [LGWT, *HTML templates*, *Command line*].
- **O7. `go.mod`: declare a recent Go version** (loop semantics in 1.22, `synctest` in 1.25)
  [LGWT, *Concurrency*].
- **O8. Contracts and acceptance tests in the CI pipeline** to be warned quickly when an external
  dependency changes [LGWT, *Working without mocks*].

## 18. Enforcing the architecture

- **X1. Check the dependency rules automatically**: the domain must not import the API, the
  database or the adapters [DDH, *Enforcing architecture* (dependency-cruiser, ArchUnit)].
- **X2. In Go** [Added]:
  - the compiler already forbids import cycles;
  - `internal/` limits who can import what;
  - an architecture test (`go list -deps ./internal/core/...`, then fail if an adapter package or
    `net/http` shows up), or `depguard` / `go-arch-lint` through golangci-lint.
- **X3. Keep modules independent**: a module must be deletable and rewritable in a few days
  [DDH, *Modules*].

## 19. When to go lighter (small projects)

[DDH, *Recommendations for smaller APIs*] [LGWT, *Generics*, *IO*]

- **Always useful**, whatever the size: modules and layers, thin handlers, services, entities that
  protect their invariants, dependency injection, respected boundaries, TDD, typed errors,
  `-race`.
- **To weigh** for a mostly CRUD application with little logic: one type per primitive, a separate
  persistence model and mappers, a command bus, domain events, CQRS with separate databases, ports
  in front of a database that will never change. A simple MVC can be enough.
- **An interface without real polymorphism** (nobody mocks it, a single plausible implementation,
  `*os.File`) brings nothing: keep the concrete type [LGWT, *IO*].

## 20. How this repository applies them

How these rules apply to RCP For All Camera (see [architecture.md](architecture.md) and the
[documentation home](index.html#architecture)):

| Notion | Here |
|---|---|
| Domain (contracts, business types) | `internal/core`: `ControlSpec`, `Value`, `ModelInfo`, `Page`, errors |
| Application (use cases) | `internal/app`: `Service` (slots, controls, presets, settings), `Host` |
| Ports | `core.Module`, `core.Camera`, `app.Store`, `app.Secrets`, `app.PanelSources`, `panel.Device`, `gui.StatsSampler`, `gui.FilePicker` |
| Inbound adapters | `internal/gui` (Fyne), `internal/surfaces/panel` (the panel drives the app) |
| Outbound adapters | `internal/plugins` (WASM module host, anti-corruption layer for modules), `internal/surfaces/micropanel` (HID), `internal/config` (`rig.json`, secrets) |
| Camera modules | `modules/<id>/`, one WASM program per model, written with `sdk/rcpmod` |
| Fakes | the `demo` module, the fake camera servers of the module tests, `internal/surfaces/panelsim` |
| Contracts | `sdk/rcpmod/rcpmodtest` (`Module`, `Online`), run by every module and by the host |
| Architecture test | `internal/archtest/dependency_rules_test.go` |
| Assembly | `cmd/rcp` |

Points that follow from it:

- protocol details stay in the module (A1, I2); the core sees only `core.Value`;
- `TODO(verify)` ranges are assumptions about an external system: they belong in a contract that
  can be checked on the hardware (Q18), like the `Live` tests of the Blackmagic module;
- the demo camera and the panel simulator are fakes: they must pass the same contracts as the real
  things (Q18, Q19);
- a camera answer is untrusted data (S2, I3): it is validated and normalised in the adapter (float32
  values rounded to 6 decimals), and the module host validates everything a module reports.

## 21. Review checklist

**Architecture**
- [ ] The domain and the application import no adapter, framework or `net/http` (A1, X1).
- [ ] Handlers only parse, call and respond (H1).
- [ ] `main` only assembles (A6).
- [ ] No use case calls another use case (U3); commands return no business data (U4).

**Domain**
- [ ] New types have unexported fields, a validating constructor and a controlled zero value
      (D3, D17).
- [ ] No public setter; invariants are checked again on every mutation (D4).
- [ ] No `json` tag and no database type on domain types (H4, I7).

**Errors and security**
- [ ] Errors are wrapped with `%w` and context, tested with `errors.Is`/`As`, never by their text
      (E3, E4).
- [ ] Every external input (user, network, file, hardware) is validated at the boundary (S2, I3).
- [ ] Responses expose an allow-list of fields (H5).
- [ ] Every I/O has a timeout and a `context` (S10, K8); no `http.DefaultClient` (I9).
- [ ] No new dependency without justification (S6).

**Concurrency**
- [ ] `go test -race` passes; no copied or embedded mutex (K1, K4).
- [ ] Every blocking `select` has a way out (K7); every goroutine has an end.

**Tests**
- [ ] The test was seen failing, with a clear message (P2).
- [ ] It tests the public API, in a `_test` package (Q2, T1).
- [ ] No more than 3 doubles; fakes with a contract for important dependencies (Q16, Q18).
- [ ] No arbitrary `time.Sleep`; no test can block (T16).
- [ ] No string comparison on JSON/XML or error messages (T9, E3).

---

### Sources

- Sairyss, *Domain-Driven Hexagon*, README: <https://github.com/Sairyss/domain-driven-hexagon>
- Chris James and contributors, *Learn Go with Tests*, all chapters (Go fundamentals, testing
  fundamentals, build an application, questions and answers, meta):
  <https://quii.gitbook.io/learn-go-with-tests> (source: <https://github.com/quii/learn-go-with-tests>)
- Books quoted by these guides: *Secure by Design* (Johnsson, Deogun, Sawano), *Growing
  Object-Oriented Software, Guided by Tests* (Freeman, Pryce), *Refactoring* (Fowler), *Modern
  Software Engineering* (Farley).
