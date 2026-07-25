# DiffBeacon — Especificación inicial del producto

**Producto:** DiffBeacon  
**Comando:** `diffbeacon`  
**Repositorio:** `diffbeacon`  
**Versión del documento:** 0.1-draft  
**Fecha:** 2026-07-25  
**Estado:** Base inicial para un flujo de Specification-Driven Development (SDD)

---

## 1. Propósito del documento

Este documento define la primera especificación funcional y técnica de DiffBeacon. Su objetivo es servir como base confiable para:

- Validar el alcance del producto antes de implementar.
- Identificar decisiones todavía abiertas.
- Descomponer el trabajo en features, historias y tareas.
- Derivar criterios de aceptación y pruebas.
- Evitar que la implementación convierta accidentalmente a DiffBeacon en un cliente Git generalista.

Las decisiones se clasifican de la siguiente manera:

- **Confirmado:** acordado y parte del producto.
- **Propuesto:** recomendación inicial que debe validarse durante el flujo SDD.
- **Abierto:** requiere una decisión posterior.
- **Fuera de alcance:** no forma parte de la versión inicial.

---

## 2. Resumen ejecutivo

DiffBeacon es una TUI liviana, local y orientada a teclado para observar y revisar en tiempo real los cambios de un repositorio Git.

La aplicación se ejecuta dentro de un proyecto, detecta el repositorio correspondiente y muestra sus archivos:

- Staged.
- Unstaged.
- Untracked.

Al seleccionar una entrada, DiffBeacon muestra el cambio correspondiente y permite alternar:

- Diff inline o side-by-side.
- Solo cambios o archivo completo con cambios resaltados.

La aplicación actualiza automáticamente la lista y el diff visible cuando cambia el working tree o el índice de Git. También permite hacer stage y unstage de archivos completos, hunks individuales o el conjunto completo de cambios, sin abandonar la TUI.

DiffBeacon es agnóstico respecto del origen de los cambios. No intenta determinar si fueron producidos por una persona, un agente de IA, un editor, una tarea automatizada o cualquier otro proceso.

---

## 3. Visión del producto

### 3.1 Problema

Durante el desarrollo —especialmente cuando agentes de IA trabajan en segundo plano— los archivos pueden cambiar rápidamente y algunos cambios pueden pasar al staging area antes de que el usuario los revise.

Los clientes Git existentes suelen cubrir muchas operaciones adicionales y pueden introducir ruido, complejidad visual o cambios de contexto. Para revisar el trabajo en curso, el usuario necesita una herramienta enfocada en:

- Saber qué cambió.
- Ver cómo cambió.
- Mantener el contexto del archivo.
- Observar nuevos cambios mientras ocurren.
- Controlar el estado staged/unstaged sin salir de la herramienta.

### 3.2 Propuesta de valor

> DiffBeacon ofrece una vista viva, enfocada y comprensible de los cambios locales de Git.

### 3.3 Principios

1. **Revisión antes que administración:** comprender cambios es la función principal.
2. **Estado Git real:** Git es la fuente de verdad.
3. **Actualización automática:** el usuario no debe refrescar manualmente para ver cambios normales.
4. **Contexto bajo demanda:** se puede pasar de hunks aislados al archivo completo.
5. **Acciones explícitas y seguras:** stage y unstage son centrales; operaciones destructivas no lo son.
6. **Interfaz liviana:** pocos conceptos visibles y navegación predecible.
7. **Agnóstico al autor:** se observan cambios, no procesos ni agentes.
8. **Degradación elegante:** archivos binarios, repositorios inusuales o terminales pequeños no deben romper la aplicación.

---

## 4. Objetivos y no objetivos

### 4.1 Objetivos de la versión inicial

- Detectar el repositorio Git asociado al directorio de ejecución.
- Mostrar cambios staged, unstaged y untracked en secciones diferenciadas.
- Mostrar correctamente un archivo que posee cambios staged y unstaged al mismo tiempo.
- Actualizar automáticamente la interfaz ante cambios en archivos o en el índice.
- Mostrar diffs inline y side-by-side.
- Mostrar solo hunks o el archivo completo con cambios resaltados.
- Mostrar archivos untracked completos.
- Aplicar syntax highlighting cuando sea posible.
- Permitir stage y unstage de archivos, hunks y todos los cambios.
- Mantener una UX rápida y estable durante actualizaciones concurrentes.
- Funcionar sin servicios remotos y sin configuración de agentes.

### 4.2 No objetivos de la versión inicial

- Crear commits.
- Editar mensajes de commit.
- Navegar el historial.
- Administrar branches, tags o remotes.
- Hacer fetch, pull, push, merge, rebase o cherry-pick.
- Resolver conflictos de merge desde la aplicación.
- Descartar cambios del working tree.
- Editar archivos.
- Atribuir cambios a personas o agentes.
- Integrarse obligatoriamente con NucleusOS.
- Revisar pull requests remotos.
- Reemplazar a Git o implementar Git completamente en Go.

---

## 5. Usuarios y escenarios

### 5.1 Usuario principal

Desarrollador que trabaja en un repositorio local y necesita observar o revisar cambios mientras otra herramienta, agente o persona modifica el proyecto.

### 5.2 Escenarios principales

#### Escenario A: observar un archivo en modificación

1. El usuario ejecuta `diffbeacon`.
2. Selecciona un archivo unstaged.
3. Observa su diff.
4. Otro proceso modifica el mismo archivo.
5. DiffBeacon actualiza automáticamente el diff visible.
6. El usuario permanece en el mismo archivo y conserva, cuando sea posible, su posición de revisión.

