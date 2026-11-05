package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// Plain Drizzle module: the receiver is the adapter's database directly.
const slp207PlainDbModule = `import { drizzle } from "drizzle-orm/node-postgres";
import { Pool } from "pg";

export const pool = new Pool({ connectionString: process.env.DATABASE_URL });
export const db = drizzle(pool);
`

// Scoped-proxy module: the exported receiver is a scoped executor proxy over a
// Drizzle database, typed with Drizzle database/transaction executor types.
const slp207ScopedDbModule = `import { drizzle } from "drizzle-orm/node-postgres";
import { createScopedDatabaseExecutor } from "./lib/scopedDatabaseExecutor";

export type DbInstance = ReturnType<typeof drizzle>;
export type DbExecutor = DbInstance | Parameters<Parameters<DbInstance["transaction"]>[0]>[0];

const base = drizzle(process.env.DATABASE_URL!);
const scopedDatabase = createScopedDatabaseExecutor<DbExecutor>(base);
export const db = scopedDatabase.executor as DbInstance;
export const withDatabaseExecutor = scopedDatabase.run;
`

const slp207ScopedExecutorModule = `import { AsyncLocalStorage } from "node:async_hooks";

export function createScopedDatabaseExecutor<T extends object>(fallback: T) {
  const context = new AsyncLocalStorage<{ executor: T; active: boolean }>();
  const executor = new Proxy(fallback, {
    get(target, property) {
      const scope = context.getStore();
      if (scope && !scope.active) {
        throw new Error("executor scope has ended");
      }
      const current = scope?.executor ?? target;
      const value = Reflect.get(current, property, current);
      return typeof value === "function" ? value.bind(current) : value;
    },
  });
  return {
    executor,
    async run<R>(selected: T, operation: () => Promise<R>): Promise<R> {
      const scope = { executor: selected, active: true };
      return context.run(scope, async () => {
        try {
          return await operation();
        } finally {
          scope.active = false;
        }
      });
    },
  };
}
`

// Managed Drizzle transaction: single-line begin, awaited, rollback by Drizzle.
const slp207ManagedService = `import { eq } from "drizzle-orm";
import { db } from "../db";
import { users } from "../schema";

export async function activateUser(name: string) {
  await db.transaction(async (tx) => {
    const [row] = await tx.insert(users).values({ name }).returning();
    if (!row) throw new Error("insert failed");
    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));
  });
}
`

// Multiline begin with a serializable isolation option, returned to caller.
const slp207MultilineService = `import { eq } from "drizzle-orm";
import { db } from "../db";
import { audit, users } from "../schema";

export async function markSeen(id: string, actor: string) {
  return db.transaction(
    async (tx) => {
      const rows = await tx.select().from(users).where(eq(users.id, id)).for("update");
      if (!rows.length) throw new Error("missing user");
      await tx.update(users).set({ seen: true }).where(eq(users.id, id));
      await recordAudit(actor, tx);
      return rows[0];
    },
    { isolationLevel: "serializable" },
  );
}
`

// Managed Drizzle transaction plus an unmanaged knex transaction in one hunk.
const slp207MixedService = `import { db } from "../db";
import knex from "../knex";
import { users } from "../schema";

export async function copyUser(name: string) {
  await db.transaction(async (tx) => {
    await tx.insert(users).values({ name });
  });
  const trx = await knex.transaction();
  await trx("users_archive").insert({ name });
  if (!name) {
    throw new Error("missing name");
  }
}
`

