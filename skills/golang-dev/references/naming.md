# Naming

Go files only. Skip `vendor/`, `// Code generated`, `*_mock.go`, `*.pb.go`. External/generated symbols are out of scope.

| Rule | Bad | Good |
| ---- | --- | ---- |
| No package/type stutter | `user.UserService` | `user.Service` |
| Acronyms full caps | `userId`, `httpUrl` | `userID`, `httpURL` |
| Receiver 1–3 letters, consistent | `func (this *User)` | `func (u *User)` |
| Verb-first actions | `UserGet` | `GetUser` |
| No redundant suffixes | `GetUserFunc`, `UserStruct` | `GetUser`, `User` |
| Booleans are predicates | `enable` | `isEnabled` |
| Single-method interface `-er` | `ReadInterface` | `Reader` (`IReader` ok) |
| Errors: `Err*` var, `*Error` type | `NotFound` | `ErrNotFound`, `NotFoundError` |
| Constants `SCREAMING_SNAKE` | `MaxRetries` | `MAX_RETRIES` |
| `any` not `interface{}` | `map[string]interface{}` | `map[string]any` |

Packages: short lowercase singular noun; no `util`/`common`/`helpers`; last path element = package name; contents don't repeat the package (`chain.New()` not `chain.NewChain()`). Don't steal good local names (`bufio` not `buf`). `foo_test` external-test packages are fine.

Rename with `gopls rename -w file:line:col NewName` only — never Edit/sed. Sequential, then `go build ./...`. Package rename: `gopls` on the `package` clause, then `git mv` the directory. Don't automate a `util` split — describe it.
