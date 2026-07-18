# Resolución de comentarios del PR #40: implementation plan

<!-- markdownlint-disable MD010 MD032 -->

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolver los 14 comentarios de revisión abiertos del PR #40 mediante cambios mínimos de contrato, validación de estado, rutas, YAML y regresiones.

**Architecture:** Mantener los límites actuales. `internal/install` cargará y validará los documentos persistidos antes de cambiar el Manifest; `internal/repositorystate` seguirá siendo dueño de invariantes de estado, rutas persistidas y YAML; `internal/source/file` validará rutas de descriptor con reglas portables. Reutilizar `managedPathKey` y no introducir un validador de rutas compartido nuevo.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, biblioteca estándar `path`, `filepath`, `os`, tests nativos de Go, Markdownlint y `just`.

## Global constraints

- Resolver los 14 comentarios del PR #40; rechazar ninguno.
- No implementar bloqueo runtime ni reparación manual para `Recovery State`.
- No implementar materialización de `Fragment`.
- Validar Lockfile y Materialization Record en `DeclareOnly` antes de `AddDeclaration` y del no-op.
- Usar `managedPathKey` para unicidad de rutas de Recovery State.
- Rechazar separadores `\\`, traversal, rutas absolutas y prefijos de unidad en rutas relativas publicadas.
- Propagar errores de filesystem distintos de `os.ErrNotExist` al resolver ancestros symlink.
- No añadir dependencias ni crear commits sin confirmación explícita del usuario en ese momento.

---

## File map

- Modify `CONTEXT.md`: alinear la definición de `Recovery State` con el alcance 0.1.
- Modify `UBIQUITOUS_LANGUAGE.md`: quitar relaciones activas de fragmentos del glosario 0.1.
- Modify `docs/adr/0005-operation-output-logs-and-exit-codes.md`: corregir encabezado.
- Modify `docs/superpowers/plans/2026-07-18-contract-reset.md`: corregir encabezado y registrar ambos checks de whitespace.
- Modify `internal/install/service.go`: cargar y validar estado persistido en ambos modos de instalación.
- Modify `internal/install/service_test.go`: cubrir `DeclareOnly` con estado inválido y huérfano.
- Modify `internal/repositorystate/cross_document.go`: eliminar la excepción para Lockfile vacío.
- Modify `internal/repositorystate/cross_document_test.go`: cubrir Record sin resolución correspondiente.
- Modify `internal/repositorystate/source_reference.go`: resolver el ancestro symlink existente más profundo para hojas ausentes.
- Modify `internal/repositorystate/manifest_test.go`: usar directorio temporal externo y cubrir hoja ausente tras symlink.
- Modify `internal/repositorystate/recovery.go`: validar separadores y usar `managedPathKey`.
- Modify `internal/repositorystate/recovery_test.go`: cubrir separadores y exigir claves YAML presentes.
- Create `internal/repositorystate/recovery_windows_test.go`: cubrir colisiones de mayúsculas en Windows.
- Modify `internal/repositorystate/store_test.go`: usar Recovery State válido en prueba de raíz ausente.
- Modify `internal/repositorystate/yaml.go`: rechazar tags URI no-core normalizados.
- Modify `internal/repositorystate/yaml_test.go`: cubrir tags URI verbatim y tags core.
- Modify `internal/source/file/source.go`: validar rutas de descriptor con `path` y prefijos de unidad.
- Modify `internal/source/file/source_test.go`: cubrir rutas relativas portables.

### Task 1: Alinear documentación y plan

**Files:**
- Modify: `CONTEXT.md:263-265`
- Modify: `UBIQUITOUS_LANGUAGE.md:81`
- Modify: `docs/adr/0005-operation-output-logs-and-exit-codes.md:1`
- Modify: `docs/superpowers/plans/2026-07-18-contract-reset.md:1,151-153`

**Interfaces:**
- Consumes: decisiones aprobadas en `docs/superpowers/specs/2026-07-18-pr-40-review-resolution-design.md`.
- Produces: documentación 0.1 sin promesa de bloqueo runtime, sin relación activa de fragmentos y con ambos checks de whitespace.