const (
	slp207ServicePath = "server/services/user.ts"
	slp207DbPath      = "server/db.ts"
	slp207ScopedPath  = "server/lib/scopedDatabaseExecutor.ts"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// slp207NewFileDiff renders content as a newly added file.
func slp207NewFileDiff(path, content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, path, path, len(lines))
	for _, l := range lines {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

// slp207Scan writes files under a temporary repo root and scans target as a
// newly added file. diffContent, when non-empty, overrides the diff side so
// a test can make the diff disagree with the snapshot.
func slp207Scan(t *testing.T, files map[string]string, target, diffContent string, withRoot bool) []Finding {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if diffContent == "" {
		diffContent = files[target]
	}
	text := slp207NewFileDiff(target, diffContent)
	if !withRoot {
		return SLP207{}.Check(parseDiff(t, text))
	}
	return SLP207{}.Check(parseDiffWithRoot(t, root, text))
}

func slp207MustReplace(t *testing.T, s, old, repl string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture mutation did not apply: %q not found", old)
	}
	return strings.Replace(s, old, repl, 1)
}

func slp207LineOf(t *testing.T, content, needle string) int {
	t.Helper()
	for i, l := range strings.Split(content, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("line containing %q not found", needle)
	return 0
}

func slp207Lines(fs []Finding) []int {
	var out []int
	for _, f := range fs {
		out = append(out, f.Line)
	}
	return out
}

func slp207PlainFiles(service string) map[string]string {
	return map[string]string{slp207DbPath: slp207PlainDbModule, slp207ServicePath: service}
}

func slp207ScopedFiles(service string) map[string]string {
	return map[string]string{
		slp207DbPath:      slp207ScopedDbModule,
		slp207ScopedPath:  slp207ScopedExecutorModule,
		slp207ServicePath: service,
	}
}

// ---------------------------------------------------------------------------
// Acceptance: callback-managed Drizzle transactions are not reported
// ---------------------------------------------------------------------------

func TestSLP207_ManagedDrizzle_DirectAdapter(t *testing.T) {
	files := slp207PlainFiles(slp207ManagedService)
	// Diff-only analysis (no snapshot) keeps the original heuristic finding.
	if got := slp207Scan(t, files, slp207ServicePath, "", false); len(got) != 1 {
		t.Fatalf("diff-only: want 1 finding, got %d: %+v", len(got), got)
	}
	if got := slp207Scan(t, files, slp207ServicePath, "", true); len(got) != 0 {
		t.Fatalf("managed Drizzle transaction: want 0 findings, got %+v", got)
	}
}

func TestSLP207_ManagedDrizzle_MultilineSerializable(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"plain":  slp207PlainFiles(slp207MultilineService),
		"scoped": slp207ScopedFiles(slp207MultilineService),
	} {
		t.Run(name, func(t *testing.T) {
			if got := slp207Scan(t, files, slp207ServicePath, "", false); len(got) != 1 {
				t.Fatalf("diff-only: want 1 finding, got %+v", got)
			}
			if got := slp207Scan(t, files, slp207ServicePath, "", true); len(got) != 0 {
				t.Fatalf("multiline serializable managed transaction: want 0 findings, got %+v", got)
			}
		})
	}
}

func TestSLP207_ManagedDrizzle_MixedHunkStillReportsUnmanaged(t *testing.T) {
	knexLine := slp207LineOf(t, slp207MixedService, "knex.transaction()")
	t.Run("managed excluded, unmanaged reported", func(t *testing.T) {
		got := slp207Scan(t, slp207PlainFiles(slp207MixedService), slp207ServicePath, "", true)
		if len(got) != 1 || got[0].Line != knexLine {
			t.Fatalf("want exactly the knex begin at line %d, got lines %v", knexLine, slp207Lines(got))
		}
	})
	t.Run("rollback inside managed callback does not vouch for sibling", func(t *testing.T) {
		svc := slp207MustReplace(t, slp207MixedService,
			"    await tx.insert(users).values({ name });\n",
			"    await tx.insert(users).values({ name });\n    if (!name) await tx.rollback();\n")
		knexLine := slp207LineOf(t, svc, "knex.transaction()")
		got := slp207Scan(t, slp207PlainFiles(svc), slp207ServicePath, "", true)
		if len(got) != 1 || got[0].Line != knexLine {
			t.Fatalf("want exactly the knex begin at line %d, got lines %v", knexLine, slp207Lines(got))
		}
	})
	t.Run("diff-only keeps both", func(t *testing.T) {
		if got := slp207Scan(t, slp207PlainFiles(slp207MixedService), slp207ServicePath, "", false); len(got) != 2 {
			t.Fatalf("diff-only: want 2 findings, got %v", slp207Lines(got))
		}
	})
}

// ---------------------------------------------------------------------------
// Negative controls: each is one change from an accepted case and must keep
// the SLP207 finding.
// ---------------------------------------------------------------------------

func TestSLP207_ManagedDrizzle_UnsafeControlsStillReported(t *testing.T) {
	type tc struct {
		name   string
		files  func(t *testing.T) map[string]string
		target string
		diff   string // optional diff-side override
	}
	plain := func(svc string) func(t *testing.T) map[string]string {
		return func(t *testing.T) map[string]string { return slp207PlainFiles(svc) }
	}
	svcWith := func(old, repl string) func(t *testing.T) map[string]string {
		return func(t *testing.T) map[string]string {
			return slp207PlainFiles(slp207MustReplace(t, slp207ManagedService, old, repl))
		}
	}
	dbWith := func(old, repl string) func(t *testing.T) map[string]string {
		return func(t *testing.T) map[string]string {
			f := slp207PlainFiles(slp207ManagedService)
			f[slp207DbPath] = slp207MustReplace(t, slp207PlainDbModule, old, repl)
			return f
		}
	}
	scopedWith := func(old, repl string) func(t *testing.T) map[string]string {
		return func(t *testing.T) map[string]string {
			f := slp207ScopedFiles(slp207ManagedService)
			f[slp207ScopedPath] = slp207MustReplace(t, slp207ScopedExecutorModule, old, repl)
			return f
		}
	}
	scopedDbWith := func(old, repl string) func(t *testing.T) map[string]string {
		return func(t *testing.T) map[string]string {
			f := slp207ScopedFiles(slp207ManagedService)
			f[slp207DbPath] = slp207MustReplace(t, slp207ScopedDbModule, old, repl)
			return f
		}
	}
	cases := []tc{
		// Receiver binding and provenance.
		{name: "receiver shadowed by local const", files: svcWith(
			"export async function activateUser(name: string) {\n",
			"export async function activateUser(name: string) {\n  const db = legacyDatabase();\n")},
		{name: "receiver shadowed by parameter", files: svcWith(
			"export async function activateUser(name: string) {", "export async function activateUser(name: string, db: any) {")},
		{name: "receiver reassigned in importing file", files: svcWith(
			"export async function activateUser", "db.transaction = async (cb: any) => cb(db);\nexport async function activateUser")},
		{name: "receiver from non-relative alias", files: svcWith(`from "../db"`, `from "@server/db"`)},
		{name: "receiver default import", files: svcWith(`import { db } from "../db";`, `import db from "../db";`)},
		{name: "receiver module is not Drizzle", files: dbWith(
			`export const db = drizzle(pool);`, `export const db = knex({ client: "pg" });`)},
		{name: "receiver from unverified Drizzle adapter", files: dbWith(
			`"drizzle-orm/node-postgres"`, `"drizzle-orm/custom-driver"`)},
		{name: "transaction member overridden in db module", files: dbWith(
			"export const db = drizzle(pool);\n", "export const db = drizzle(pool);\ndb.transaction = async (cb: any) => cb(db);\n")},
		{name: "transaction redefined via Object.defineProperty", files: dbWith(
			"export const db = drizzle(pool);\n", "export const db = drizzle(pool);\nObject.defineProperty(db, \"transaction\", { value: () => null });\n")},
		{name: "drizzle shadowed in db module", files: dbWith(
			"export const pool", "const drizzle = (p: any) => fakeDatabase(p);\nexport const pool")},
		// Scoped proxy verification.
		{name: "proxy rewrites transaction", files: scopedWith(
			"const value = Reflect.get(current, property, current);",
			"const value = property === \"transaction\" ? async (cb: any) => cb(current) : Reflect.get(current, property, current);")},
		{name: "proxy forwards to a different object", files: scopedWith(
			"const current = scope?.executor ?? target;", "const current = scope?.executor ?? fakeExecutor;")},
		{name: "proxy adds a set trap", files: scopedWith(
			"      return typeof value === \"function\" ? value.bind(current) : value;\n    },\n",
			"      return typeof value === \"function\" ? value.bind(current) : value;\n    },\n    set() {\n      return true;\n    },\n")},
		{name: "factory mutates the proxy", files: scopedWith(
			"  return {\n    executor,", "  Object.assign(executor, { transaction: async (cb: any) => cb(fallback) });\n  return {\n    executor,")},
		{name: "factory installs an arbitrary executor", files: scopedWith(
			"  return {\n    executor,", "  context.enterWith({ executor: replacementExecutor as T, active: true });\n  return {\n    executor,")},
		{name: "scoped executor typed any", files: scopedDbWith(
			"createScopedDatabaseExecutor<DbExecutor>(base)", "createScopedDatabaseExecutor<any>(base)")},
		{name: "scoped executor fallback not Drizzle", files: scopedDbWith(
			"const base = drizzle(process.env.DATABASE_URL!);", "const base = makeDatabase(process.env.DATABASE_URL!);")},
		// Call shape.
		{name: "detached transaction promise", files: svcWith(
			"  await db.transaction(async (tx) => {", "  const pending = db.transaction(async (tx) => {")},
		{name: "failure swallowed by .catch on the call", files: svcWith(
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));\n  });",
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));\n  }).catch(() => null);")},
		{name: "options object computed", files: svcWith(
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));\n  });",
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));\n  }, isolation());")},
		{name: "non-callback argument", files: plain(strings.Replace(slp207ManagedService,
			"  await db.transaction(async (tx) => {\n    const [row] = await tx.insert(users).values({ name }).returning();\n    if (!row) throw new Error(\"insert failed\");\n    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));\n  });",
			"  await db.transaction(work);", 1))},
		// Callback containment.
		{name: "try/catch inside callback", files: svcWith(
			"    if (!row) throw new Error(\"insert failed\");",
			"    try {\n      await tx.delete(users);\n    } catch {\n      return;\n    }")},
		{name: "unawaited query on tx", files: svcWith(
			"    await tx.update(users)", "    tx.update(users)")},
		{name: "tx escapes into nested function", files: svcWith(
			"    if (!row) throw new Error(\"insert failed\");", "    setTimeout(() => tx.delete(users), 0);")},
		{name: "tx stored in a variable", files: svcWith(
			"    if (!row) throw new Error(\"insert failed\");", "    const handle = tx;")},
		{name: "outer receiver used inside callback", files: svcWith(
			"    await tx.update(users)", "    await db.update(users)")},
		{name: "sibling import from receiver module used inside callback", files: func(t *testing.T) map[string]string {
			svc := slp207MustReplace(t, slp207ManagedService, `import { db } from "../db";`, `import { db, pool } from "../db";`)
			svc = slp207MustReplace(t, svc, "    await tx.update(users)", "    await pool.query(\"select 1\");\n    await tx.update(users)")
			return slp207PlainFiles(svc)
		}},
		{name: "transaction-control SQL inside callback", files: svcWith(
			"    if (!row) throw new Error(\"insert failed\");", "    await tx.execute(sql.raw(\"COMMIT\"));")},
		{name: "nested transaction inside callback", files: svcWith(
			"    if (!row) throw new Error(\"insert failed\");", "    await tx.transaction(async (sp) => sp.delete(users));")},
		{name: "then handler inside callback", files: svcWith(
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id));",
			"    await tx.update(users).set({ active: true }).where(eq(users.id, row.id)).then(ok, () => null);")},
		// Source binding.
		{name: "diff line disagrees with snapshot", files: plain(slp207ManagedService),
			diff: strings.Replace(slp207ManagedService, "  await db.transaction(async (tx) => {", "  await db.transaction(async (tx) => { // diff side", 1)},
		{name: "begin is a different line in snapshot", files: plain(strings.Replace(slp207ManagedService,
			"export async function activateUser", "// moved\nexport async function activateUser", 1)),
			diff: slp207ManagedService},
		{name: "tsx is not recognized", files: func(t *testing.T) map[string]string {
			f := slp207PlainFiles(slp207ManagedService)
			f["server/services/user.tsx"] = slp207ManagedService
			return f
		}, target: "server/services/user.tsx"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := c.target
			if target == "" {
				target = slp207ServicePath
			}
			got := slp207Scan(t, c.files(t), target, c.diff, true)
			if len(got) == 0 {
				t.Fatalf("unsafe variant was exempted; want the SLP207 finding retained")
			}
		})
	}
}