#### Escenario B: aparece un archivo nuevo

1. DiffBeacon está abierto.
2. Un proceso crea o modifica un archivo previamente limpio.
3. El archivo aparece automáticamente en la sección correspondiente.
4. La selección actual no cambia.

#### Escenario C: el agente hace stage

1. El usuario está observando un archivo unstaged.
2. Otro proceso ejecuta `git add`.
3. DiffBeacon detecta el cambio del índice.
4. El archivo aparece en Staged.
5. Si todavía posee cambios posteriores, también permanece en Unstaged.

#### Escenario D: stage parcial desde DiffBeacon

1. El usuario selecciona un hunk de un archivo unstaged.
2. Ejecuta la acción Stage hunk.
3. Solo ese hunk pasa al índice.
4. DiffBeacon refresca ambas representaciones.
5. El archivo puede aparecer simultáneamente en Staged y Unstaged.

#### Escenario E: revisar un archivo untracked

1. Aparece un archivo nuevo no trackeado.
2. El usuario lo selecciona.
3. DiffBeacon muestra el archivo completo.
4. El usuario puede stagearlo sin salir de la TUI.

---

## 6. Modelo de estado Git

### 6.1 Fuente de verdad

**Confirmado:** DiffBeacon usa el ejecutable nativo de Git. Se asume que `git` está instalado y disponible en `PATH`.

DiffBeacon no mantiene una representación autoritativa independiente. Después de toda acción o señal de cambio, vuelve a consultar a Git.

### 6.2 Ámbitos de cambio

| Ámbito | Base | Destino | Comando conceptual |
|---|---|---|---|
| Staged | `HEAD` | índice | `git diff --cached` |
| Unstaged | índice | working tree | `git diff` |
| Untracked | inexistente | working tree | lectura del archivo completo |

### 6.3 Archivo con doble estado

Un archivo puede tener:

- Una versión en `HEAD`.
- Otra versión en el índice.
- Otra versión en el working tree.

Por lo tanto, el mismo path puede generar dos entradas distintas:

- `path + staged`.
- `path + unstaged`.

La identidad de una selección nunca debe ser solamente el path.

### 6.4 Estados mínimos

- Added.
- Modified.
- Deleted.
- Renamed.
- Copied, cuando Git lo detecte.
- Type changed.
- Untracked.
- Unmerged/conflicted.

### 6.5 Archivos ignorados

**Confirmado:** los archivos ignorados por Git no se muestran por defecto.

### 6.6 Repositorio sin commits

DiffBeacon debe soportar, o degradar explícitamente, repositorios válidos sin `HEAD` inicial.

En ese estado:

- El contenido staged se compara con el árbol vacío.
- Unstage no puede depender exclusivamente de `HEAD`.
- La implementación debe elegir el comando seguro adecuado para quitar paths del índice sin borrar el working tree.

---

## 7. Estructura de la interfaz

### 7.1 Layout principal

```text
┌ Files / Changes ─────────────┬ Diff / File ────────────────────────────────┐
│ STAGED                       │ src/store.go                                │
│ M  src/store.go              │                                             │
│                              │  42 │ func Open(path string) {              │
│ UNSTAGED                     │  43 │-    return oldOpen(path)              │
│ M  src/store.go              │  43 │+    return openRepository(path)      │
│ M  README.md                 │  44 │ }                                     │
│                              │                                             │
│ UNTRACKED                    │                                             │
│ ?  docs/notes.md             │                                             │
├──────────────────────────────┴─────────────────────────────────────────────┤
│ All · Inline · Changes     branch: main     ↑↓ navigate   ? help           │
└────────────────────────────────────────────────────────────────────────────┘
```

El dibujo es ilustrativo; no fija medidas ni colores definitivos.

### 7.2 Panel izquierdo

Responsabilidades:

- Mostrar las entradas agrupadas por Staged, Unstaged y Untracked.
- Mostrar estado de cada archivo.
- Diferenciar visualmente entradas con el mismo path pero distinto ámbito.
- Indicar cantidad de archivos por sección.
- Conservar la selección durante refrescos cuando la identidad siga existiendo.

### 7.3 Panel derecho

Responsabilidades:

- Mostrar el diff o archivo de la entrada seleccionada.
- Permitir scroll vertical.
- Permitir scroll horizontal cuando el contenido exceda el ancho.
- Mostrar números de línea anteriores y/o actuales.
- Resaltar líneas y, cuando sea posible, segmentos modificados dentro de una línea.
- Mostrar un estado vacío o informativo cuando el contenido no pueda representarse como texto.

### 7.4 Barra de estado

Información propuesta:

- Scope o filtro activo.
- Modo inline/split.
- Modo changes/full file.
- Branch actual.
- Estado de actualización.
- Mensaje transitorio de acciones.
- Ayuda compacta de teclas.

### 7.5 Terminal pequeña

**Propuesto:** cuando no haya ancho suficiente para dos paneles:

- Usar una vista de un solo panel.
- Alternar lista/diff con `Enter`, `Esc`, `h/l` o una tecla dedicada.
- Forzar inline si side-by-side no tiene ancho mínimo útil.
- Mostrar una explicación breve si una vista no está disponible.

El umbral exacto queda abierto.

---

## 8. Modos de visualización

Las siguientes dimensiones son independientes:

### 8.1 Scope/filtro