- [ ] **Step 1: Actualizar el contrato canónico**

  En `CONTEXT.md`, reemplazar la definición actual por:

  ```markdown
  **Recovery State**:
  The explicit failure state recorded atomically when verified best-effort rollback cannot restore every affected path to its prior observed state. **Dry Run** may report it but never clears it.
  _Avoid_: Silent partial failure, implicit dirty state
  ```

  En `UBIQUITOUS_LANGUAGE.md`, eliminar de Relationships el bullet que afirma que un `Managed Artifact` posee `Fragments`; mantener las definiciones generales de `Fragment` y `Ownership Conflict` como vocabulario diferido.

- [ ] **Step 2: Corregir encabezados y validación registrada**

  Usar exactamente estos encabezados:

  ```markdown
  # ADR-0005: Operation output and exit codes
  ```

  ```markdown
  # 0.1 contract reset implementation plan
  ```

  En la checklist del plan, conservar las líneas existentes y añadir:

  ```markdown
  - [x] Run `git diff --check HEAD`.
  - [x] Run `git diff --check main...HEAD`.
  ```

- [ ] **Step 3: Ejecutar el check documental**

  Run: `just check-md`

  Expected: exit code `0` and no diagnostics.

### Task 2: Hacer obligatoria la validación cruzada del estado

**Files:**
- Modify: `internal/repositorystate/cross_document.go:5-15`
- Test: `internal/repositorystate/cross_document_test.go`
- Modify: `internal/install/service.go:121-155`
- Test: `internal/install/service_test.go`

**Interfaces:**
- Consumes: `Store.LoadLockfile`, `Store.LoadMaterializationRecord`, `ValidateCrossDocumentState` y sus flags `Present`.
- Produces: `Install` rechaza estado inválido, huérfano o inconsistente en modo normal y `DeclareOnly` antes de declarar o devolver no-op.

- [ ] **Step 1: Escribir la regresión del Record sin resolución**

  Añadir a `cross_document_test.go`:

  ```go
  func TestValidateCrossDocumentStateRejectsRecordWithoutLockResolution(t *testing.T) {
  	version := "sha256:" + strings.Repeat("a", 64)
  	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{
  		Source: SourceIdentity{Type: SourceTypeFile, Locator: "./source"},
  		ResolvedVersion: version,
  		Artifact: "a",
  		ArtifactVersion: "1.0.0",
  		Files: []ManagedFileRecord{{Path: "a", Digest: version}},
  	}}}
  	if err := ValidateCrossDocumentState(Lockfile{}, record); err == nil {
  		t.Fatal("expected missing lockfile resolution rejection")
  	}
  }
  ```

- [ ] **Step 2: Ejecutar la regresión y confirmar fallo**

  Run: `go test ./internal/repositorystate -run '^TestValidateCrossDocumentStateRejectsRecordWithoutLockResolution$' -count=1`

  Expected: FAIL because the current empty-resolution shortcut returns `nil`.

