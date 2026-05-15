# Backlog Issues — gip

---

## [FEAT-01] Comando `exec` — esecuzione di comandi arbitrari su tutti i repository

**Tipo:** Feature  
**Priorità:** Alta  
**Componente:** `cmd/gip`

### Descrizione

Aggiungere il comando `exec` che permette di eseguire un comando shell arbitrario all'interno della directory di ogni progetto configurato. Il comando viene eseguito in parallelo su tutti i repository, con lo stesso meccanismo di pool/timeout già usato da `pull` e `status`.

### Motivazione

I comandi predefiniti (`status`, `pull`, `fetch`) coprono i casi d'uso comuni, ma gli utenti necessitano spesso di eseguire operazioni non contemplate: `git fetch --prune`, `git checkout main`, `git remote -v`, oppure comandi non-git come `make build` o script personalizzati. Senza `exec`, l'utente deve iterare manualmente su ogni directory.

### Comportamento atteso

```
gip exec -- git fetch --prune
gip exec -- git checkout main
gip exec -- make test
```

- Il separatore `--` è obbligatorio per evitare ambiguità con i flag di gip.
- Il comando viene eseguito nella `local_path` di ogni progetto.
- L'output (stdout + stderr) di ogni progetto viene raccolto e stampato in modo sincronizzato (stesso mutex attuale).
- Vengono rispettati i flag globali `-j`, `-t`, `-q`, `-d`, `-m`.
- Se la directory locale non esiste, il comportamento è identico agli altri comandi: errore o skip se `-m` è attivo.
- Exit code di gip è non-zero se almeno un'esecuzione fallisce.

### Criteri di accettazione

- [ ] `gip exec -- <cmd> [args...]` esegue il comando in ogni `local_path`
- [ ] L'output è sincronizzato (no interleaving)
- [ ] Supporta timeout tramite `-t`
- [ ] Supporta parallelismo tramite `-j`
- [ ] Test unitari: esecuzione corretta, gestione errori, rispetto timeout
- [ ] Test E2E con comando reale (es. `git rev-parse HEAD`)
- [ ] Documentazione aggiornata in README

---

## [FEAT-02] Comando `fetch` — esecuzione di `git fetch` su tutti i repository

**Tipo:** Feature  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Aggiungere il comando `fetch` che esegue `git fetch --all --prune` su ogni repository configurato, in parallelo. A differenza di `pull`, non integra le modifiche nel working tree locale: aggiorna solo i riferimenti ai remote.

### Motivazione

Molti utenti preferiscono separare il fetch (aggiornamento dei remote) dal merge/rebase (integrazione locale). Con `pull` questo non è possibile senza modificare il comportamento globale. `fetch` è un'operazione sicura (non modifica il working tree) e quindi adatta a essere eseguita frequentemente su tutti i repo senza rischi.

### Comportamento atteso

```
gip fetch
gip fetch -j 8
gip fetch -t 60
```

- Esegue `git fetch --all --prune` in ogni `local_path`.
- Rispetta `pull_policy: never` — i repository con questa policy vengono saltati.
- Rispetta i flag globali `-j`, `-t`, `-q`, `-d`, `-m`.
- L'output mostra quali remote sono stati aggiornati o se non ci sono novità.

### Criteri di accettazione

- [ ] `gip fetch` esegue `git fetch --all --prune` in parallelo
- [ ] I repo con `pull_policy: never` vengono saltati con messaggio esplicito
- [ ] Supporta `-j` e `-t`
- [ ] Test unitari e E2E
- [ ] Documentazione aggiornata

---

## [FEAT-03] Tag e gruppi di repository — filtrare i progetti per etichetta

**Tipo:** Feature  
**Priorità:** Alta  
**Componente:** `lib/core`, `cmd/gip`

### Descrizione

Introdurre il campo opzionale `tags` nella configurazione dei progetti. Tutti i comandi (status, pull, fetch, exec, branch, list) devono accettare il flag `--tag` per operare solo sui progetti che corrispondono al/ai tag specificati.

### Motivazione

Chi gestisce molti repository (decine o centinaia) li organizza per contesto: lavoro, personale, per cliente, per tecnologia. Eseguire `gip pull` su tutto quando si vuole aggiornare solo i repo di lavoro è inefficiente e potenzialmente rischioso. I tag risolvono questo senza richiedere file di configurazione multipli.