- **All:** muestra las tres secciones.
- **Staged:** muestra solo entradas staged.
- **Unstaged:** muestra unstaged y, según decisión de UX, untracked.
- **Untracked:** filtro específico opcional.

**Abierto:** decidir si Untracked forma parte del filtro Unstaged o posee siempre un filtro separado. En la lista All debe ser una sección diferenciada.

### 8.2 Layout del diff

#### Inline

- Una única secuencia de líneas.
- Eliminaciones y adiciones se muestran en el mismo panel.
- Números de línea anteriores y actuales cuando correspondan.

#### Side-by-side

- Contenido anterior a la izquierda.
- Contenido nuevo a la derecha.
- Bloques alineados.
- Scroll vertical sincronizado.
- Scroll horizontal sincronizado por defecto.

**Propuesto:** permitir desacoplar el scroll horizontal en una versión posterior, no en v0.1.

### 8.3 Densidad de contexto

#### Changes only

- Muestra hunks con contexto limitado.
- Permite navegar entre hunks.
- El número de líneas de contexto debe ser configurable posteriormente.

#### Full file

- Muestra todo el contenido relevante.
- Mantiene indicaciones visuales de líneas agregadas, eliminadas o modificadas.
- Permite saltar entre cambios sin perder el resto del archivo.

### 8.4 Matriz de comportamiento

| Tipo | Changes only | Full file | Inline | Side-by-side |
|---|---|---|---|---|
| Staged | Sí | Sí | Sí | Sí |
| Unstaged | Sí | Sí | Sí | Sí |
| Untracked | Siempre completo | Siempre completo | Sí | Izquierda vacía, derecha completa |
| Deleted | Diff | Contenido anterior completo | Sí | Izquierda completa, derecha vacía |
| Binary | Resumen | No textual | Resumen | Resumen |
| Unmerged | Limitado/informativo | Working tree cuando sea legible | Propuesto | No garantizado |

### 8.5 Archivos untracked

**Confirmado:** se muestran siempre completos.

- El toggle Changes/Full file queda deshabilitado o no cambia el resultado.
- Inline muestra el archivo completo.
- Side-by-side muestra un lado anterior vacío y el archivo completo en el lado nuevo.
- El syntax highlighting se aplica normalmente.
- La UI indica claramente que el archivo es nuevo/untracked.
- No es necesario generar un patch textual para renderizarlo.

### 8.6 Syntax highlighting

- Chroma detecta el lexer por filename/extensión y, cuando sea útil, por contenido.
- Si no existe lexer, se muestra texto plano.
- El resaltado sintáctico no puede ocultar el significado de agregado/eliminado.
- La paleta debe mantener contraste suficiente en terminales claras y oscuras.
- El contenido ANSI proveniente del archivo no debe ejecutarse ni alterar la TUI.

---

## 9. Actualización en tiempo real

### 9.1 Resultado esperado

Cuando se modifica el repositorio:

- Nuevas entradas aparecen automáticamente.
- Entradas limpias desaparecen automáticamente.
- Cambios de stage actualizan las secciones.
- El diff seleccionado se recalcula automáticamente.
- La UI no bloquea mientras Git procesa el nuevo estado.

### 9.2 Estrategia

**Propuesto:** enfoque híbrido:

1. `fsnotify` para respuesta inmediata ante eventos del filesystem.
2. Debounce para agrupar ráfagas de eventos.
3. Reconciliación periódica para recuperar eventos perdidos.
4. Refresh manual disponible como fallback.

### 9.3 Elementos observados

- Directorios relevantes del working tree.
- Índice de Git.
- `HEAD` y referencias relevantes cuando afecten el baseline visible.
- Cambios de ubicación del índice o metadata en worktrees.

La ubicación de `.git` no debe asumirse como un directorio `.git` dentro del root: en worktrees puede ser un archivo que apunta al Git dir real. Debe resolverse mediante Git.

### 9.4 Debounce

Valor inicial propuesto: entre 100 y 250 ms.

Requisitos:

- Varios eventos de un mismo guardado deben producir un solo refresh lógico.
- Un stream continuo de cambios no debe posponer el refresh indefinidamente.
- Debe existir un máximo de espera antes de refrescar.

### 9.5 Reconciliación

Intervalo inicial propuesto: entre 2 y 5 segundos.

**Abierto:** permitir configurar o desactivar este intervalo.

### 9.6 Consistencia y resultados obsoletos

Cada refresh debe tener una generación o revision interna.

- Si finaliza una consulta anterior después de una más reciente, su resultado se descarta.
- Las operaciones Git deben aceptar cancelación mediante `context.Context`.
- La UI nunca debe volver a un estado anterior por una respuesta fuera de orden.

### 9.7 Preservación de contexto

Ante refresh:

- Mantener la selección por `path + scope`.
- Si la entrada cambia de scope, intentar seleccionar su nueva entrada equivalente.
- Si desaparece, elegir la entrada siguiente; si no existe, la anterior.
- Mantener el hunk seleccionado cuando todavía pueda identificarse.
- Mantener la posición vertical aproximada cuando no sea posible mapear el hunk.
- No seleccionar automáticamente un archivo nuevo si el usuario ya está revisando otro.

### 9.8 Objetivo de latencia

**Propuesto:** en un repositorio pequeño o mediano, un cambio debe reflejarse normalmente en menos de 500 ms desde que finaliza el guardado.

Este objetivo no es una garantía para repositorios extremadamente grandes, filesystems remotos o comandos Git lentos.

---

## 10. Stage y unstage

### 10.1 Alcance confirmado