- [ ] **Step 3: Eliminar la excepción de Lockfile vacío**

  En `ValidateCrossDocumentState`, dejar el lookup y la comparación como único camino:

  ```go
  func ValidateCrossDocumentState(lock Lockfile, record MaterializationRecord) error {
  	for _, artifact := range record.Artifacts {
  		resolution, locked, ok := lock.Artifact(ArtifactKey{Source: artifact.Source, Name: artifact.Artifact})
  		if !ok || resolution.ResolvedVersion != artifact.ResolvedVersion || resolution.Commit != artifact.Commit || locked.Version != artifact.ArtifactVersion {
  			return fmt.Errorf("materialized artifact %q does not match a lockfile resolution", artifact.Artifact)
  		}
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Ejecutar la regresión y la suite del paquete**

  Run: `go test ./internal/repositorystate -run 'TestValidateCrossDocumentState' -count=1`

  Expected: PASS.

- [ ] **Step 5: Escribir pruebas de validación temprana en `Install`**

  Añadir estas dos pruebas a `internal/install/service_test.go`:

  ```go
  func TestDeclareOnlyPropagatesLockfileLoadError(t *testing.T) {
  	root := t.TempDir()
  	if err := os.WriteFile(filepath.Join(root, repositorystate.LockfileFileName), []byte("schema_version: ["), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	service, impl := testService(testResolved(testArtifact("a", "a")))
  	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a", DeclareOnly: true})
  	if err == nil {
  		t.Fatal("expected lockfile load error")
  	}
  	if impl.calls != 0 {
  		t.Fatalf("resolve calls = %d, want 0", impl.calls)
  	}
  }

  func TestDeclareOnlyRejectsStateWithoutManifest(t *testing.T) {
  	root := t.TempDir()
  	lock := repositorystate.Lockfile{Resolutions: []repositorystate.Resolution{{
  		Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"},
  		ResolvedVersion: testSnapshotVersion,
  		Artifacts: []repositorystate.ArtifactResolution{{Name: "a", Version: "1.0.0"}},
  	}}}
  	if err := repositorystate.NewStore().WriteLockfile(context.Background(), root, lock); err != nil {
  		t.Fatal(err)
  	}
  	service, impl := testService(testResolved(testArtifact("a", "a")))
  	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a", DeclareOnly: true})
  	if err == nil || !strings.Contains(err.Error(), "require a manifest") {
  		t.Fatalf("Install() error = %v, want missing-manifest validation", err)
  	}
  	if impl.calls != 0 {
  		t.Fatalf("resolve calls = %d, want 0", impl.calls)
  	}
  }
  ```

- [ ] **Step 6: Ejecutar las pruebas nuevas y confirmar fallo**

  Run: `go test ./internal/install -run '^TestDeclareOnly(PropagatesLockfileLoadError|RejectsStateWithoutManifest)$' -count=1`

  Expected: FAIL because `Install` currently skips persisted state loading for `DeclareOnly`.

- [ ] **Step 7: Mover carga y validación antes de declarar**

  En `Install`, después de cargar el Manifest y antes de `declarationFor`, colocar:

  ```go
  var lock repositorystate.Lockfile
  var record repositorystate.MaterializationRecord
  lock, lockPresent, err := service.loadLockfile(ctx, request.Root)
  if err != nil {
  	return Result{}, err
  }
  record, recordPresent, err := service.loadMaterializationRecord(ctx, request.Root)
  if err != nil {
  	return Result{}, err
  }
  if !manifestPresent && (lockPresent || recordPresent) {
  	return Result{}, fmt.Errorf("lockfile and materialization record require a manifest")
  }
  if err := repositorystate.ValidateCrossDocumentState(lock, record); err != nil {
  	return Result{}, err
  }
  ```

  Eliminar la declaración duplicada de `lock`/`record` y todo el bloque condicionado por `!request.DeclareOnly`. Mantener el resto del flujo sin cambios.

- [ ] **Step 8: Ejecutar las pruebas de instalación**

  Run: `go test ./internal/install -run 'Test(DeclareOnly|InstallRejectsState|InstallPropagatesStateLoadErrors)' -count=1`

  Expected: PASS.

### Task 3: Validar rutas de descriptores independientemente del host

**Files:**
- Modify: `internal/source/file/source.go:291-299`
- Test: `internal/source/file/source_test.go`

**Interfaces:**
- Consumes: rutas slash-normalizadas de source y artifact descriptors.
- Produces: `validateRelativePath` rechaza sintaxis absoluta, traversal, backslash y drive prefixes igual en Unix y Windows.

- [ ] **Step 1: Escribir la regresión portable**

  Añadir a `source_test.go`:

  ```go
  func TestValidateRelativePathUsesPortableRules(t *testing.T) {
  	for _, test := range []struct {
  		name  string
  		value string
  		valid bool
  	}{
  		{name: "nested relative", value: "dir/file", valid: true},
  		{name: "windows drive", value: "C:/outside"},
  		{name: "backslash", value: `dir\\file`},
  		{name: "parent", value: "../outside"},
  	} {
  		t.Run(test.name, func(t *testing.T) {
  			err := validateRelativePath(test.value)
  			if test.valid && err != nil {
  				t.Fatalf("validateRelativePath(%q) = %v", test.value, err)
  			}
  			if !test.valid && err == nil {
  				t.Fatalf("validateRelativePath(%q) unexpectedly succeeded", test.value)
  			}
  		})
  	}
  }
  ```

- [ ] **Step 2: Ejecutar la regresión y confirmar fallo**

  Run: `go test ./internal/source/file -run '^TestValidateRelativePathUsesPortableRules$' -count=1`

  Expected: FAIL for `C:/outside` on Unix because `filepath.IsAbs` is host-dependent.

- [ ] **Step 3: Implementar la validación con `path`**

  Añadir `path` a los imports y reemplazar la función por:

  ```go
  func validateRelativePath(value string) error {
  	if value == "" ||
  		strings.Contains(value, "\\") ||
  		path.IsAbs(value) ||
  		(len(value) >= 2 && value[1] == ':' &&
  			((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z'))) {
  		return fmt.Errorf("path must be clean and relative")
  	}
  	clean := path.Clean(value)
  	if clean != value || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
  		return fmt.Errorf("path must be clean and relative")
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Ejecutar pruebas del paquete**

  Run: `go test ./internal/source/file -count=1`

  Expected: PASS.

### Task 4: Resolver symlinks de ancestros con hojas ausentes

**Files:**
- Modify: `internal/repositorystate/source_reference.go:57-67`
- Test: `internal/repositorystate/manifest_test.go`

**Interfaces:**
- Consumes: ruta absoluta limpia y raíz ya resuelta por `NormalizeSourceIdentity`.
- Produces: ruta canónica que conserva el sufijo inexistente después de resolver el ancestro existente más profundo.

- [ ] **Step 1: Escribir la regresión de hoja ausente**

  Añadir a `manifest_test.go`:

  ```go
  func TestNormalizeSourceIdentityCanonicalizesMissingPathThroughSymlink(t *testing.T) {
  	root, external := t.TempDir(), t.TempDir()
  	if err := os.Symlink(external, filepath.Join(root, "linked")); err != nil {
  		t.Skipf("create symlink: %v", err)
  	}
  	got, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: "linked/missing"})
  	want := filepath.ToSlash(filepath.Join(external, "missing"))
  	if err != nil || got.Locator != want {
  		t.Fatalf("NormalizeSourceIdentity() = %#v, %v, want %q", got, err, want)
  	}
  }
  ```

- [ ] **Step 2: Ejecutar la regresión y confirmar fallo**

  Run: `go test ./internal/repositorystate -run '^TestNormalizeSourceIdentityCanonicalizesMissingPathThroughSymlink$' -count=1`

  Expected: FAIL because the current code keeps the unresolved `root/linked/missing` path when the leaf is absent.

- [ ] **Step 3: Añadir resolución de sufijo inexistente**

  Añadir a `source_reference.go`:

  ```go
  func evalSymlinksWithMissingSuffix(value string) (string, error) {
  	canonical, err := filepath.EvalSymlinks(value)
  	if err == nil {
  		return canonical, nil
  	}
  	if !os.IsNotExist(err) {
  		return "", err
  	}
  	suffix := []string{}
  	candidate := value
  	for {
  		if _, statErr := os.Lstat(candidate); statErr == nil {
  			canonical, evalErr := filepath.EvalSymlinks(candidate)
  			if evalErr != nil {
  				return "", evalErr
  			}
  			for i := len(suffix) - 1; i >= 0; i-- {
  				canonical = filepath.Join(canonical, suffix[i])
  			}
  			return canonical, nil
  		} else if !os.IsNotExist(statErr) {
  			return "", statErr
  		}
  		parent := filepath.Dir(candidate)
  		if parent == candidate {
  			return "", err
  		}
  		suffix = append(suffix, filepath.Base(candidate))
  		candidate = parent
  	}
  }
  ```

  En `NormalizeSourceIdentity`, reemplazar la llamada directa a `filepath.EvalSymlinks(path)` por `evalSymlinksWithMissingSuffix(path)` y retornar cualquier error recibido.

- [ ] **Step 4: Ejecutar pruebas de referencias**

  Run: `go test ./internal/repositorystate -run 'Test(NormalizeSourceIdentity|AcquisitionLocator)' -count=1`

  Expected: PASS.

### Task 5: Endurecer Recovery State y corregir pruebas débiles

**Files:**
- Modify: `internal/repositorystate/recovery.go:32-43`
- Modify: `internal/repositorystate/recovery_test.go`
- Create: `internal/repositorystate/recovery_windows_test.go`
- Modify: `internal/repositorystate/store_test.go:232-239`
- Modify: `internal/repositorystate/manifest_test.go:19-29`

**Interfaces:**
- Consumes: `managedPathKey` existente y `RecoveryState` validado.
- Produces: observaciones root-relative sin backslash, únicas según semántica del sistema y pruebas que alcanzan las rutas de error previstas.

- [ ] **Step 1: Escribir pruebas de backslash y serialización completa**

  En `recovery_test.go`, añadir `Path: "dir\\file"` a los casos inválidos de `TestValidateRecoveryStateRejectsRawErrorAndUnsafeObservation`.

  En `TestRecoveryStateRoundTripsSortedSanitizedObservations`, reemplazar la aserción por:

  ```go
  text := string(data)
  aIndex := strings.Index(text, "path: a")
  zIndex := strings.Index(text, "path: z")
  if !strings.Contains(text, "source: file:./source") || aIndex < 0 || zIndex < 0 || aIndex > zIndex {
  	.Fatalf("recovery YAML = %s", data)
  }
  ```

  En `store_test.go`, reemplazar `RecoveryState{}` en `TestStoreWriteReportsMissingRoot` por:

  ```go
  state := RecoveryState{
  	Code: RecoveryCodeRollbackIncomplete,
  	Summary: "rollback incomplete",
  	Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}},
  }
  if err := store.WriteRecoveryState(context.Background(), root, state); err == nil {
  	t.Fatal("expected recovery write failure")
  }
  ```

  En `manifest_test.go`, reemplazar `/tmp/outside` por:

  ```go
  external := t.TempDir()
  out, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: external})
  if err != nil || out.Locator != filepath.ToSlash(external) {
  	t.Fatalf("external = %#v, %v", out, err)
  }
  ```

- [ ] **Step 2: Escribir la regresión específica de Windows**

  Crear `internal/repositorystate/recovery_windows_test.go`:

  ```go
  //go:build windows

  package repositorystate

  import "testing"

  func TestValidateRecoveryStateRejectsCaseAliasPaths(t *testing.T) {
  	state := RecoveryState{
  		Code: RecoveryCodeRollbackIncomplete,
  		Summary: "rollback incomplete",
  		Observations: []RecoveryObservation{
  			{Path: "Folder/File", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent},
  			{Path: "folder/file", Result: RecoveryResultVerificationFailed, ExpectedState: RecoveryExpectedAbsent},
  		},
  	}
  	if err := ValidateRecoveryState(t.TempDir(), state); err == nil {
  		t.Fatal("expected case-alias rejection")
  	}
  }
  ```

- [ ] **Step 3: Ejecutar las pruebas y confirmar los fallos esperados**

  Run: `go test ./internal/repositorystate -run 'Test(RecoveryState|ValidateRecoveryState|StoreWriteReportsMissingRoot|NormalizeSourceIdentityStoresRootRelativeAndExternalAbsoluteLocators)' -count=1`

  Expected: FAIL only for the new backslash validation until implementation is applied; the assertion and missing-root fixes must remain passing.

- [ ] **Step 4: Aplicar validación y unicidad existentes**

  En `ValidateRecoveryState`, incluir `strings.Contains(observation.Path, "\\")` en la condición de rechazo y cambiar la clave del mapa a:

  ```go
  key := managedPathKey(observation.Path)
  if _, ok := paths[key]; ok {
  	return fmt.Errorf("recovery observation path must be unique")
  }
  paths[key] = struct{}{}
  ```

- [ ] **Step 5: Ejecutar pruebas nativas y compilar Windows**

  Run: `go test ./internal/repositorystate -count=1`

  Expected: PASS on the host platform.

  Run:

  ```sh
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  GOOS=windows go test -c -o "$tmpdir/repositorystate.test" ./internal/repositorystate
  ```

  Expected: exit code `0`, confirming compilation of `recovery_windows_test.go` and Windows path code.

### Task 6: Rechazar tags YAML URI no-core

**Files:**
- Modify: `internal/repositorystate/yaml.go:59-61`
- Modify: `internal/repositorystate/yaml_test.go`

**Interfaces:**
- Consumes: `yaml.Node.Tag` normalizado por `yaml.v3`.
- Produces: tags core `!!*` aceptados; tags custom `!custom` y URI `tag:example.com,...` rechazados.

- [ ] **Step 1: Añadir pruebas de URI y tag core**

  Añadir al mapa de `TestDecodeStrictYAMLRejectsUnsafeSyntax`:

  ```go
  "verbatim tag URI": "schema_version: 1\nname: !<tag:example.com,2026:value> one\n",
  ```

  Añadir una prueba de aceptación:

  ```go
  func TestDecodeStrictYAMLAcceptsCoreTag(t *testing.T) {
  	var got yamlTestDocument
  	if err := decodeStrictYAML([]byte("schema_version: 1\nname: !!str one\n"), &got); err != nil {
  		t.Fatalf("decodeStrictYAML() rejected core tag: %v", err)
  	}
  }
  ```

- [ ] **Step 2: Ejecutar la prueba de URI y confirmar fallo**

  Run: `go test ./internal/repositorystate -run 'TestDecodeStrictYAML' -count=1`

  Expected: FAIL because `yaml.v3` normalizes the verbatim URI without a leading `!`.

- [ ] **Step 3: Rechazar URI normalizada no-core**

  Cambiar la condición de tags en `validateYAMLNode` a:

  ```go
  if (strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!")) || strings.HasPrefix(node.Tag, "tag:") {
  	return fmt.Errorf("custom YAML tags are not supported")
  }
  ```

- [ ] **Step 4: Ejecutar la suite YAML**

  Run: `go test ./internal/repositorystate -run 'TestDecodeStrictYAML|TestEncodeYAML' -count=1`

  Expected: PASS.

### Task 7: Formatear, validar y preparar cierre de revisión

**Files:**
- Verify all files modified by Tasks 1–6.

**Interfaces:**
- Consumes: cambios implementados y pruebas de cada causa raíz.
- Produces: branch validada contra working tree y rango completo `main...HEAD`.

- [ ] **Step 1: Formatear Go**

  Run: `gofmt -w internal/install/service.go internal/install/service_test.go internal/repositorystate/cross_document.go internal/repositorystate/cross_document_test.go internal/repositorystate/source_reference.go internal/repositorystate/manifest_test.go internal/repositorystate/recovery.go internal/repositorystate/recovery_test.go internal/repositorystate/recovery_windows_test.go internal/repositorystate/store_test.go internal/repositorystate/yaml.go internal/repositorystate/yaml_test.go internal/source/file/source.go internal/source/file/source_test.go`

  Expected: exit code `0`.

- [ ] **Step 2: Ejecutar las pruebas completas**

  Run: `go test ./...`

  Expected: PASS.

- [ ] **Step 3: Ejecutar checks del repositorio**

  Run: `just check`

  Expected: PASS.

  Run: `just check-go`

  Expected: PASS.

  Run: `just check-md`

  Expected: PASS.

  Run: `git diff --check HEAD`

  Expected: no output and exit code `0`.

  Run: `git diff --check main...HEAD`

  Expected: no output and exit code `0`.

- [ ] **Step 4: Revisar cobertura contra los 14 hilos**

  Confirmar en el diff que existen cambios para: `CONTEXT.md`, ambos encabezados, el check `main...HEAD`, `DeclareOnly`, Lockfile vacío, las tres pruebas débiles, rutas portables, ancestro symlink, tags URI y relaciones de Fragment.

- [ ] **Step 5: Responder y resolver hilos después de validar**

  Para cada URL de revisión del PR #40, dejar una respuesta breve con el cambio aplicado y la prueba relevante. Marcar el hilo como resuelto solo después de que el diff y los checks pasen. No cerrar hilos antes de verificar el comportamiento.

## Self-review checklist

- [ ] Cada comentario del diseño tiene una tarea concreta.
- [ ] Las tareas de código escriben una regresión antes de modificar la implementación.
- [ ] No hay nuevos helpers de rutas ni dependencias.
- [ ] `DeclareOnly` valida estado antes de declaración y no-op.
- [ ] Las reglas Unix/Windows usan `managedPathKey` existente.
- [ ] No se promete bloqueo runtime de Recovery State.
- [ ] La resolución de symlinks conserva sufijos inexistentes y propaga errores reales.