### Modifica al modello dati

```yaml
# Nuovo campo opzionale
- name: frontend
  repository: https://github.com/org/frontend.git
  local_path: ~/projects/frontend
  tags: [work, js, client-acme]

- name: dotfiles
  repository: https://github.com/user/dotfiles.git
  local_path: ~/dotfiles
  tags: [personal]
```

### Comportamento atteso

```
gip status --tag work
gip pull --tag js,client-acme    # OR logico tra tag multipli
gip exec --tag personal -- git log --oneline -5
```

- `--tag` accetta uno o più tag separati da virgola (logica OR: include i progetti che hanno almeno uno dei tag indicati).
- Se `--tag` non è specificato, il comportamento rimane invariato (tutti i progetti).
- Un progetto senza `tags` non viene mai incluso quando `--tag` è attivo.

### Criteri di accettazione

- [ ] Campo `tags` (lista di stringhe) aggiunto a `gipProject`, parsing YAML e JSON
- [ ] Flag `--tag` aggiunto a tutti i comandi
- [ ] Logica OR per tag multipli separati da virgola
- [ ] `gip list` mostra la colonna `TAGS`
- [ ] Test per parsing configurazione con tags
- [ ] Test per filtro con tag singolo, multiplo, nessun match
- [ ] Documentazione aggiornata con esempi

---

## [FEAT-04] Comando `branch` — visualizzare il branch corrente di ogni repository

**Tipo:** Feature  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Aggiungere il comando `branch` che esegue `git rev-parse --abbrev-ref HEAD` in ogni repository e mostra il branch corrente in formato tabellare.

### Motivazione

Prima di un `gip pull` è fondamentale sapere su quale branch si trovano i propri repository. Attualmente l'unico modo è eseguire `gip exec -- git branch --show-current`, ma questo richiede la feature FEAT-01 e produce output verboso. Un comando dedicato fornisce una vista immediata e leggibile.

### Comportamento atteso

```
gip branch
```

Output:
```
NAME           BRANCH               PATH
frontend       feature/login        ~/projects/frontend
backend        develop              ~/projects/backend
mobile         main                 ~/projects/mobile
infra          hotfix/db-timeout    ~/work/infra
```

- Le righe con branch diverso da `main`/`master`/`develop` sono evidenziate (es. colore diverso) per attirare l'attenzione.
- Se il repository è in stato detached HEAD, mostra il commit hash abbreviato con un indicatore visivo (es. `(detached) abc1234`).
- Rispetta `-j`, `-t`, `-q`, `-d`, `-m`.

### Criteri di accettazione

- [ ] `gip branch` mostra nome progetto, branch corrente, local_path
- [ ] Gestisce detached HEAD senza panic
- [ ] Highlight visivo per branch non-default
- [ ] Formato tabellare allineato
- [ ] Test unitari e E2E
- [ ] Documentazione aggiornata

---

## [FEAT-05] Comando `init` — inizializzazione interattiva della configurazione

**Tipo:** Feature  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Aggiungere il comando `gip init [directory]` che scansiona ricorsivamente una directory alla ricerca di repository Git (presenza di `.git/`), rileva il remote `origin`, e genera automaticamente il file di configurazione `~/.gip` in formato YAML.

### Motivazione

Il principale ostacolo all'adozione di gip è la creazione manuale del file di configurazione. Chi ha già decine di repository clonati deve scrivere ogni entry a mano. `gip init` elimina questo attrito e rende gip utilizzabile in pochi secondi.

### Comportamento atteso

```
gip init ~/projects
gip init ~/projects --output ~/.gip-work
gip init .
```

Flusso di esecuzione:

1. Scansiona ricorsivamente la directory specificata (o `.` se omessa).
2. Per ogni `.git/` trovato, rileva:
   - Nome della directory come `name`
   - Remote `origin` come `repository` (da `git remote get-url origin`)
   - Path assoluto come `local_path`
3. Mostra un riepilogo dei repository trovati.
4. Se `~/.gip` esiste già, chiede conferma prima di sovrascrivere (a meno che `--force` sia presente).
5. Scrive il file YAML nel percorso indicato da `--output` (default: `~/.gip`).