La versión inicial incluye:

- Stage de archivo completo.
- Unstage de archivo completo.
- Stage de hunk.
- Unstage de hunk.
- Stage de todos los cambios.
- Unstage de todos los cambios.

No incluye descarte de cambios del working tree.

### 10.2 Operaciones conceptuales

| Acción | Implementación Git inicial |
|---|---|
| Stage file | `git add -- <path>` |
| Stage all | `git add -A` |
| Unstage file | `git restore --staged -- <path>` o alternativa segura según estado |
| Stage hunk | patch validado + `git apply --cached` |
| Unstage hunk | patch staged validado + `git apply --cached --reverse` |
| Unstage all | comando seguro dependiente de existencia de `HEAD` |

Los comandos definitivos deben manejar repositorios sin commits y versiones de Git soportadas.

### 10.3 Stage de archivos untracked

- Stage file agrega el archivo al índice.
- Al refrescar, la entrada pasa de Untracked a Staged.
- Si la operación termina correctamente, debe mantenerse seleccionado el mismo path en su nuevo scope.

### 10.4 Hunk seleccionado

Para stage/unstage parcial:

- DiffBeacon construye un patch mínimo y válido a partir del modelo de diff.
- Antes de modificar el índice ejecuta una validación equivalente a `git apply --check`.
- La aplicación del patch se hace directamente contra el índice.
- No se delega la interacción a `git add -p`, porque eso rompería el modelo de UI de DiffBeacon.

### 10.5 Protección contra diffs obsoletos

Entre la visualización y la acción, otro proceso puede modificar el archivo o el índice.

Regla:

1. Validar el patch contra el estado actual.
2. Si la validación falla, no intentar reconstruir o aplicar silenciosamente un cambio aproximado.
3. Refrescar el estado.
4. Informar: “El archivo cambió; revisá el hunk actualizado”.

### 10.6 Resultado de una acción

Después de cualquier stage/unstage:

- Ejecutar refresh inmediato, sin esperar al watcher.
- Mostrar confirmación breve.
- Mantener path y contexto cuando sea posible.
- Mostrar stderr resumido si Git falla.
- Permitir ver detalle del error sin corromper la pantalla.

### 10.7 Confirmaciones

**Propuesto:**

- Stage/unstage de archivo o hunk: sin confirmación.
- Stage all/unstage all: confirmación configurable o diálogo breve.
- Acciones destructivas futuras: siempre requieren una política explícita.

---

## 11. Navegación y keybindings

Los keybindings son propuestos y deben validarse con pruebas de UX.

| Tecla | Acción |
|---|---|
| `j` / `↓` | Siguiente entrada o línea |
| `k` / `↑` | Entrada o línea anterior |
| `Tab` | Cambiar foco entre lista y contenido |
| `Enter` | Abrir contenido / entrar al panel en layout compacto |
| `Esc` | Volver al panel anterior o cerrar overlay |
| `[` / `]` | Hunk anterior/siguiente |
| `v` | Alternar inline/side-by-side |
| `f` | Alternar changes only/full file |
| `1` | All |
| `2` | Unstaged |
| `3` | Staged |
| `s` | Stage de selección actual |
| `u` | Unstage de selección actual |
| `S` | Stage all |
| `U` | Unstage all |
| `r` | Refresh manual |
| `/` | Filtrar/buscar archivos |
| `?` | Ayuda completa |
| `q` | Salir |

Puntos abiertos:

- Cómo alternar entre selección de archivo y selección de hunk.
- Si `s/u` actúa sobre hunk cuando hay un hunk activo y sobre archivo en caso contrario.
- Si se usan `h/l` para foco, scroll horizontal o navegación compacta.
- Soporte de mouse.
- Keymap alternativo estilo Emacs.

---

## 12. Requisitos funcionales

### 12.1 Inicialización

**FR-001 — Detección del repositorio**  
Al ejecutar `diffbeacon`, la aplicación debe localizar el repositorio correspondiente al directorio actual o a un path explícito.

**FR-002 — Fuera de un repositorio**  
Si no se encuentra un repositorio, debe mostrar un error claro y salir con código distinto de cero.

**FR-003 — Git no disponible**  
Si `git` no existe o no puede ejecutarse, debe mostrar un diagnóstico claro.

**FR-004 — Repositorio bare**  
La versión inicial debe rechazar repositorios bare con un mensaje explícito, salvo que se decida soportarlos.

### 12.2 Estado

**FR-010 — Estado estructurado**  
La aplicación debe obtener el estado usando una salida de Git adecuada para parsing, preferentemente `--porcelain=v2 -z`.

**FR-011 — Secciones**  
Debe diferenciar Staged, Unstaged y Untracked.

**FR-012 — Doble presencia**  
Debe permitir que el mismo path aparezca en Staged y Unstaged.

**FR-013 — Renames**  
Debe conservar old path y new path cuando Git informe un rename.

**FR-014 — Estado vacío**  
Si no hay cambios, debe mostrar un estado vacío comprensible y continuar observando.

### 12.3 Visualización

**FR-020 — Selección**  
Seleccionar una entrada debe cargar su contenido correspondiente.

**FR-021 — Inline**  
Debe existir una vista inline.

**FR-022 — Side-by-side**  
Debe existir una vista side-by-side con alineación por bloques.

**FR-023 — Changes only**  
Debe existir una vista limitada a hunks.

**FR-024 — Full file**  
Debe existir una vista de archivo completo con cambios resaltados.

