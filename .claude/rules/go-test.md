---
paths:
  - "**/*_test.go"
---

# Go tests

- **testify + table-driven.** Use `assert` / `require`; prefer table-driven cases. Tests live next to the source file.
- **`require` vs `assert`.** `require` for anything the rest of the test cannot proceed without — setup, `require.NoError(t, err)`, nil/length checks before indexing or dereferencing. `assert` for the actual expectations, so one failed check still reports the rest.
- **Table format.** Declare cases as `testCases := []struct{ name string; ... }` and iterate with `for _, testCase := range testCases { t.Run(testCase.name, ...) }` — the variables are named `testCases` / `testCase`. Case names are PascalCase segments joined by `_`, one segment per aspect (scenario, condition, expectation): `TimestampNotNullNoDefault`, `DropPrimaryKeyConstraint_AlterColumnNullable`, `ForeignKeyConstraint_DoesNotExist_SCreateAndDropConstraintTrue`.
- **No hoisted test constants.** Vary fixture strings across cases instead of hoisting a constant — never introduce a shared const for test data. 