```
Scansione ~/projects...

Trovati 8 repository Git:
  ✓ frontend       https://github.com/org/frontend.git
  ✓ backend        https://github.com/org/backend.git
  ✗ old-archive    (nessun remote origin configurato — saltato)
  ...

~/.gip già esistente. Sovrascrivere? [y/N]: y
Configurazione scritta in ~/.gip
```

Flag:
- `--output <path>` — percorso alternativo per il file generato
- `--force` — sovrascrive senza chiedere conferma
- `--depth <n>` — profondità massima di scansione (default: 5)

### Criteri di accettazione

- [ ] Scansione ricorsiva con profondità configurabile
- [ ] Rilevamento automatico di nome, repository e local_path
- [ ] I repo senza remote origin sono saltati con avviso
- [ ] Conferma interattiva se il file di output esiste già
- [ ] Flag `--force` per sovrascrittura senza conferma
- [ ] Flag `--output` per percorso alternativo
- [ ] Output YAML valido leggibile da gip
- [ ] Test con directory contenenti repo misti (con/senza remote)
- [ ] Documentazione aggiornata

---

## [UX-01] Output strutturato con riepilogo finale dell'esecuzione

**Tipo:** Miglioramento UX  
**Priorità:** Alta  
**Componente:** `cmd/gip`, `lib/core`

### Descrizione

Aggiungere alla fine di ogni comando un riepilogo sintetico che mostri il numero di progetti completati con successo, quelli con errori, quelli saltati e la durata totale dell'esecuzione.

### Motivazione

Attualmente l'output termina con l'ultimo risultato parallelo senza alcuna indicazione sull'esito complessivo. Con molti repository è difficile capire a colpo d'occhio se tutto è andato bene o se ci sono stati problemi. Il riepilogo finale trasforma gip da strumento opaco a strumento affidabile.

### Comportamento atteso

Output corrente (nessun riepilogo):
```
[frontend] nothing to commit
[backend]  M src/main.go
[infra]    error: ...
```

Output con riepilogo:
```
[frontend]  OK  — nessuna modifica
[backend]   OK  — 1 file modificato
[infra]     ERR — connection timed out

─────────────────────────────────────────
Completati: 2   Errori: 1   Saltati: 0   Durata: 3.4s
```