**FR-025 — Untracked completo**  
Los archivos untracked deben mostrarse completos en todos los modos.

**FR-026 — Syntax highlighting**  
Debe aplicar syntax highlighting cuando exista un lexer apropiado y fallback a texto plano.

**FR-027 — Binarios**  
Debe detectar contenido binario y mostrar metadata/resumen en lugar de bytes arbitrarios.

**FR-028 — Scroll**  
Debe soportar scroll vertical y horizontal.

**FR-029 — Hunk navigation**  
Debe permitir saltar entre cambios.

### 12.4 Tiempo real

**FR-030 — Aparición automática**  
Un archivo que comienza a cambiar debe aparecer sin refresh manual.

**FR-031 — Actualización del seleccionado**  
Si cambia el archivo seleccionado, su contenido debe actualizarse.

**FR-032 — Cambio del índice**  
Si otro proceso hace stage/unstage, la lista debe reflejarlo.

**FR-033 — Eliminación de entrada limpia**  
Si un archivo deja de tener cambios, debe desaparecer.

**FR-034 — Selección estable**  
La aparición de una nueva entrada no debe desplazar la selección actual.

### 12.5 Stage/unstage

**FR-040 — Stage file**  
Debe permitir stagear el archivo seleccionado.

**FR-041 — Unstage file**  
Debe permitir sacar del índice el archivo seleccionado sin modificar su working tree.

**FR-042 — Stage hunk**  
Debe permitir stagear el hunk seleccionado.

**FR-043 — Unstage hunk**  
Debe permitir sacar del índice el hunk seleccionado.

**FR-044 — Stage all**  
Debe permitir stagear el conjunto completo.

**FR-045 — Unstage all**  
Debe permitir sacar del índice el conjunto completo sin borrar contenido local.

**FR-046 — Acción obsoleta**  
No debe aplicar un hunk si el patch ya no coincide con el estado actual.

### 12.6 Errores

**FR-050 — Error no fatal**  
Un error al cargar un archivo no debe cerrar toda la aplicación.

**FR-051 — Git stderr**  
Los errores de Git deben presentarse de manera breve, con detalle accesible.

**FR-052 — Recuperación**  
Después de un error transitorio, un refresh posterior debe poder recuperar la vista.

---

## 13. Requisitos no funcionales

**NFR-001 — Responsividad**  
La TUI no debe bloquear el procesamiento de teclas mientras ejecuta Git o calcula highlighting.

**NFR-002 — Binario único**  
La distribución debe producir un binario Go autocontenido, salvo la dependencia declarada del ejecutable Git.

**NFR-003 — Sin red**  
La versión inicial no debe requerir acceso a red.

**NFR-004 — Sin shell**  
Los comandos Git deben ejecutarse mediante argumentos estructurados, no concatenando strings para un shell.

**NFR-005 — Paths seguros**  
Los paths deben pasarse después de `--` cuando corresponda y deben soportar espacios, Unicode, guiones iniciales y caracteres especiales.

**NFR-006 — Parsing seguro**  
Las listas de archivos deben usar separación NUL para evitar ambigüedad.

**NFR-007 — Render determinista**  
El mismo estado y dimensiones de terminal deben producir la misma vista.

**NFR-008 — Memoria acotada**  
Archivos grandes no deben causar crecimiento ilimitado ni congelar la TUI.

**NFR-009 — Cancelación**  
Consultas anteriores deben poder cancelarse o descartarse.

**NFR-010 — Compatibilidad Git**  
Debe documentarse una versión mínima de Git.

**NFR-011 — Accesibilidad visual**  
La información no debe depender exclusivamente del color; debe usar símbolos, texto o estilos adicionales.

**NFR-012 — Privacidad**  
No debe enviar código, paths o telemetría a servicios externos.

**NFR-013 — Inicio rápido**  
Objetivo propuesto: primera vista útil en menos de 500 ms para repositorios pequeños/medianos en filesystem local.

---

## 14. Stack tecnológico

### 14.1 Lenguaje

**Confirmado:** Go.

### 14.2 Dependencias

| Componente | Tecnología | Uso |
|---|---|---|
| Runtime TUI | Bubble Tea | Loop de eventos, estado y comandos |
| Componentes | Bubbles | Viewports, listas, ayuda, inputs |
| Estilos | Lip Gloss | Layout, bordes, colores y estilos |
| Highlighting | Chroma | Syntax highlighting |
| Filesystem events | fsnotify | Detección inmediata de cambios |
| Backend VCS | Git nativo | Estado, contenidos, diffs y staging |

### 14.3 Decisión sobre go-git

**Confirmado para la versión inicial:** no usar `go-git` como fuente principal.

Motivos:

- Git nativo es la fuente más compatible con la semántica real del repositorio.
- Evita reproducir edge cases de índices, worktrees, atributos y configuraciones.
- Reduce riesgo durante la primera versión.

---

## 15. Arquitectura propuesta

```text
cmd/diffbeacon
       │
       ▼
internal/app ─────────────── Estado y coordinación
       │
       ├── internal/git ─── Estado, blobs, índice y operaciones
       ├── internal/watch ─ Eventos, debounce y reconciliación
       ├── internal/diff ── Modelo, hunks y alineación
       ├── internal/highlight
       └── internal/ui ──── Layout, input y render
```

### 15.1 `internal/git`

Responsabilidades:

- Detectar root, Git dir y branch.
- Consultar estado.
- Obtener contenido desde `HEAD`.
- Obtener contenido desde el índice.
- Leer contenido del working tree.
- Ejecutar stage/unstage.
- Validar/aplicar patches parciales.
- Normalizar errores de Git.

