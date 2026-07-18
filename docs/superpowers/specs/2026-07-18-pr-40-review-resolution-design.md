# Diseño de resolución de comentarios del PR #40

## Contexto

El PR #40 (`feat: reset 0.1 contract and canonical persistence`) tiene 14 hilos de revisión sin resolver. Todos siguen aplicando al estado actual; ninguno está obsoleto.

Decisión: resolver los 14 comentarios. No se rechaza ninguno. La única discusión inicial —semántica de mayúsculas en rutas de `Recovery State`— queda resuelta reutilizando `managedPathKey`.

## Enfoque

Agrupar los cambios por causa raíz:

- contrato y documentación;
- validación de estado persistido;
- portabilidad y seguridad de rutas;
- validación YAML;
- pruebas de regresión.

No se crean nuevas abstracciones de rutas ni se amplía el alcance a un endurecimiento general del sistema.

## Contrato y documentación

- En `CONTEXT.md`, definir `Recovery State` como el registro persistido de un fallo de rollback verificado, sin prometer bloqueo de mutaciones posteriores. Mantener persistencia, sanitización y el comportamiento de Dry Run.
- En `UBIQUITOUS_LANGUAGE.md`, eliminar de Relationships las relaciones activas que atribuyen ownership de `Fragment` a `Managed Artifact` y solapamiento de fragmentos a `Ownership Conflict`. Mantener las definiciones generales de esos términos diferidos.
- En `docs/adr/0005-operation-output-logs-and-exit-codes.md`, cambiar el encabezado a sentence case.
- En `docs/superpowers/plans/2026-07-18-contract-reset.md`, cambiar el encabezado a sentence case y añadir `git diff --check main...HEAD` junto a `git diff --check HEAD`.

Estos cambios no implementan bloqueo de recuperación ni materialización de fragmentos.

## Estado persistido

En `internal/install/service.go`, el flujo será:

1. cargar Manifest, Lockfile y Materialization Record;
2. rechazar Lockfile o Record presentes sin Manifest;
3. ejecutar `ValidateCrossDocumentState`;
4. añadir la declaración y evaluar el no-op de `DeclareOnly`;
5. continuar con confianza, resolución y escritura existentes.

Los errores de carga y validación retornan inmediatamente. Estado ausente sigue siendo válido para una declaración nueva.

En `internal/repositorystate/cross_document.go`, eliminar la excepción para Lockfile sin resoluciones. Cada artefacto materializado debe tener una resolución coincidente en fuente, nombre, versión, commit y versión de artefacto.

Pruebas:

- `DeclareOnly` rechaza estado inválido u huérfano;
- un Materialization Record con Lockfile vacío falla validación cruzada;
- estado completamente ausente permite una declaración nueva.

## Rutas y symlinks

En `internal/source/file/source.go`, `validateRelativePath` usará `path.Clean` y `path.IsAbs`. Rechazará rutas vacías, separadores `\\`, absolutas, `.`/`..`, traversal y prefijos de unidad como `C:/outside`, independientemente del sistema anfitrión. Mantendrá rutas relativas slash-normalizadas.

En `internal/repositorystate/recovery.go`, se rechazarán separadores `\\` y la unicidad usará el `managedPathKey` existente. Por tanto, las rutas serán sensibles a mayúsculas en Unix e insensibles en Windows, igual que `MaterializationRecord`.

En `internal/repositorystate/source_reference.go`, cuando la hoja no exista se resolverá el ancestro existente más profundo, se aplicarán sus symlinks y se añadirá el sufijo inexistente antes de calcular la relación con la raíz. Se propagarán errores distintos de `os.ErrNotExist`.

En `internal/repositorystate/manifest_test.go`, la ruta externa de prueba se creará con `t.TempDir()` en lugar de `/tmp/outside`.

## YAML y pruebas de regresión

En `internal/repositorystate/yaml.go`, se conservará la aceptación de tags `!!*` y se rechazarán tags URI no-core normalizados, como `tag:example.com,2026:value`.

Además:

- `recovery_test.go` exigirá que existan ambas claves `path: a` y `path: z` antes de comparar su orden;
- `store_test.go` usará un `RecoveryState` válido al probar una raíz inexistente;
- se añadirá una prueba para tags URI verbatim y se conservarán las pruebas de tags core;
- se añadirán las regresiones de `DeclareOnly`, estado cruzado, rutas portables y ancestros symlink.

## Validación

Ejecutar:

- `gofmt` sobre los archivos Go modificados;
- `go test ./...`;
- `just check`;
- `just check-go`;
- `just check-md`;
- `git diff --check HEAD`;
- `git diff --check main...HEAD`.

## Fuera de alcance

- ciclo de vida de rollback y reparación manual;
- bloqueo runtime por `Recovery State`;
- materialización de fragmentos;
- nueva abstracción compartida de validación de rutas;
- matriz multiplataforma extensa no exigida por estos comentarios.