- In modalità `--quiet` il riepilogo rimane visibile (è l'unico output significativo).
- In modalità `--json` il riepilogo è incluso nel JSON come campo `summary`.
- Il conteggio `Saltati` include i repo con `pull_policy: never` e quelli mancanti con `-m`.

### Criteri di accettazione

- [ ] Riepilogo stampato al termine di ogni comando (status, statusfull, pull, fetch, exec, branch)
- [ ] Campi: completati OK, errori, saltati, durata
- [ ] In modalità `--quiet` il riepilogo è l'unico output
- [ ] In modalità `--json` è incluso come campo `summary`
- [ ] Test che verificano presenza e correttezza dei conteggi
- [ ] La durata è misurata dall'inizio dell'elaborazione alla fine

---

## [UX-02] Barra di avanzamento durante l'esecuzione parallela

**Tipo:** Miglioramento UX  
**Priorità:** Bassa  
**Componente:** `cmd/gip`

### Descrizione

Mostrare una progress bar testuale aggiornata in tempo reale durante l'esecuzione dei comandi paralleli, indicando quanti progetti sono stati completati sul totale.

### Motivazione

Con molti repository (es. 50+) e operazioni lente (pull, clone, exec), l'utente non riceve feedback sull'avanzamento. Non è chiaro se il processo sta lavorando o si è bloccato. Una progress bar semplice riduce l'incertezza e migliora la percezione delle performance.

### Comportamento atteso

```
Elaborazione... [████████░░░░░░░░░░░░] 8/20 (40%)  — frontend
```

- La barra viene aggiornata ogni volta che un progetto completa.
- Mostra: barra grafica, contatore `N/TOT`, percentuale, nome dell'ultimo progetto completato.
- Viene cancellata (overwrite della riga) al completamento, sostituita dal riepilogo (UX-01).
- Disabilitata automaticamente in modalità `--quiet`, `--json`, o quando stdout non è un TTY (es. redirect a file/pipe).
- Non interferisce con l'output normale dei singoli progetti.

### Criteri di accettazione

- [ ] Progress bar visibile durante elaborazione su TTY
- [ ] Aggiornamento in-place (overwrite riga, non nuova riga a ogni update)
- [ ] Disabilitata se stdout non è TTY, o con `--quiet`, o con `--json`
- [ ] Non mescola la propria riga con l'output dei singoli progetti
- [ ] Test che verificano che non appaia in modalità non-TTY

---

## [UX-03] Ordinamento e priorità visiva degli errori nell'output

**Tipo:** Miglioramento UX  
**Priorità:** Alta  
**Componente:** `cmd/gip`, `lib/core`

### Descrizione

Modificare la presentazione dell'output in modo che i progetti con errori siano evidenziati visivamente e, opzionalmente, raggruppati o elencati in coda separata rispetto ai progetti completati con successo.

### Motivazione

Con output parallelo i messaggi di errore si mescolano ai messaggi di successo nell'ordine di completamento. In una lista di 30 repository, un errore critico può passare inosservato perché sepolto tra righe di output normale. Gli errori devono emergere senza dover scorrere tutto l'output.

### Comportamento atteso

Strategia in due parti:

1. **Colori semantici** (subito implementabile):
   - Successo → verde o neutro
   - Warning (repo mancante, policy skip) → giallo
   - Errore → rosso in evidenza

2. **Sezione errori in coda** (comportamento opzionale con `--errors-last`):
   ```
   [frontend]  OK  — nessuna modifica
   [backend]   OK  — 2 file modificati
   [mobile]    OK  — nessuna modifica

   ── Errori ──────────────────────────────
   [infra]    ERR  — ssh: connect to host git.corp.com: Connection refused
   [archive]  ERR  — timeout dopo 30s
   ```

- I colori rispettano la libreria `clui` già in uso.
- `--errors-last` è un flag opzionale (default: errori inline nell'ordine di completamento).
- In modalità `--json` gli errori hanno un campo `"status": "error"` distinto.

### Criteri di accettazione

- [ ] Gli errori sono stampati in rosso tramite clui
- [ ] I warning (skip, missing) sono in giallo
- [ ] Flag `--errors-last` raggruppa gli errori in una sezione finale
- [ ] In modalità `--json` ogni entry ha campo `"status": "ok"|"error"|"skipped"`
- [ ] Test che verificano i codici colore nell'output
- [ ] Documentazione del flag `--errors-last`

---

## [UX-04] Formato tabellare per il comando `list`

**Tipo:** Miglioramento UX  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Ridisegnare l'output del comando `list` come tabella allineata con colonne, includendo le informazioni più rilevanti per ogni progetto: nome, path locale, policy, provider e (quando implementato) tags.

### Motivazione

L'output attuale di `list` è una semplice lista di nomi o path, insufficiente per avere una visione d'insieme della configurazione. Una tabella permette di verificare a colpo d'occhio path, policy e provider senza dover aprire il file di configurazione.

### Comportamento atteso

```
$ gip list

NAME           LOCAL_PATH                   POLICY    PROVIDER       TAGS
frontend       ~/projects/frontend          default   github.com     work, js
backend        ~/projects/backend           always    github.com     work
dotfiles       ~/dotfiles                   never     github.com     personal
old-lib        /srv/legacy/old-lib          default   gitlab.com     —
```

- Le colonne sono allineate dinamicamente in base alla lunghezza massima dei valori.
- `POLICY` mostra `default` quando il campo è vuoto nella configurazione.
- `PROVIDER` è estratto dall'URL del repository (logica già presente in `repoProvider()`).
- `TAGS` mostra `—` se non configurati (o la colonna è omessa se nessun progetto ha tags).
- In modalità `--json` restituisce un array JSON con tutti i campi.

### Criteri di accettazione

- [ ] Output tabellare con colonne NAME, LOCAL_PATH, POLICY, PROVIDER
- [ ] Colonne allineate dinamicamente
- [ ] Colonna TAGS presente se almeno un progetto ha tags configurati (dipende da FEAT-03)
- [ ] `--json` restituisce array JSON con tutti i campi
- [ ] Test che verificano allineamento e contenuto colonne
- [ ] Test per tabella con nomi di lunghezza variabile

---

## [UX-05] Modalità `--noop` (dry-run) per tutti i comandi

**Tipo:** Miglioramento UX  
**Priorità:** Media  
**Componente:** `cmd/gip`, `lib/core`

### Descrizione

Aggiungere il flag globale `--noop` che, quando attivo, fa eseguire a gip tutta la logica di selezione, validazione e planning senza eseguire effettivamente i comandi Git. L'output descrive cosa verrebbe fatto.

### Motivazione

Prima di eseguire un `gip pull` su 40 repository, o un `gip exec -- git reset --hard origin/main`, l'utente vuole sapere esattamente su quali repository verrebbe operato e con quale comando, senza rischiare modifiche indesiderate. `--noop` è il pattern standard per questo nei tool CLI professionali (`ansible --check`, `terraform plan`, `rsync --dry-run`).

### Comportamento atteso

```
$ gip pull --noop

[DRY-RUN] frontend   → git pull  (in ~/projects/frontend)
[DRY-RUN] backend    → git clone https://github.com/org/backend.git ~/projects/backend  (directory mancante)
[DRY-RUN] archive    → SALTATO  (pull_policy: never)

Nessuna operazione eseguita. Rimuovi --noop per procedere.
```

- Ogni riga mostra il progetto, il comando esatto che verrebbe eseguito, e la directory di lavoro.
- I repository saltati (policy never, missing con `-m`) sono mostrati con la motivazione dello skip.
- Funziona con tutti i comandi: pull, fetch, exec, branch, status.
- Exit code 0 anche in dry-run (non ci sono errori reali).

### Criteri di accettazione

- [ ] Flag `--noop` disponibile come flag globale (tutti i comandi)
- [ ] Nessun comando Git viene eseguito con `--noop`
- [ ] Output descrive comando esatto e directory per ogni progetto
- [ ] I repo saltati mostrano la motivazione dello skip
- [ ] Exit code 0 con `--noop`
- [ ] Test che verificano che nessun processo sia avviato
- [ ] Documentazione con esempi d'uso

---

## [UX-06] Auto-rilevamento del file di configurazione

**Tipo:** Miglioramento UX  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Modificare la logica di ricerca del file di configurazione per seguire una cascata di percorsi, cercando prima un file locale (`.gip` nella directory corrente), poi il file utente (`~/.gip`), prima di fallire con errore.

### Motivazione

Il pattern di auto-discovery della configurazione dalla directory corrente è consolidato in molti tool (git, npm, docker-compose). Permette di avere configurazioni per-progetto o per-contesto senza dover specificare `-f` ogni volta, e abilita workflow come mantenere un `.gip` diverso per ogni workspace (lavoro, personale, cliente).

### Comportamento atteso

Cascata di ricerca (in ordine di priorità):

1. Valore del flag `-f/--file` (se specificato)
2. Variabile d'ambiente `GIP_FILE` (se impostata)
3. `.gip` nella directory corrente (`./`)
4. `.gip` nella home dell'utente (`~/.gip`)
5. Errore: nessun file di configurazione trovato

```
$ cd ~/work
$ ls .gip         # esiste
$ gip status      # usa ~/work/.gip automaticamente

$ cd ~/personal
$ gip status      # non trovato localmente, usa ~/.gip

$ GIP_FILE=/tmp/test.yaml gip list   # usa il path dalla variabile d'ambiente
```

- In modalità `--debug` mostrare quale file di configurazione è stato trovato e usato.

### Criteri di accettazione

- [ ] Ricerca in cascata: flag `-f` → env `GIP_FILE` → `./.gip` → `~/.gip`
- [ ] Supporto variabile d'ambiente `GIP_FILE`
- [ ] In modalità `--debug` il path del file usato è loggato
- [ ] Test per ogni livello della cascata
- [ ] Test che verifica la priorità corretta (flag > env > locale > home)
- [ ] Documentazione aggiornata con la logica di lookup

---

## [UX-07] Output `--json` per integrazione con script e pipeline

**Tipo:** Miglioramento UX  
**Priorità:** Media  
**Componente:** `cmd/gip`

### Descrizione

Aggiungere il flag globale `--json` che produce output in formato JSON strutturato invece del formato testuale colorato, facilitando l'integrazione di gip con script shell, pipeline CI/CD e tool di monitoring.

### Motivazione

gip è uno strumento da riga di comando usato spesso in contesti automatizzati. Il parsing dell'output testuale è fragile (dipende da versione e locale). Un output JSON stabile e documentato permette di costruire script robusti, dashboard, notifiche Slack, ecc. senza dipendere dalla formattazione dell'output.

### Struttura JSON per comando `status`

```json
{
  "command": "status",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "frontend",
      "local_path": "/home/user/projects/frontend",
      "status": "ok",
      "has_changes": true,
      "changes": " M src/App.tsx\n M src/index.css",
      "error": null
    },
    {
      "name": "archive",
      "local_path": "/home/user/archive",
      "status": "skipped",
      "reason": "pull_policy: never",
      "error": null
    },
    {
      "name": "broken",
      "local_path": "/home/user/broken",
      "status": "error",
      "error": "exit status 128: not a git repository"
    }
  ],
  "summary": {
    "total": 3,
    "ok": 1,
    "errors": 1,
    "skipped": 1,
    "duration_ms": 1240
  }
}
```

- Il formato è consistente tra tutti i comandi (stessa struttura di envelope).
- In modalità `--json` non viene usato nessun colore ANSI nell'output.
- `--json` e `--quiet` sono mutualmente esclusivi; `--json` ha precedenza.
- Exit code rimane non-zero in caso di errori (comportamento invariato).

### Criteri di accettazione

- [ ] Flag `--json` disponibile come flag globale
- [ ] Output JSON valido e parseable con `jq`
- [ ] Struttura consistente tra status, pull, fetch, exec, branch, list
- [ ] Nessun output ANSI con `--json`
- [ ] Campo `summary` con conteggi e durata
- [ ] `--json` ha precedenza su `--quiet`
- [ ] Test che verificano validità e struttura del JSON
- [ ] Documentazione della struttura JSON per ogni comando

---

## [UX-08] Avvisi di configurazione all'avvio

**Tipo:** Miglioramento UX  
**Priorità:** Bassa  
**Componente:** `lib/core`, `cmd/gip`

### Descrizione

Eseguire una validazione del file di configurazione al momento del caricamento e stampare avvisi (warning, non errori fatali) per ogni anomalia rilevata, prima di procedere con il comando richiesto. Gli avvisi sono soppressi in modalità `--quiet`.

### Motivazione

Attualmente i problemi di configurazione (campo `repository` vuoto, `pull_policy` non valida) vengono scoperti solo quando il comando fallisce su quel progetto, mescolati all'output di tutti gli altri. Un controllo preventivo all'avvio centralizza i problemi e li rende immediatamente visibili, riducendo il tempo di debug.

### Anomalie da rilevare

| Condizione | Messaggio di avviso |
|---|---|
| `repository` vuoto o mancante | `WARN [nome]: campo 'repository' mancante` |
| `pull_policy` non valida (≠ never/always/"") | `WARN [nome]: pull_policy non valida: "xxx" (valori: never, always)` |
| `local_path` vuoto o mancante | `WARN [nome]: campo 'local_path' mancante` |
| `name` vuoto o mancante | `WARN [indice]: campo 'name' mancante (progetto #N)` |
| Provider non riconoscibile da URL | `WARN [nome]: impossibile determinare il provider da "url"` |
| Duplicati nel campo `name` | `WARN: nome duplicato rilevato: "nome" (progetti #N e #M)` |

### Comportamento atteso

```
$ gip status

WARN [old-lib]: campo 'repository' mancante
WARN [test]:    pull_policy non valida: "sometimes" (valori accettati: never, always)
WARN:           nome duplicato rilevato: "backend" (voci #3 e #7)

[frontend]  OK  — nessuna modifica
...
```

- Gli avvisi sono stampati prima dell'output dei comandi, in giallo.
- Con `--quiet` gli avvisi sono soppressi.
- Con `--json` gli avvisi sono inclusi nel campo `"warnings": [...]` dell'envelope.
- La presenza di avvisi non blocca l'esecuzione.
- Un progetto con `local_path` mancante viene saltato (come oggi), ma ora con avviso esplicito.

### Criteri di accettazione

- [ ] Validazione eseguita dopo il parsing del file di configurazione
- [ ] Avvisi per tutti i casi elencati nella tabella
- [ ] Avvisi soppressi con `--quiet`
- [ ] Avvisi inclusi in `"warnings"` con `--json`
- [ ] Avvisi in colore giallo (clui)
- [ ] Avvisi non bloccano l'esecuzione
- [ ] Test per ogni tipo di anomalia
- [ ] Test che verificano assenza avvisi con configurazione corretta