No debe contener lógica de presentación.

### 15.2 `internal/watch`

Responsabilidades:

- Registrar paths observables.
- Convertir eventos en señales lógicas de refresh.
- Aplicar debounce.
- Ejecutar reconciliación periódica.
- Reconfigurar watchers si aparecen directorios nuevos.

### 15.3 `internal/diff`

Responsabilidades:

- Representar before/after.
- Calcular operaciones por línea.
- Agrupar hunks.
- Calcular cambios inline dentro de líneas cuando sea viable.
- Construir filas alineadas side-by-side.
- Generar patches parciales válidos para staging.

### 15.4 `internal/ui`

Responsabilidades:

- Traducir teclas a intenciones.
- Gestionar foco.
- Renderizar lista, diff, overlays y status.
- Adaptar layout al tamaño de terminal.
- No ejecutar Git directamente.

### 15.5 `internal/app`

Responsabilidades:

- Mantener el modelo de aplicación.
- Coordinar comandos asíncronos.
- Controlar generaciones de refresh.
- Preservar selección y scroll.
- Convertir acciones UI en operaciones de dominio.

---

## 16. Modelo de dominio inicial

```go
type ChangeScope uint8

const (
    ScopeStaged ChangeScope = iota
    ScopeUnstaged
    ScopeUntracked
)

type ChangeKind uint8

const (
    ChangeAdded ChangeKind = iota
    ChangeModified
    ChangeDeleted
    ChangeRenamed
    ChangeCopied
    ChangeTypeChanged
    ChangeUnmerged
)

type ChangeID struct {
    Path  string
    Scope ChangeScope
}

type FileChange struct {
    ID         ChangeID
    OldPath    string
    Kind       ChangeKind
    Binary     bool
    Insertions int
    Deletions  int
}

type ContentRef struct {
    Source ContentSource
    Path   string
}

type FileComparison struct {
    Before ContentRef
    After  ContentRef
}
```

Modelo de diff propuesto:

```go
type DiffModel struct {
    File   FileChange
    Hunks  []Hunk
    Before Document
    After  Document
}

type Hunk struct {
    ID      HunkID
    OldFrom int
    OldLen  int
    NewFrom int
    NewLen  int
    Rows    []DiffRow
}

type DiffRow struct {
    Kind    RowKind
    OldLine *int
    NewLine *int
    OldText string
    NewText string
}
```

La forma definitiva puede cambiar; las propiedades semánticas no.

---

## 17. Uso seguro de Git

### 17.1 Ejecución

- Usar `exec.CommandContext`.
- Usar `git --no-pager`.
- Desactivar color en salida parseada.
- Desactivar external diff donde corresponda.
- No interpretar stdout como ANSI confiable.
- No ejecutar comandos a través de `sh -c`, `bash -c`, PowerShell o equivalentes.

### 17.2 Configuración del usuario

La configuración global/local de Git puede afectar output y comportamiento.

**Propuesto:**

- Respetar configuración semántica relevante, como renames y atributos.
- Neutralizar pagers, color y external diff para parsing estable.
- Documentar cualquier configuración ignorada.

### 17.3 Locks

Git puede fallar por `index.lock` u otra operación concurrente.

DiffBeacon debe:

- No borrar locks.
- Mostrar el error.
- Mantener la UI activa.
- Refrescar posteriormente.

### 17.4 Symlinks

- No seguir symlinks arbitrariamente para watcher o lectura fuera del root sin entender el estado Git.
- Mostrar el contenido lógico que Git considera para el path.
- Evitar escribir sobre el working tree.

---

## 18. Casos límite

La implementación y las pruebas deben cubrir:

1. Repositorio limpio.
2. Un solo archivo modificado.
3. Archivo untracked.
4. Archivo staged.
5. Archivo con cambios staged y unstaged.
6. Archivo eliminado.
7. Archivo renombrado.
8. Archivo vacío agregado.
9. Archivo vacío eliminado.
10. Repositorio sin commits.
11. Path con espacios.
12. Path Unicode.
13. Path que comienza con `-`.
14. Archivo binario.
15. Archivo grande.
16. Línea extremadamente larga.
17. Archivo sin newline final.
18. Cambio de permisos/modo sin cambio textual.
19. Symlink.
20. Submodule modificado.
21. Merge conflict/unmerged.
22. Worktree con `.git` como archivo.
23. Directorio creado mientras la aplicación está abierta.
24. Archivo eliminado mientras se carga.
25. Archivo modificado durante stage hunk.
26. Otro proceso stagea mientras DiffBeacon está abierto.
27. Cambio de branch externo.
28. `git index.lock`.
29. Terminal redimensionada.
30. Terminal demasiado pequeña.

**Abierto:** definir soporte completo, parcial o informativo para submodules y conflictos en v0.1.

---

## 19. Rendimiento y archivos grandes

### 19.1 Riesgos

- `git status` lento en monorepos.
- Highlighting costoso.
- Diffs patológicos.
- Archivos minificados o con líneas enormes.
- Ráfagas continuas de eventos.

### 19.2 Estrategias

- Ejecutar Git fuera del loop de render.
- Cachear contenido por identidad de estado cuando sea seguro.
- Cancelar highlighting obsoleto.
- Renderizar solo el viewport visible.
- Limitar cálculo inline de caracteres por tamaño/tiempo.
- Mostrar una vista degradada para archivos excesivamente grandes.
- Mantener un refresh coalescido.