// Unrelated begin forms keep the baseline heuristic unchanged even when a
// snapshot is available.
func TestSLP207_ManagedDrizzle_OtherLanguagesUnchanged(t *testing.T) {
	goSrc := "package h\n\nfunc process(ctx context.Context, db *sql.DB) error {\n\ttx, err := db.Begin(ctx)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = tx.ExecContext(ctx, \"INSERT INTO t VALUES (1)\")\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n"
	files := map[string]string{"handler.go": goSrc}
	with := slp207Scan(t, files, "handler.go", "", true)
	without := slp207Scan(t, files, "handler.go", "", false)
	if len(with) != 1 || len(without) != 1 || with[0].Line != without[0].Line {
		t.Fatalf("go begin must be reported identically with and without a snapshot: with=%v without=%v",
			slp207Lines(with), slp207Lines(without))
	}
}

func TestSLP207_TSLexer(t *testing.T) {
	src := "const a = `x ${b + `y ${c}`} z`;\nconst re = /[/]\\//g; // c\n/* block\n comment */ const d = { e: [1, (2)] };\n"
	s, ok := slp207Lex("a.ts", src)
	if !ok {
		t.Fatal("lexer rejected valid input")
	}
	var idents []string
	for _, tk := range s.toks {
		if tk.kind == slp207TokIdent {
			idents = append(idents, tk.text)
		}
	}
	if got := strings.Join(idents, ","); got != "const,a,b,c,const,re,const,d,e" {
		t.Fatalf("idents = %s", got)
	}
	if last := s.toks[len(s.toks)-1]; last.line != 4 {
		t.Fatalf("line tracking: last token on line %d, want 4", last.line)
	}
	for _, bad := range []string{"const a = 'unterminated\n';", "f(];", "const a = `x ${b`;", "const a = (1;"} {
		if _, ok := slp207Lex("bad.ts", bad); ok {
			t.Fatalf("lexer accepted malformed input %q", bad)
		}
	}
}