### 19.3 Límites

**Abierto:** definir:

- Tamaño máximo para full-file highlighting.
- Tamaño máximo para diff inline por caracteres.
- Longitud máxima de línea antes de degradar.
- Política de lectura parcial.

La UI nunca debe fallar silenciosamente; debe indicar cuándo usa una vista degradada.

---

## 20. Manejo de errores y mensajes

### 20.1 Categorías

- Error de inicialización.
- Error transitorio de Git.
- Error de lectura.
- Estado cambió durante una acción.
- Contenido no representable.
- Feature no soportada.

### 20.2 Ejemplos de mensajes

- `No se encontró un repositorio Git desde <path>.`
- `No se pudo ejecutar Git. Verificá que esté instalado y disponible en PATH.`
- `El archivo cambió; revisá el hunk actualizado antes de stagearlo.`
- `Git está usando el índice en otra operación. Se reintentará al refrescar.`
- `Archivo binario: no hay vista textual disponible.`
- `La terminal es demasiado angosta para el modo side-by-side.`

### 20.3 Logs

**Propuesto:**

- Sin logs visibles por defecto.
- Flag `--debug` o variable de entorno para escribir un log local.
- Nunca registrar contenido completo de archivos por defecto.
- Registrar comandos de forma sanitizada, duración y exit code.

---

## 21. CLI inicial

Forma básica:

```text
diffbeacon [path]
```

Opciones propuestas:

```text
--repo <path>          Path explícito
--refresh <duration>   Intervalo de reconciliación
--no-watch             Desactivar fsnotify y usar refresh manual/periódico
--theme <name>         Tema
--debug                Diagnóstico
--version              Versión
--help                 Ayuda
```

**Abierto:** decidir si el path posicional y `--repo` son redundantes.

---

## 22. Configuración

### 22.1 Versión inicial

La aplicación debe funcionar sin archivo de configuración.

### 22.2 Configuración futura/propuesta

- Keybindings.
- Tema.
- Context lines.
- Intervalo de reconciliación.
- Límites para archivos grandes.
- Detección de renames.
- Confirmación de bulk actions.
- Inclusión/exclusión visual de ciertas secciones.

**Abierto:** formato y ubicación del archivo de configuración. TOML es una opción, no una decisión.

---

## 23. Estrategia de pruebas

### 23.1 Unitarias

- Parser de `status --porcelain=v2 -z`.
- Construcción de `ChangeID`.
- Modelo before/after por scope.
- Algoritmo de hunks.
- Alineación side-by-side.
- Preservación de selección.
- Debounce.
- Reducción de eventos a refresh.
- Generación y validación de patches parciales.
- Truncado y cálculo de ancho Unicode.

### 23.2 Integración con Git real

Crear repositorios temporales y ejecutar Git real para:

- Staged/unstaged/untracked.
- Doble estado.
- Stage/unstage file.
- Stage/unstage hunk.
- Repositorio sin commits.
- Renames/deletes.
- Paths especiales.
- Conflictos.
- Worktrees.

No mockear Git en las pruebas que verifican semántica Git.

### 23.3 Golden tests

Usar snapshots/golden files para:

- Render inline.
- Render side-by-side.
- Full file.
- Terminales con distintos tamaños.
- Temas.
- Estados vacíos y errores.

### 23.4 Race y concurrencia

- Ejecutar `go test -race`.
- Simular refreshes fuera de orden.
- Modificar archivos durante carga.
- Ejecutar stage externo durante una acción interna.

### 23.5 Pruebas manuales

- tmux.
- Terminales claras y oscuras.
- Redimensionamiento continuo.
- Repositorios reales pequeños y grandes.
- macOS/Linux y posteriormente Windows según matriz soportada.

---

## 24. Criterios de aceptación de la v0.1

La versión inicial se considera funcionalmente aceptable cuando:

1. `diffbeacon` abre correctamente desde un subdirectorio de un repositorio.
2. Un repositorio limpio muestra estado vacío y reacciona cuando aparece un cambio.
3. Staged, Unstaged y Untracked se muestran diferenciados.
4. Un archivo con doble estado aparece dos veces con diffs correctos.
5. Seleccionar una entrada muestra la comparación correcta para su scope.
6. Inline y side-by-side funcionan para archivos de texto.
7. Changes only y Full file funcionan para staged y unstaged.
8. Untracked siempre muestra el archivo completo.
9. El diff seleccionado se actualiza automáticamente tras guardar.
10. Un archivo nuevo aparece sin alterar la selección actual.
11. Un `git add` externo mueve/duplica correctamente la entrada entre secciones.
12. Stage/unstage de archivo funciona sin modificar indebidamente el working tree.
13. Stage/unstage de hunk aplica solamente el cambio seleccionado.
14. Una acción sobre un hunk obsoleto se rechaza de forma segura.
15. Stage all/unstage all funciona con confirmación según la política elegida.
16. Paths con espacios, Unicode y guion inicial funcionan.
17. Un archivo binario no rompe el render.
18. La UI continúa respondiendo durante refreshes.
19. Salir restaura correctamente el terminal.
20. No se ejecuta ninguna operación de red ni telemetría.

---

## 25. Entregas propuestas

### Milestone 0 — Spike técnico

- Detectar repo.
- Parsear status.
- Listar secciones.
- Ejecutar un diff inline básico.
- Probar refresco asíncrono.
- Probar Bubble Tea + Bubbles + Lip Gloss + Chroma.

### Milestone 1 — Read-only usable

- Lista completa.
- Selección estable.
- Inline.
- Side-by-side.
- Full file.
- Untracked completo.
- Watch + debounce + reconciliación.
- Syntax highlighting.
- Edge cases textuales principales.

### Milestone 2 — Staging

- Stage/unstage file.
- Stage/unstage hunk.
- Stage/unstage all.
- Protección contra estado obsoleto.
- Mensajes y recuperación de errores.

### Milestone 3 — Hardening

- Repositorio sin commits.
- Worktrees.
- Renames, binarios, submodules y conflictos según alcance elegido.
- Archivos grandes.
- Compatibilidad multiplataforma.
- Packaging y documentación.

La división en milestones no cambia el alcance acordado de v0.1; organiza el orden de implementación.

---

## 26. Decisiones confirmadas

1. Nombre provisional/actual: **DiffBeacon**.
2. Binario/comando: `diffbeacon`.
3. Repositorio: `diffbeacon`.
4. Lenguaje: Go.
5. Stack TUI: Bubble Tea, Bubbles y Lip Gloss.
6. Highlighting: Chroma.
7. Watcher: fsnotify con debounce y estrategia de reconciliación.
8. Git nativo como backend.
9. Git instalado es una precondición.
10. Producto agnóstico al origen de los cambios.
11. Secciones staged y unstaged diferenciadas.
12. Archivos untracked visibles.
13. Untracked se muestra siempre completo.
14. Diff inline y side-by-side.
15. Diff-only y full-file.
16. Actualización automática del archivo visible y de la lista.
17. Stage/unstage es parte de la primera versión.
18. Stage/unstage incluye archivos y hunks.
19. No se incluyen operaciones destructivas de descarte en la primera versión.

---

## 27. Preguntas abiertas para el flujo SDD

### Producto/UX

1. ¿Untracked pertenece al filtro Unstaged o tiene filtro propio?
2. ¿Cómo se selecciona un hunk sin complicar la navegación del archivo?
3. ¿Qué comportamiento exacto tienen `s` y `u` según el foco?
4. ¿Se confirma Stage all y Unstage all?
5. ¿Qué información aparece en cada fila de archivo: estado, stats, timestamp?
6. ¿Se muestra un árbol de paths o una lista plana?
7. ¿Hay búsqueda fuzzy en v0.1?
8. ¿Qué tema y paleta son los defaults?
9. ¿Se soporta mouse en v0.1?
10. ¿Cómo se representa visualmente un archivo presente en dos scopes?

### Git

11. ¿Versión mínima de Git?
12. ¿Alcance de submodules?
13. ¿Alcance de merge conflicts?
14. ¿Se respeta la configuración de algoritmo de diff del usuario?
15. ¿Se habilita rename detection siempre o se respeta Git config?
16. ¿Qué comando exacto se usa para unstage en repositorios sin commits?
17. ¿Stage all incluye siempre deletes y untracked mediante `git add -A`?

### Rendimiento

18. ¿Límites de tamaño para highlighting y full file?
19. ¿Intervalos exactos de debounce y reconciliación?
20. ¿Qué repositorios definen el benchmark de rendimiento?

### Plataformas y distribución

21. ¿Linux y macOS primero, o Windows desde v0.1?
22. ¿Instalación mediante Homebrew, Go install, releases o varios métodos?
23. ¿Dónde vive la configuración?

### Calidad

24. ¿Qué terminales forman la matriz oficial?
25. ¿Qué nivel de cobertura se exige?
26. ¿Se harán tests de UX con usuarios antes de congelar keybindings?

---

## 28. Riesgos principales

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Diffs side-by-side mal alineados | UX confusa | Modelo intermedio propio y golden tests |
| Eventos perdidos por watcher | Estado desactualizado | Reconciliación periódica |
| Ráfagas de eventos | Alto uso de CPU | Debounce y coalescing |
| Patch parcial obsoleto | Índice incorrecto | `--check`, generation y refresh |
| Repositorios grandes | UI lenta | Async, cancelación, límites y degradación |
| Config Git modifica output | Parsing inconsistente | Porcelain v2, NUL y flags controlados |
| Scope ambiguo | Mostrar diff incorrecto | Identidad `path + scope` |
| Crecimiento hacia cliente Git completo | Pérdida de foco | No objetivos explícitos |
| Edge cases multiplataforma | Bugs tardíos | Tests de integración y matriz definida |

---

## 29. Definición de “confiable”

En este proyecto, confiable significa:

- La UI representa el estado consultado a Git, no una suposición persistida.
- Nunca se stagea o unstagea un hunk distinto del revisado.
- Un resultado asíncrono viejo no reemplaza uno nuevo.
- Paths especiales no cambian la interpretación de comandos.
- Una operación concurrente de Git genera un error recuperable, no una corrección invasiva.
- La aplicación no borra archivos, locks ni cambios.
- El usuario puede entender en qué scope está revisando y actuando.
- Los fallos de rendering o highlighting no afectan el contenido del repositorio.

---

## 30. Próximo paso recomendado

Usar este documento como entrada del flujo SDD y producir, en orden:

1. Resolución de preguntas abiertas de UX críticas.
2. Especificación de interacción y keybindings.
3. Contrato del adaptador Git.
4. Contrato del modelo de diff.
5. Plan de pruebas de integración.
6. ADRs para watcher, diff engine y staging parcial.
7. Backlog por milestones con criterios de aceptación trazables a `FR-*` y `NFR-*`.

