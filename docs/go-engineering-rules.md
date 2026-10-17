# Règles pour une application Go scalable, maintenable et sécurisée

Synthèse de deux guides, transposée au Go :

- **[DDH]** [Domain-Driven Hexagon](https://github.com/Sairyss/domain-driven-hexagon) (Sairyss) :
  architecture (DDD, hexagonale / ports & adapters, Clean, Secure by Design). Les exemples d'origine
  sont en TypeScript/NestJS, ils sont réécrits ici avec les idiomes Go.
- **[LGWT]** [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests) (Chris James) :
  TDD, conception guidée par les tests, idiomes Go, tests d'acceptation, fakes et contrats.
- **[Ajout]** : un complément Go qui ne vient d'aucun des deux guides, signalé comme tel.

Chaque règle a un numéro (`A3`, `T7`…) pour pouvoir la citer en revue de code.

> **Règle zéro.** Ce sont des recommandations, pas des dogmes [DDH]. Avant d'appliquer un
> pattern, vérifie que son bénéfice vaut la complexité qu'il ajoute (YAGNI, KISS). Pour enfreindre
> une règle, il faut d'abord la comprendre et savoir quel compromis on fait [LGWT, *IO and sorting*].
> Mais « il est plus facile de refactorer un excès de conception que l'absence de conception » [DDH].

---

## Sommaire

1. [Principes généraux](#1-principes-généraux)
2. [Architecture : couches et sens des dépendances](#2-architecture--couches-et-sens-des-dépendances)
3. [Modules et arborescence Go](#3-modules-et-arborescence-go)
4. [Couche domaine](#4-couche-domaine)
5. [Couche application (use cases)](#5-couche-application-use-cases)
6. [Adaptateurs entrants (HTTP, CLI, GUI, messages)](#6-adaptateurs-entrants-http-cli-gui-messages)
7. [Infrastructure (adaptateurs sortants)](#7-infrastructure-adaptateurs-sortants)
8. [Erreurs](#8-erreurs)
9. [Sécurité (Secure by Design)](#9-sécurité-secure-by-design)
10. [Interfaces, injection de dépendances, API publique](#10-interfaces-injection-de-dépendances-api-publique)
11. [Concurrence, `context`, temps](#11-concurrence-context-temps)
12. [Processus TDD](#12-processus-tdd)
13. [Stratégie de test : niveaux et doublures](#13-stratégie-de-test--niveaux-et-doublures)
14. [Techniques de test en Go](#14-techniques-de-test-en-go)
15. [Refactoring](#15-refactoring)
16. [Performance](#16-performance)
17. [Outillage et discipline](#17-outillage-et-discipline)
18. [Faire respecter l'architecture](#18-faire-respecter-larchitecture)
19. [Quand alléger (petits projets)](#19-quand-alléger-petits-projets)
20. [Correspondance avec ce dépôt](#20-correspondance-avec-ce-dépôt)
21. [Liste de contrôle pour une revue](#21-liste-de-contrôle-pour-une-revue)

---

## 1. Principes généraux

- **G1. Le logiciel doit pouvoir changer.** Un système utilisé doit évoluer, sinon il devient inutile
  (loi du changement continu de Lehman). Sa complexité augmente tant qu'on ne travaille pas à la
  réduire (loi de la complexité croissante). On ne livre pas des fonctionnalités à la chaîne sans
  refactorer [LGWT, *Why*].
- **G2. Ne cherche pas à prédire l'avenir.** Pas de système « parfaitement extensible » conçu à
  l'avance : garde le code malléable avec des tests et du refactoring permanent [LGWT, *Why*].
- **G3. Petits morceaux composables.** Modules, fonctions, types et services doivent rester assez
  petits pour être réécrits en quelques jours [DDH, *Modules*].
- **G4. Sépare la complexité essentielle (le métier) de la complexité accidentelle (réseau,
  disque, protocoles, frameworks).** C'est le but de toute l'architecture qui suit [LGWT, *Scaling
  acceptance tests*].
- **G5. Une responsabilité par brique** (principe de responsabilité unique), quand ça a du sens :
  les repositories pour l'accès aux données, les entités pour la logique métier, etc. [DDH, *Diagram*].
- **G6. Simple n'est pas facile.** Ajouter un champ à une table de test, copier-coller une boucle,
  c'est facile, mais pas forcément simple. Ne confonds pas non plus ce qui t'est peu familier et ce
  qui est complexe [LGWT, *Anti-patterns*, *Revisiting arrays with generics*].
- **G7. Avance par tranches verticales fines.** Fais marcher une petite fonctionnalité de bout en
  bout, couverte par des tests, puis itère. Évite le « big bang » et la conception bottom-up non
  validée [LGWT, *Mocking*, *Scaling acceptance tests*, *Reading files*].
- **G8. « Make it work, make it right, make it fast »**, dans cet ordre. Pas d'optimisation
  prématurée [LGWT, *Concurrency*].
- **G9. Les noms comptent.** Utilise le langage du métier (*ubiquitous language*) [DDH] et les noms
  établis des patterns (`Map`, `Reduce`, `Find`, adapter, decorator) plutôt que d'en inventer
  [LGWT, *Revisiting arrays with generics*].

## 2. Architecture : couches et sens des dépendances

Flux d'une requête [DDH, *Diagram*] :

```
Adaptateur entrant (HTTP / CLI / GUI / message)
   │  DTO brut → validation → Command / Query
   ▼
Couche application (use case / service)
   │  orchestre ; appelle le domaine ; utilise les ports (interfaces)
   ▼
Couche domaine (entités, agrégats, value objects, services du domaine)
   ▲
   │  implémente les ports
Infrastructure (repositories, clients d'API, drivers, broker, logger…)
```

- **A1. Les dépendances pointent vers l'intérieur.** Les couches externes peuvent importer les
  couches internes, jamais l'inverse. Le cœur (domaine et application) n'importe ni framework, ni
  driver, ni `net/http` [DDH, *Ports*].
- **A2. Le cœur accède au monde extérieur par des ports.** En Go, un port est une **interface
  déclarée du côté de celui qui l'utilise** (le cœur). L'implémentation vit dans l'infrastructure et
  elle est injectée [DDH, *Ports*] [LGWT, *Dependency injection*].
- **A3. Conçois un port pour le besoin du domaine,** pas comme copie de l'API de l'outil : un port
  `CameraStore`, pas une interface qui recopie le client SQL [DDH, *Ports*].
- **A4. Ne crée pas d'abstraction inutile.** Dépendre d'un type concret est souvent très bien.
  Une interface se justifie quand il y a plusieurs implémentations, une dépendance externe à
  remplacer en test, ou un besoin réel de découplage [DDH, *Ports*, *Repositories*]
  [LGWT, *Anti-patterns*].
- **A5. Le cœur ne connaît pas son contexte d'appel.** Il doit pouvoir servir un handler HTTP, une
  CLI, une GUI ou un consommateur de messages sans changer [DDH, *Domain errors*, *Resolvers*].
- **A6. `main` ne fait que l'assemblage.** Il construit les briques et les branche ensemble ; aucune
  logique dedans [LGWT, *Scaling acceptance tests*, *Command line*].

## 3. Modules et arborescence Go

- **M1. Découpe par modules métier** (contextes bornés), chacun dans son dossier, et non par type
  technique (`controllers/`, `services/`, `repositories/`), qui disperse une même fonctionnalité dans
  plusieurs dossiers et mène au code spaghetti [DDH, *Folder structure*].
- **M2. Garde ensemble ce qui change ensemble** (Common Closure Principle, découpage vertical). Un
  use case regroupe son handler, son service, ses DTO et sa commande [DDH, *Folder structure*].
- **M3. Mets à part ce qui est partagé par plusieurs composants** (entités, ports, DTO communs)
  [DDH, *Folder structure*].
- **M4. Un module est une mini-application.** Ses internes sont privés. Les modules coopèrent via
  une façade publique, un médiateur ou des messages, pas en important les détails des autres
  [DDH, *Modules*]. En Go : le package `internal/` et les identifiants non exportés font respecter
  cette règle par le compilateur [Ajout].
- **M5. Pas de dépendance entre use cases ou entre modules.** Si deux composants partagent une
  logique, extrais-la dans un troisième dont les deux dépendent [DDH, *Modules*].
- **M6. Pas de dossier fourre-tout `domain/`** avec des centaines de types sans rapport. Regroupe
  les idées qui vont ensemble dans des packages nommés d'après le métier [LGWT, *Scaling acceptance
  tests*].
- **M7. Ajoute de la structure quand elle devient nécessaire, pas avant.** La bibliothèque standard
  est très plate. Conventions : `cmd/<app>/main.go` pour chaque binaire, le code importable à côté,
  les adaptateurs regroupés de façon cohérente [LGWT, *Command line*, *Scaling acceptance tests*].
- **M8. Évite l'imbrication profonde de dossiers,** et déplace les fichiers jusqu'à ce que la
  structure « sonne juste » [DDH, *Folder structure*].
- **M9. Noms de fichiers explicites** pour la recherche floue : `create_user.go`, `user_store.go`,
  `camera_handler.go` (DDH conseille `*.service.ts` ; en Go, préfère les underscores et des suffixes
  parlants) [DDH, *File names*].
- **M10. Fournis la bibliothèque avec le programme,** et inversement : une API qu'on peut aussi
  lancer en ligne de commande est plus facile à apprendre [LGWT, *Maths*].

## 4. Couche domaine

### Entités

- **D1. Les entités portent la logique métier.** Une logique métier qui finit dans les services
  produit un modèle anémique. Exception : les services du domaine, pour une logique qui couvre
  plusieurs entités [DDH, *Entities*].
- **D2. Une entité a une identité stable**, et l'égalité se fait sur l'identifiant [DDH].
- **D3. Une entité est toujours valide.** Elle valide ses invariants à la création et échoue vite à
  la première violation. En Go : **champs non exportés + constructeur `NewX(...) (X, error)`**, pas
  de construction à vide [DDH, *Entities*].
- **D4. Pas de setters publics.** L'état change par des méthodes métier (`Deposit`, `Withdraw`,
  `AssignTo`) qui revérifient les invariants [DDH, *Entities*] [LGWT, *Pointers & errors*].
- **D5. Rends immuable ce qui ne doit pas changer** (`id`, `createdAt`) : pas de méthode qui les
  modifie [DDH].
- **D6. Modélise le domaine pour la logique métier, pas pour le schéma de la base** [DDH].
- **D7. Le domaine ne connaît pas les couches supérieures** (services, handlers) [DDH].
- **D8. Un module peut contenir plusieurs entités,** à condition que leur logique soit liée. Pas un
  module par entité [DDH, *Entities*].

### Agrégats

- **D9. Un agrégat est un groupe d'objets traité comme une seule unité.** Toute référence venant de
  l'extérieur passe par la **racine** [DDH, *Aggregates*].
- **D10. Les opérations sur un agrégat sont transactionnelles** (tout ou rien), et tous ses
  invariants sont vrais après chaque modification (frontière de cohérence) [DDH].
- **D11. Entre agrégats, on se référence par identifiant,** pas par pointeur [DDH].
- **D12. Seules les racines se chargent directement depuis la persistance** [DDH].
- **D13. Évite les gros agrégats** : ils coûtent en performance et en maintenance [DDH].

### Value objects et primitives du domaine

- **D14. Remplace les primitives qui ont des règles par des types du domaine** (contre l'obsession
  des primitives) : `Email`, `IPAddress`, `FStop`, `Kelvin`, `Bitcoin`. En Go, un type nommé
  (`type Bitcoin int`) ou une struct à champ non exporté avec son constructeur validant
  [DDH, *Value objects*] [LGWT, *Pointers & errors*].
- **D15. Un value object n'a pas d'identité** (égalité structurelle) et il est **immuable** : en Go,
  receveurs par valeur, et les méthodes renvoient une nouvelle valeur [DDH].
- **D16. Le value object détient son invariant et son comportement** (validation, formatage,
  comparaison). Les entités n'ont plus à le faire [DDH, citant *Secure by Design*].
- **D17. La valeur zéro existe toujours en Go.** Conçois le type pour que la valeur zéro soit
  valide, ou détectable comme invalide (`IsZero()`), puisque n'importe qui peut écrire `X{}` [Ajout].
- **D18. Ne crée un type que pour les primitives qui ont des règles ou un comportement.** Un type par
  champ est excessif dans un petit projet [DDH, *Value objects*, notes].
- **D19. Un value object ne sort pas tel quel du processus** (DTO, événements, persistance) : il est
  converti en primitives [DDH, *Value objects*, *Warning*].

### Rendre les états illégaux impossibles à représenter

- **D20. Laisse le compilateur porter le plus de règles possible** : constantes typées (`type Mode
  int` + `const`), types distincts, interfaces fermées (méthode non exportée) pour simuler une union,
  *typestate* (un type par état) [DDH, *Make illegal states unrepresentable*].
- **D21. Ce qui ne peut pas être vérifié à la compilation l'est à l'exécution, à deux endroits** :
  à l'entrée (validation des DTO) puis dans le domaine (garde des invariants) [DDH, *Validation at
  runtime*].

### Validation ou garde

- **D22. La validation filtre l'extérieur** : une entrée invalide n'a rien d'exceptionnel, on la
  rejette avec des messages clairs, de préférence tous à la fois (pattern *Notification*) [DDH].
- **D23. La garde protège le domaine** : une violation d'invariant dans le domaine révèle un bogue.
  En Go, le constructeur renvoie une erreur ; un état « impossible » détecté en interne peut justifier
  un `panic` (échec rapide), jamais pour une entrée utilisateur [DDH, *Guarding vs validating*]
  [Ajout pour l'idiome Go].
- **D24. Pas de bibliothèque de validation générique dans le domaine.** Garde-la à la frontière.
  Une bibliothèque très spécialisée liée à un ou deux value objects est acceptable. Méfie-toi des
  expressions régulières maison qui contredisent celles de la validation d'entrée [DDH].

### Services et événements du domaine

- **D25. Un service du domaine contient la logique qui couvre plusieurs entités** et qui
  n'appartient naturellement à aucune. Il ne manipule que des types du domaine [DDH, *Domain
  services*].
- **D26. Événements du domaine** : pour propager un changement à d'autres agrégats du même processus
  sans les coupler (en Go : abonnés en mémoire, callbacks ou canal). Ils servent aussi au journal
  d'audit [DDH, *Domain events*].
- **D27. N'utilise pas que des événements pour un workflow long** : une chaîne d'événements devient
  illisible. Un orchestrateur garde tout le flux au même endroit [DDH].
- **D28. Événements d'intégration** (vers d'autres processus) : publie-les **après** la persistance.
  En distribué : outbox transactionnelle, sagas, cohérence à terme [DDH, *Integration events*].

### Bibliothèques dans le cœur

- **D29. Le cœur n'importe pas de bibliothèque qui** fait des E/S (HTTP, base de données), apporte
  de la technique sans rapport avec le métier (ORM, logger, framework), introduit de l'aléatoire
  (UUID, `time.Now`), ou change souvent ou tire beaucoup de dépendances. Pour ça : un adaptateur ou
  une façade, c'est-à-dire une couche anti-corruption [DDH, *Using libraries*].
- **D30. Injecte l'horloge et le générateur d'identifiants** (`func() time.Time`, interface
  `IDGenerator`) pour des tests déterministes. En Go on ne peut pas les remplacer à la volée comme en
  JS [DDH] [LGWT, *Mocking*, *Time*].
- **D31. Les frameworks restent à l'extérieur.** Tu dois pouvoir extraire le domaine et reconstruire
  une infrastructure autour sans casser la logique métier [DDH].

## 5. Couche application (use cases)

- **U1. Un service applicatif par use case** (*interactor*). Il orchestre : il charge les agrégats
  via les ports, appelle leurs méthodes, persiste, émet les effets externes. **Il ne contient pas de
  logique métier** [DDH, *Application services*].
- **U2. Il reçoit des types scalaires et les convertit en types du domaine** [DDH].
- **U3. Un service applicatif ne dépend pas d'un autre service applicatif** (risque de cycles)
  [DDH]. En Go, un cycle d'imports ne compile même pas [Ajout].
- **U4. Sépare les commandes et les requêtes (CQS).** Une commande change l'état et ne renvoie pas
  de données métier (au plus un identifiant ou un statut). Une requête lit et ne modifie rien
  [DDH, *Commands and Queries*].
- **U5. Une commande est une struct d'intention** (`CreateUserCommand`), distincte du DTO d'entrée
  [DDH, *DTOs*].
- **U6. Pas de commande qui déclenche une autre commande.** Chaîne-les par un événement
  (commande → événement → commande) [DDH].
- **U7. Une requête peut court-circuiter le domaine et les repositories** et lire directement un
  modèle de lecture [DDH, *Queries*].
- **U8. CQS dès le départ** : séparer plus tard lecture et écriture (CQRS, bases distinctes) devient
  possible sans réécriture. C'est un levier de **scalabilité** [DDH].
- **U9. Un bus de commandes ou de requêtes n'est utile que pour découpler l'appelant de
  l'exécutant.** En Go, un appel de méthode sur une interface suffit le plus souvent [DDH] [Ajout].

## 6. Adaptateurs entrants (HTTP, CLI, GUI, messages)

- **H1. Un handler a trois responsabilités, pas plus** : 1) analyser et valider la requête, 2) appeler
  le service applicatif, 3) traduire le résultat en réponse (statut, corps). Ni règle métier, ni
  connexion à la base, ni envoi d'e-mail [LGWT, *Revisiting HTTP handlers*] [DDH, *Controllers*].
- **H2. Un `http.HandlerFunc` n'est qu'une fonction : teste-le en l'appelant** avec
  `httptest.NewRecorder()` et `httptest.NewRequest` [LGWT, *HTTP server*].
- **H3. Un adaptateur par type de déclencheur** (HTTP, CLI, message), tous branchés sur le même use
  case : seul l'adaptateur change d'un transport à l'autre [DDH, *Controllers*] [LGWT, *Scaling
  acceptance tests*].
- **H4. DTO de requête** : le contrat que le client doit respecter. Ce sont des structs plates avec
  les balises `json` **uniquement à cet endroit**, jamais sur les types du domaine [DDH, *DTOs*].
- **H5. DTO de réponse en liste blanche** : n'expose que les champs prévus, jamais l'entité
  directement, pour ne pas fuiter un champ sensible ajouté plus tard [DDH, *Additional
  recommendations*].
- **H6. Les DTO protègent les clients des changements internes.** Pour casser un contrat, versionne
  (`/v2/...`) [DDH].
- **H7. Un DTO local (entre modules) se justifie pour ne pas fuiter les entités d'un module à
  l'autre**, mais coûte du mapping [DDH, *Local DTOs*].
- **H8. Traduis les erreurs du domaine en codes de transport à la frontière** (`ErrNotFound` → 404,
  `ErrAlreadyExists` → 409). Un 500 pour toutes les erreurs donne une très mauvaise API
  [LGWT, *Revisiting HTTP handlers*] [DDH, *Domain errors*].
- **H9. Annonce le format de la réponse** (`Content-Type: application/json`) [LGWT, *JSON*].
- **H10. Routage par `http.ServeMux` construit une seule fois dans le constructeur,** pas à chaque
  requête [LGWT, *JSON*].
- **H11. HTML : `html/template`** (échappement contre l'injection), templates parsés une fois et
  embarqués avec `embed`. `template.HTML` est réservé à du contenu de confiance [LGWT, *HTML
  templates*].
- **H12. Templates sans logique** : prépare un *view model* testable à part plutôt que des fonctions
  dans le template [LGWT, *HTML templates*].
- **H13. Arrêt propre** : écoute `SIGTERM` (`os/signal`), cesse d'accepter des requêtes, laisse finir
  celles en cours (`http.Server.Shutdown`) [LGWT, *Intro to acceptance tests*].

## 7. Infrastructure (adaptateurs sortants)

- **I1. L'infrastructure est la couche la plus volatile** : garde-la loin du domaine, remplaçable
  [DDH, *Infrastructure*].
- **I2. Un adaptateur implémente un port,** et on ne l'appelle qu'à travers ce port [DDH,
  *Adapters*].
- **I3. Un adaptateur a** son port, un *mapper* vers et depuis le domaine, un DTO pour les données
  reçues, et **une validation des données entrantes** : une réponse d'API ou de base n'est pas digne
  de confiance [DDH, *Adapters*, *Validation at runtime*].
- **I4. Un adaptateur sert de couche anti-corruption** devant un système legacy ou une API mal
  conçue [DDH].
- **I5. Un repository est une collection d'agrégats** : il reçoit et renvoie des types du domaine et
  cache où et comment les données sont stockées [DDH, *Repositories*].
- **I6. Abstraire la base n'est pas toujours utile** : on change rarement de SGBD et l'abstraction
  empêche d'en exploiter toute la puissance. Décide au cas par cas [DDH, *Repositories*].
- **I7. Sépare modèle du domaine et modèle de persistance,** avec un mapper : on peut alors
  normaliser ou dénormaliser sans toucher au domaine. C'est excessif pour une petite application
  [DDH, *Persistence models*].
- **I8. Pour les données à forte charge, préfère des requêtes SQL explicites à un ORM** [DDH].
- **I9. Client HTTP : jamais `http.DefaultClient`.** Injecte un `*http.Client` configuré avec un
  timeout [LGWT, *Scaling acceptance tests*].
- **I10. Ferme toujours `resp.Body`,** sinon les descripteurs de fichiers fuient [LGWT, *Select*].
- **I11. Ne recrée pas une connexion à chaque appel** (`sync.Once`, ou construction une seule fois)
  [LGWT, *Scaling acceptance tests*].
- **I12. Profite des interfaces standard** (`io.Reader`, `io.Writer`, `io.ReadWriteSeeker`, `fs.FS`) :
  le code devient réutilisable (fichier, réseau, `embed.FS`, buffer de test) [LGWT, *Reading files*,
  *IO*].
- **I13. Les écritures de fichier remplacent tout le contenu** : tronque le fichier (`Truncate`)
  avant de réécrire depuis le début [LGWT, *IO*]. Pour une écriture atomique : fichier temporaire
  puis renommage [Ajout].
- **I14. Logger, tâches planifiées, code de framework** sont aussi de l'infrastructure
  [DDH, *Other things*].

## 8. Erreurs

- **E1. Le cœur ne renvoie jamais de code HTTP ni d'erreur propre à un transport.** Il renvoie des
  erreurs du domaine [DDH, *Domain errors*].
- **E2. Les erreurs métier attendues sont des valeurs renvoyées et explicites** (l'équivalent du
  `Result` de DDH est simplement `(T, error)` en Go). Deux formes :
  - **erreur sentinelle** : `var ErrInsufficientFunds = errors.New("cannot withdraw, insufficient
    funds")`, testée avec `errors.Is` ;
  - **type d'erreur** avec des données (`BadStatusError{URL, Status}`), extrait avec `errors.As`
    [DDH] [LGWT, *Pointers & errors*, *Error types*, *Maps*].
- **E3. Ne compare jamais le texte d'une erreur** (ni dans le code, ni dans les tests) : c'est
  fragile pour toi comme pour tes appelants [LGWT, *Error types*].
- **E4. Ajoute du contexte en enveloppant avec `%w`** : `fmt.Errorf("withdraw from %s: %w", account,
  err)`. Un `return err` nu à chaque couche fait perdre l'origine. `if err != nil { return err }`
  partout n'est **pas** idiomatique [LGWT, *Pointers & errors*, *IO*].
- **E5. Une erreur précise par cas utile** (`ErrNotFound`, `ErrWordDoesNotExist`) : l'appelant peut
  réagir différemment [LGWT, *Maps*].
- **E6. Erreurs constantes** : `type DictionaryErr string` avec sa méthode `Error()` rend les
  sentinelles immuables [LGWT, *Maps*].
- **E7. Distingue l'attendu de l'irrécupérable.** Une panne technique (connexion perdue, mémoire)
  qu'on ignorerait laisserait le programme dans un état incohérent : propage-la jusqu'à une couche
  qui sait l'arrêter, la journaliser ou réessayer, et ne la masque jamais. Une violation
  d'invariant interne peut être un `panic` [DDH, *Domain errors*, *Warning*] [Ajout pour l'idiome].
- **E8. Ne laisse aucune erreur non vérifiée** (`errcheck`, `go vet`) [LGWT, *Pointers & errors*].
- **E9. Vérifie `nil` avant de déréférencer** un pointeur ou une interface renvoyés, et
  n'écris jamais dans une map `nil` : initialise-la (`map[K]V{}` ou `make`) [LGWT, *Pointers*,
  *Maps*].
- **E10. Ne teste pas `return err` pour lui-même** : un test qui vérifie seulement qu'une erreur est
  propagée apporte peu. Teste quand tu **fais** quelque chose de l'erreur [LGWT, *Scaling acceptance
  tests*, *Reading files*].

## 9. Sécurité (Secure by Design)

- **S1. La sécurité vient de la conception.** Des types du domaine toujours valides donnent une
  validation des entrées et des sorties dans **chaque** fonction, sans effort supplémentaire
  [DDH, citant *Secure by Design*, chap. 5].
- **S2. Ne fais confiance à aucune donnée** : utilisateur, API externe, base de données, fichier de
  configuration, ou erreur de programmation [DDH, *Validation at runtime*].
- **S3. Deux lignes de défense** : validation à la frontière (DTO), puis garde des invariants dans
  le domaine [DDH].
- **S4. Réponses en liste blanche** (H5) : aucune fuite d'un champ ajouté plus tard [DDH].
- **S5. Le cœur n'a pas accès aux ressources extérieures** (pas d'E/S dans le domaine) : une surface
  d'attaque réduite et un comportement testable [DDH, *Using libraries*].
- **S6. Peu de dépendances.** Chaque dépendance est une source possible de bogues et de failles.
  Trouve l'équilibre : ne réécris pas une bibliothèque éprouvée, mais n'importe pas pour trois lignes
  [DDH, *Using libraries*]. Vérifie les vulnérabilités connues avec `govulncheck` [Ajout].
- **S7. Encapsulation** : n'exporte que le nécessaire. N'embarque pas un type concret dont les
  méthodes publiques deviendraient les tiennes (un `sync.Mutex` embarqué expose `Lock`/`Unlock`, un
  `http.ServeMux` embarqué permet à un appelant d'ajouter des routes) [LGWT, *Sync*, *JSON*].
- **S8. HTML échappé par défaut** (`html/template`), `template.HTML` seulement pour du contenu de
  confiance ou passé par un nettoyeur fiable [LGWT, *HTML templates*].
- **S9. `context.Value` informe, il ne contrôle pas** : un identifiant de trace, oui ; des données
  nécessaires au fonctionnement, non. Elles passent en paramètres typés [LGWT, *Context*].
- **S10. Timeouts partout** : client HTTP, `select` avec `time.After`, `context.WithTimeout`. Une
  attente sans fin est un déni de service qu'on s'inflige soi-même [LGWT, *Select*, *Context*].
- **S11. Pas de « mode test » dans le code de production** (`GetData(mode string)`) : injecte la
  dépendance à la place [LGWT, *OS exec*].
- **S12. Secrets hors du code et hors de la configuration versionnée,** fichiers avec des droits
  restreints [Ajout, déjà appliqué ici : `rig.json` en 0600, mots de passe à part].

## 10. Interfaces, injection de dépendances, API publique

- **C1. L'injection de dépendances, c'est juste passer la dépendance en paramètre.** Pas besoin de
  framework, et elle ne complique pas la conception [LGWT, *Dependency injection*].
- **C2. Accepte des interfaces, renvoie des structs.** Prends l'interface la plus petite qui suffit
  (`io.Writer` plutôt que `*bytes.Buffer`, `io.Reader` plutôt que `fs.File`, `string` plutôt que
  `fs.DirEntry` si seul le nom sert) [LGWT, *DI*, *Reading files*].
- **C3. Plus l'interface est grande, plus l'abstraction est faible.** Une interface à beaucoup de
  méthodes trahit une abstraction qui fuit. Vise une méthode [LGWT, *Anti-patterns*].
- **C4. Évite la pollution d'interfaces** : expose une struct avec ses méthodes, et laisse chaque
  client déclarer l'interface du sous-ensemble dont il a besoin. N'expose une interface que si elle
  est petite et cohérente, **et** qu'il faut découpler (plusieurs implémentations, dépendance externe
  à remplacer) [LGWT, *Anti-patterns*] [DDH, *Ports* (ségrégation des interfaces)].
- **C5. Pour une interface à une seule méthode, fournis le type fonction adaptateur**
  (`type BlindAlerterFunc func(...)`), comme `http.HandlerFunc` [LGWT, *Time*].
- **C6. Pas d'état global** (pool de connexions, configuration en variables globales) : il rend le
  code intestable et cache les responsabilités. Avec l'injection, la liste des dépendances documente
  l'intention [LGWT, *DI*, *Time*].
- **C7. Une liste de dépendances qui grossit est un signal** : à partir de 4, le type en fait
  probablement trop. Regroupe-les dans un collaborateur métier (`Game`) [LGWT, *Time*,
  *Anti-patterns*].
- **C8. Constructeurs `NewX(...)`** dès que la valeur zéro n'est pas utilisable ou que des champs
  privés doivent être remplis ; ils montrent au lecteur qu'il ne doit pas initialiser le type à la
  main [LGWT, *Sync*, *Command line*].
- **C9. Calcule dans le constructeur ce qui ne dépend pas des arguments de la méthode** (URL de
  base, template parsé, encodeur) [LGWT, *Refactoring checklist*, *IO*, *HTML templates*].
- **C10. Receveurs cohérents** : si une méthode a besoin d'un pointeur (mutation, mutex), toutes les
  méthodes du type utilisent un pointeur [LGWT, *Pointers*].
- **C11. Composition plutôt qu'héritage** : pas de chaînes d'héritage profondes. En Go, pas
  d'héritage du tout ; l'embarquement est de la composition, mais il élargit ton API publique (S7)
  [DDH, *Prevent massive inheritance chains*] [LGWT, *JSON*].
- **C12. Pour ajouter un comportement sans modifier le code** (logs, télémétrie, retries,
  annulation), utilise le pattern *decorator* ou *delegation* : une struct qui enveloppe l'interface
  [LGWT, *Context-aware reader*, *Working without mocks*].
- **C13. Évite `interface{}` / `any` et la réflexion** : tu perds la sûreté des types et tu
  repousses les erreurs à l'exécution. Préfère une vraie interface ou les génériques
  [LGWT, *Reflection*, *Generics*].
- **C14. Génériques : seulement après avoir vu le même code environ trois fois,** en partant
  d'implémentations concrètes testées. Un peu de duplication vaut mieux qu'un couplage à une mauvaise
  abstraction [LGWT, *Generics*].
- **C15. Les erreurs de compilation sont tes amies, celles d'exécution tes ennemies** : écoute le
  compilateur et exprime les contraintes dans les types [LGWT, *Arrays*, *Generics*].
- **C16. Documente les API exportées** (commentaires de doc) et ajoute des `Example` testables : la
  doc reste juste parce qu'elle est compilée et exécutée [LGWT, *Integers*].

## 11. Concurrence, `context`, temps

- **K1. Lance toujours les tests avec `-race`.** Un accès concurrent non synchronisé est un bogue
  même s'il « marche » [LGWT, *Concurrency*, *Revisiting time with synctest*].
- **K2. Canaux pour transférer la propriété d'une donnée, mutex pour protéger un état partagé** (wiki
  Go, *MutexOrChannel*). N'abuse pas des goroutines et des canaux juste parce que c'est possible
  [LGWT, *Sync*].
- **K3. Une seule valeur partagée : `sync/atomic`** (`atomic.Int64`, `atomic.Bool`) ; plusieurs
  champs liés par un invariant : `sync.Mutex` [LGWT, *Sync*].
- **K4. Un mutex ne se copie pas** (`go vet` le détecte) : passe un pointeur, fournis un
  constructeur. Ne l'embarque pas (S7) [LGWT, *Sync*].
- **K5. Ne communique pas en partageant la mémoire ; partage la mémoire en communiquant.** Une
  goroutine qui produit un résultat l'envoie sur un canal plutôt que d'écrire dans une variable
  partagée [LGWT, *Revisiting time with synctest*].
- **K6. Crée les canaux avec `make`** : un canal `nil` bloque pour toujours. Pour un simple signal,
  utilise `chan struct{}` et `close` [LGWT, *Select*].
- **K7. Tout `select` qui peut bloquer a une sortie** : `ctx.Done()` ou `time.After` [LGWT,
  *Select*].
- **K8. `context.Context` en premier paramètre de toute fonction sur le chemin entre une requête
  entrante et un appel sortant.** Propage-le, dérive-le (`WithCancel`, `WithTimeout`), respecte
  `ctx.Done()`, et renvoie `ctx.Err()` pour que l'appelant connaisse la cause. Chaque composant gère
  sa propre annulation plutôt qu'un appelant qui « annule » ses dépendances à la main
  [LGWT, *Context*, *Context-aware reader*].
- **K9. Variable de boucle capturée par une goroutine** : sûr depuis Go 1.22 seulement si `go.mod`
  déclare `go >= 1.22` [LGWT, *Concurrency*].
- **K10. Le temps est une dépendance** : injecte une horloge ou un planificateur (`BlindAlerter`),
  ou teste le vrai code avec `testing/synctest` (Go 1.25+) [LGWT, *Time*, *Revisiting time*].
- **K11. Dormir ne synchronise pas** : un `time.Sleep` « assez long » reste une course de données.
  Synchronise réellement (canal, `WaitGroup`, `synctest.Wait()`) [LGWT, *Revisiting time*].
- **K12. Le callback de `time.AfterFunc` tourne dans sa propre goroutine** : l'état qu'il touche
  doit être protégé [LGWT, *Revisiting time*].
- **K13. `sync.WaitGroup` pour attendre un groupe de goroutines** [LGWT, *Sync*].
- **K14. Les maps ne sont pas sûres en écriture concurrente** (erreur fatale `concurrent map
  writes`) [LGWT, *Concurrency*].
- **K15. Transactions** : elles garantissent la cohérence mais deviennent un goulot d'étranglement
  si beaucoup d'utilisateurs modifient la même ligne. Au-delà, passe à la cohérence à terme (sagas,
  outbox) [DDH, *Domain events*]. C'est la règle clé de **scalabilité** côté données.

## 12. Processus TDD

- **P1. Le cycle** [LGWT, *Hello, World*] :
  1. écris un petit test pour un petit comportement ;
  2. fais compiler (écoute le compilateur, écris juste les signatures) ;
  3. **lance-le et vois-le échouer, avec un message clair** ;
  4. écris le minimum pour qu'il passe (les « péchés » sont permis) ;
  5. refactore, code de production **et** code de test ;
  6. commit.
- **P2. Voir le test échouer est obligatoire** : c'est la seule façon d'éviter les tests qui ne
  peuvent jamais échouer et les messages d'échec inutiles (`false was not equal to true`)
  [LGWT, *Anti-patterns*].
- **P3. Sors du rouge le plus vite possible.** Plus tu fais de changements pendant que ça ne compile
  pas ou que les tests échouent, plus tu accumules de problèmes. Priorité : que ça compile, puis que
  les tests passent [LGWT, *HTTP server*].
- **P4. Le minimum pour passer révèle les trous dans les tests** (un 404 renvoyé à tout le monde fait
  passer les tests → il manque une assertion sur le 200) [LGWT, *HTTP server*].
- **P5. Le TDD sert d'abord à avoir un retour sur la conception.** Si un test est pénible à écrire,
  le code sera pénible à utiliser : le test est son premier utilisateur. **Écoute tes tests**
  [LGWT, *Anti-patterns*].
- **P6. Commence par l'extérieur (top-down)** : un test d'acceptation sert de cap, puis des tests
  unitaires font émerger les unités. Le bottom-up produit du code qui ne s'intègre pas ou fait des
  choses inutiles [LGWT, *Scaling acceptance tests*, *Reading files*].
- **P7. Écris le test que tu voudrais voir**, du point de vue du consommateur : le *quoi* et le
  *pourquoi*, pas le *comment* [LGWT, *Reading files*].
- **P8. Squelette d'abord** (« itération 0 ») : un programme minimal, construit et testé de bout en
  bout, avant les fonctionnalités [LGWT, *Scaling acceptance tests*, *Command line*].
- **P9. Intègre tôt** : branche le code dans `main` rapidement. C'est en l'utilisant depuis un autre
  package qu'on découvre les champs privés impossibles à remplir [LGWT, *Command line*].
- **P10. Le TDD n'est pas une religion** : quand tu sais où tu vas, fais de plus grands pas, mais
  écris toujours le test d'abord [LGWT, *Maths*]. Pour explorer, fais un essai jetable (après un
  commit), puis reviens en arrière et pilote la vraie solution par un test [LGWT, *WebSockets*].
- **P11. « Écris des tests jusqu'à ce que la peur se transforme en ennui »** (souvent attribué à
  Kent Beck) [LGWT, *Maths*].

## 13. Stratégie de test : niveaux et doublures

### Ce qu'on teste

- **Q1. Teste le comportement, pas l'implémentation.** Des tests couplés aux détails (triangles
  internes d'un carré) bloquent le refactoring au lieu de le permettre [LGWT, *Why*].
- **Q2. Une « unité » est une brique cohérente avec une API publique,** pas une fonction ou une
  classe. Teste son API publique ; dessous, tu refactores librement [LGWT, *Why*].
- **Q3. Ne teste pas les fonctions privées** (sauf exception assumée pour un risque précis,
  cf. `tape` dans [LGWT, *IO*]). Ne rends pas une fonction publique juste pour la tester
  [LGWT, *Anti-patterns*, *Mocking*].
- **Q4. Ne teste pas le code des autres** : ni la sortie de `encoding/json`, ni une bibliothèque
  tierce [LGWT, *JSON*, *HTML templates*].
- **Q5. Chaque test a un coût.** Le but est la confiance, pas le nombre de tests ni 100 % de
  couverture. La couverture sert à trouver les trous ; supprime les tests redondants
  [LGWT, *Arrays*, *Pointers*].
- **Q6. Les tests ne sont pas du code jetable** : un bon test dure plus longtemps que le code qu'il
  teste. Soigne-le comme du code de production [LGWT, *Maths*].

### Pyramide

- **Q7. Beaucoup de tests unitaires rapides, quelques tests d'acceptation** [LGWT, *Intro to
  acceptance tests*, *Scaling acceptance tests*].
- **Q8. Test d'acceptation** = boîte noire, du point de vue de l'utilisateur, seulement par
  l'interface publique du système. C'est de la documentation vérifiée. Coût : lents, chers, peu
  précis sur la cause d'un échec, aveugles à la qualité interne [LGWT].
- **Q9. Quand écrire un test d'acceptation ?** Pour un parcours utilisateur, ou une chose dont les
  non-techniciens parlent beaucoup. Un cas limite se teste en unitaire [LGWT, *Scaling acceptance
  tests*].
- **Q10. Structure les tests d'acceptation en Spécification → DSL → Driver → Système** (Dave Farley).
  La spécification n'exprime que le métier et ne change que si le comportement change. Le driver
  (HTTP, gRPC, navigateur, appel direct au domaine) absorbe les changements techniques. La même
  spécification peut servir de test unitaire du domaine via un adaptateur [LGWT, *Scaling acceptance
  tests*].
- **Q11. Drivers configurables** : les mêmes tests d'acceptation tournent en local, en staging et
  idéalement en production [LGWT].
- **Q12. Code legacy sans tests : écris d'abord des tests d'acceptation, refactore ensuite**
  [LGWT, *Scaling acceptance tests*].
- **Q13. Tests d'intégration entre les deux** : plusieurs unités ensemble (service + fakes, serveur
  + store) sans démarrer tout le système [LGWT, *Working without mocks*, *HTTP server*].
- **Q14. Une personne non technique qui connaît le métier doit comprendre tes tests d'acceptation**
  (Dave Farley) [LGWT]. C'est aussi l'esprit des tests comportementaux et de Gherkin [DDH,
  *Behavioral testing*].

### Doublures de test

- **Q15. Choisis la doublure consciemment** [LGWT, *Working without mocks*, *Anti-patterns*] :
  - *dummy* : remplit une liste de paramètres, jamais utilisé ; nomme-le `dummyX` ;
  - *stub* : renvoie des données fixes ; **préfère-le au mock** ;
  - *spy* : enregistre les appels ; à n'utiliser que si l'ordre ou le nombre d'appels **est** le
    comportement voulu ;
  - *mock* : attentes programmées ; puissant, donc facile à mal utiliser ;
  - *fake* : une vraie implémentation simplifiée (store en mémoire) ; elle garde un état.
- **Q16. Plus de 3 doublures dans un test : alerte rouge** sur la conception. Le code a trop de
  dépendances ou des dépendances trop fines [LGWT, *Mocking*].
- **Q17. Des interactions compliquées avec les mocks signifient du code qui interagit trop** :
  simplifie la collaboration [LGWT, *Anti-patterns*].
- **Q18. Fakes + contrats pour les dépendances importantes** : un contrat est une suite de tests
  écrite contre l'**interface** et exécutée à la fois sur le fake et sur la vraie implémentation. Les
  hypothèses sur la dépendance sont alors codifiées et vérifiées, au lieu d'être dispersées dans des
  stubs ad hoc [LGWT, *Working without mocks*].
- **Q19. Quand ta connaissance d'une dépendance change** : modifie le contrat, vérifie-le contre le
  vrai système, mets le fake en conformité, puis lance toute la suite. Les autres tests qui cassent
  sont une **bonne** nouvelle [LGWT].
- **Q20. Avec un fake, vérifie l'état final, pas les appels** (« quels enregistrements le fake
  contient-il ? ») [LGWT].
- **Q21. Pour faire échouer un fake, enveloppe-le dans un decorator** avec des champs `XxxFunc`
  surchargeables, ou garde un stub au niveau unitaire pour les chemins d'erreur [LGWT].
- **Q22. Ne mocke pas le driver de base de données et ne vérifie pas le SQL envoyé** : c'est un
  détail d'implémentation. Fakes en mémoire et contrats ; la vraie base seulement pour valider le
  contrat [LGWT, *Working without mocks*].
- **Q23. Générateurs de mocks** : acceptables en équipe pour l'homogénéité, seulement s'ils génèrent
  à partir d'une interface et sans « magie » qui dicte la forme des tests [LGWT, *Mocking*].
- **Q24. Doublures exportées** : fournir les stubs et helpers dans un `testing.go` du package aide
  les utilisateurs du package (pratique HashiCorp). C'est un choix assumé qui élargit l'API
  [LGWT, *Command line*].

## 14. Techniques de test en Go

- **T1. Écris les tests dans un package `xxx_test`** (externe) par défaut : le compilateur
  t'empêche alors de te coupler à l'interne, et tu vis l'expérience d'un vrai utilisateur
  [LGWT, *Anti-patterns*, *Command line*, *Reading files*].
- **T2. Helpers de test** : prends `testing.TB` (valable pour tests et benchmarks) et appelle
  `t.Helper()` pour que l'échec pointe la ligne de l'appelant [LGWT, *Hello, World*].
- **T3. `t.Fatal` quand la suite du test n'a plus de sens** (erreur nil inattendue, décodage raté),
  `t.Error` sinon [LGWT, *Pointers & errors*, *JSON*].
- **T4. Des messages d'échec utiles** : `got %q want %q`, `%#v` pour une struct, les entrées
  affichées. Pour les flottants : `%g` [LGWT, *Hello*, *Arrays*, *Structs*].
- **T5. Sous-tests `t.Run("cas", ...)`** pour grouper et nommer les scénarios ; on en lance un seul
  avec `go test -run TestX/cas` [LGWT, *Hello*, *Structs*].
- **T6. Vise une assertion par test** : les sous-tests séparent les assertions, ce qui rend le test
  lisible et précis quand il échoue [LGWT, *Anti-patterns*].
- **T7. Tests en table quand seule l'entrée varie** : champs nommés, noms de cas, `t.Run`. **Sors
  de la table** dès qu'il faudrait y ajouter des booléens ou des champs de contrôle
  [LGWT, *Structs*, *Anti-patterns*].
- **T8. N'asserte que ce qui t'importe** : pas toute une struct quand un seul champ compte
  [LGWT, *Anti-patterns*].
- **T9. Sorties structurées (JSON, XML, SVG) : parse-les et compare des données,** ne compare pas des
  chaînes : sinon le test est à la fois fragile et pas assez précis [LGWT, *JSON*, *Maths*].
- **T10. Grosses sorties (HTML, rapports) : tests d'approbation** (« golden files »). Les fichiers
  approuvés vont dans git [LGWT, *HTML templates*].
- **T11. Flottants : compare avec une tolérance** (ou réarrange le calcul pour garder la précision)
  [LGWT, *Structs*, *Maths*].
- **T12. Ordre non garanti (maps) : n'asserte pas sur l'ordre,** mais vérifie quand même la longueur
  et le contenu [LGWT, *Reflection*].
- **T13. Tests basés sur les propriétés** (`testing/quick`) pour les règles du domaine, par exemple
  `FromRoman(ToRoman(n)) == n`. Borne le domaine des entrées générées [LGWT, *Roman numerals*].
- **T14. Outils de la bibliothèque standard** : `httptest` (serveur et recorder), `fstest.MapFS`,
  `strings.NewReader`, `bytes.Buffer`, `io.Discard`, `os.CreateTemp` avec nettoyage
  [LGWT, *Select*, *Reading files*, *IO*].
- **T15. Pas d'appel réseau réel dans un test unitaire** : c'est lent, instable, et les cas limites
  sont impossibles à provoquer [LGWT, *Select*].
- **T16. Pas de `time.Sleep` arbitraire dans les tests.** Pour de l'asynchrone : `within(timeout,
  assert)`, `retryUntil`, ou `synctest`. **Un test ne doit jamais bloquer indéfiniment**
  [LGWT, *WebSockets*, *Revisiting time*].
- **T17. Délais configurables** (`ConfigurableRacer`) pour que le test du timeout prenne des
  millisecondes, avec une fonction par défaut pour les utilisateurs [LGWT, *Select*].
- **T18. `testing/synctest`** : bulle à horloge factice. Le temps n'avance que lorsque toutes les
  goroutines sont durablement bloquées. Toutes les goroutines doivent se terminer, et les E/S réelles
  ne sont pas durablement bloquantes (utilise `net.Pipe` à la place) [LGWT, *Revisiting time*].
- **T19. `go test ./...` doit tourner sans configuration** (à part le compilateur, voire Docker).
  Les tests lents d'acceptation sautent avec `if testing.Short() { t.Skip() }` [LGWT, *Scaling
  acceptance tests*].
- **T20. Benchmarks** (`b.Loop()`, `-bench=. -benchmem`) avant et après une optimisation
  [LGWT, *Iteration*, *Concurrency*].
- **T21. `Example` testables** pour la doc (`// Output:`) [LGWT, *Integers*].
- **T22. Casse volontairement le code** (après un commit) pour vérifier qu'un test le détecte
  [LGWT, *Mocking*].
- **T23. Sois patient avec les messages d'erreur et les traces** : tout ce qu'il faut savoir y est
  écrit [LGWT, *Concurrency*, *Structs*].

## 15. Refactoring

- **R1. Refactorer, c'est changer le code sans changer le comportement.** Si tu dois modifier les
  tests, tu ne refactores pas, tu **conçois**. Pilote alors le changement par un test
  [LGWT, *Why*, *Refactoring checklist*].
- **R2. Quand les tests sont verts, tout est permis sauf changer le comportement** : nouveaux types,
  interfaces, fonctions privées [LGWT, *Anti-patterns*].
- **R3. Petits pas, tests à chaque pas** (quelques millisecondes), commit si c'est bien, revert
  sinon [LGWT, *Refactoring checklist*].
- **R4. Ne change pas en même temps le code de production et les tests** : d'abord la production
  sans toucher son interface, puis les tests [LGWT, *Time*].
- **R5. Les renommages automatiques par l'IDE comptent comme du refactoring** même s'ils touchent les
  tests : ils sont sûrs [LGWT, *Refactoring checklist*].
- **R6. Liste de contrôle à chaque cycle** [LGWT, *Refactoring checklist*] :
  - inline une variable qui n'apporte pas de sens ;
  - extrais une variable ou une constante pour une valeur répétée ou magique, en sachant que tout ce
    qui la partage devient couplé ;
  - DRY sur l'**idée**, pas sur les lignes ; si la liste de paramètres devient confuse, c'est un
    mauvais DRY ;
  - fonctions publiques lisibles d'un coup d'œil : elles disent **quoi**, les fonctions privées
    disent **comment** ;
  - déplace la création des valeurs indépendantes des arguments dans le constructeur ;
  - remplace un commentaire par une fonction bien nommée.
- **R7. D'abord rendre le changement facile, puis faire le changement facile** (Kent Beck)
  [LGWT, *Scaling acceptance tests*].
- **R8. Refactorer ne se demande pas, ça fait partie du travail.** « Pas le temps de refactorer »
  signifie justement qu'il faut refactorer [LGWT, *Refactoring checklist*].
- **R9. Les données plutôt que des `switch`** : une table de règles (`allRomanNumerals`) plutôt que
  la logique cachée dans des branches [LGWT, *Roman numerals*].
- **R10. Une abstraction ratée se défait** : reviens en arrière sans drame, les tests sont là pour ça
  [LGWT, *Reflection*].

## 16. Performance

- **F1. Mesure avant d'optimiser** (benchmarks, `-benchmem`) [LGWT, *Iteration*].
- **F2. Concurrence pour attendre en parallèle** les E/S indépendantes, en gardant séquentiel ce qui
  doit l'être (collecte des résultats par un canal) [LGWT, *Concurrency*].
- **F3. Fais une seule fois ce qui peut l'être** : parser les templates, créer le routeur,
  l'encodeur JSON, le dialer, lire le fichier au démarrage plutôt qu'à chaque appel
  [LGWT, *HTML templates*, *JSON*, *IO*, *Scaling acceptance tests*]. Exception : un objet à état non
  réutilisable (le parser gomarkdown) se recrée à chaque fois [LGWT, *HTML templates*].
- **F4. `strings.Builder` pour concaténer en boucle** [LGWT, *Iteration*].
- **F5. Évite les gros agrégats et les transactions trop larges** (D13, K15) [DDH].
- **F6. CQS/CQRS** : des modèles de lecture optimisés séparément de l'écriture quand la charge
  l'exige (U8) [DDH].
- **F7. Modules découpés selon les contextes bornés** : on peut extraire un module en microservice
  sans toucher au domaine. C'est la **scalabilité organisationnelle et technique** [DDH,
  *Architecture*, *Modules*].
- **F8. Tri** : là où il est le plus efficace (dans la base avec `ORDER BY` si elle existe, sinon
  `sort.Slice` dans le store) [LGWT, *IO*].

## 17. Outillage et discipline

- **O1. `gofmt`, `go vet` (copie de mutex, etc.), `errcheck`, `go test -race ./...`** à chaque
  changement [LGWT, *Sync*, *Pointers & errors*, *Concurrency*].
- **O2. Ta suite unitaire doit tourner en moins d'une seconde** : c'est elle qui permet de refactorer
  hardiment [LGWT, *Why*].
- **O3. Un bouton dans l'éditeur pour lancer les tests et refactorer** (extraire, inliner, renommer)
  en un raccourci, sinon on le fait moins [LGWT, *Refactoring checklist*].
- **O4. Commit dès que les tests passent, avant de refactorer** : on peut toujours revenir à l'état
  qui marche [LGWT, *Hello*, *Revisiting arrays with generics*].
- **O5. Étudie la bibliothèque standard** : `io`, `fs`, `context`, `sync`, `time`, `httptest`,
  `embed`, `slices`. Chaque interface standard connue rend ton code réutilisable [LGWT, *DI*,
  *Reading files*].
- **O6. `embed` pour les fichiers dont le binaire a besoin** (templates, assets) : pas de chemin
  relatif fragile selon le dossier de lancement [LGWT, *HTML templates*, *Command line*].
- **O7. `go.mod` : déclare une version de Go récente** (sémantique de boucle 1.22, `synctest` en
  1.25) [LGWT, *Concurrency*].
- **O8. Contrats et tests d'acceptation dans le pipeline de CI** pour être alerté vite quand une
  dépendance externe change [LGWT, *Working without mocks*].

## 18. Faire respecter l'architecture

- **X1. Vérifie les règles de dépendance automatiquement** : le domaine ne doit pas importer
  l'API, la base, ni les adaptateurs [DDH, *Enforcing architecture* (dependency-cruiser, ArchUnit)].
- **X2. En Go** [Ajout] :
  - le compilateur interdit déjà les cycles d'imports ;
  - `internal/` limite qui peut importer quoi ;
  - un test d'architecture (`go list -deps ./internal/core/...`, puis échec si un package
    d'adaptateur ou `net/http` apparaît), ou `depguard` / `go-arch-lint` via golangci-lint.
- **X3. Garde les modules indépendants** : un module doit pouvoir être supprimé et réécrit en
  quelques jours [DDH, *Modules*].

## 19. Quand alléger (petits projets)

[DDH, *Recommendations for smaller APIs*] [LGWT, *Generics*, *IO*]

- **Toujours utile**, quelle que soit la taille : modules et couches, handlers fins, services,
  entités qui protègent leurs invariants, injection de dépendances, frontières respectées, TDD,
  erreurs typées, `-race`.
- **À peser** pour une application surtout CRUD, avec peu de logique : un type par primitive,
  modèle de persistance séparé et mappers, bus de commandes, événements du domaine, CQRS avec bases
  séparées, ports devant une base qu'on ne changera jamais. Un MVC simple peut suffire.
- **Une interface sans polymorphisme réel** (personne ne la mocke, une seule implémentation
  plausible, `*os.File`) n'apporte rien : garde le type concret [LGWT, *IO*].

## 20. Correspondance avec ce dépôt

Comment ces règles s'appliquent à RCP For All Camera (voir [architecture.md](architecture.md)) :

| Notion | Ici |
|---|---|
| Domaine (contrats, types du métier) | `internal/core` : `ControlSpec`, `Value`, `ModelInfo`, erreurs |
| Application (use cases) | `internal/app` : `Service` (presets, réglages, contrôle caméra), `Host` |
| Ports | interfaces `core.Camera`, `core.Driver`, `panel.Device`, `app.PanelSources` |
| Adaptateurs entrants | `internal/gui` (Fyne), `internal/surfaces/panel` (le panneau pilote l'app) |
| Adaptateurs sortants | `internal/cameras/*` (REST Blackmagic…), `internal/surfaces/micropanel` (HID), `internal/config` (rig.json) |
| Fakes | `internal/cameras/demo`, `internal/surfaces/panelsim` |
| Contrats | `internal/cameras/cameratest` (suite exécutée par chaque driver) |
| Assemblage | `cmd/rcp` |

Points d'attention qui en découlent :

- les détails de protocole restent dans le driver (A1, I2) ; le cœur ne voit que `core.Value` ;
- les plages `TODO(verify)` sont des hypothèses sur un système externe : leur place est dans un
  contrat vérifiable sur le matériel (Q18), comme les tests `Live` du driver Blackmagic ;
- le démo et le simulateur sont des fakes : ils doivent passer les mêmes contrats que les vrais
  (Q18, Q19) ;
- une réponse de caméra est une donnée non fiable (S2, I3) : on la valide et on la normalise dans
  l'adaptateur (déjà fait : arrondi float32 à 6 décimales).

## 21. Liste de contrôle pour une revue

**Architecture**
- [ ] Le domaine et l'application n'importent aucun adaptateur, framework ni `net/http` (A1, X1).
- [ ] Les handlers ne font que parser, appeler et répondre (H1).
- [ ] `main` ne fait qu'assembler (A6).
- [ ] Aucun use case n'appelle un autre use case (U3) ; les commandes ne renvoient pas de données
      métier (U4).

**Domaine**
- [ ] Les nouveaux types ont des champs non exportés, un constructeur validant, une valeur zéro
      maîtrisée (D3, D17).
- [ ] Pas de setter public, les invariants sont revérifiés à chaque mutation (D4).
- [ ] Aucune balise `json` ni aucun type de base de données sur les types du domaine (H4, I7).

**Erreurs et sécurité**
- [ ] Les erreurs sont enveloppées avec `%w` et du contexte, testées avec `errors.Is`/`As`, jamais
      par leur texte (E3, E4).
- [ ] Toute entrée externe (utilisateur, réseau, fichier, matériel) est validée à la frontière (S2,
      I3).
- [ ] Les réponses exposent une liste blanche de champs (H5).
- [ ] Il y a un timeout et un `context` sur chaque E/S (S10, K8) ; aucun `http.DefaultClient` (I9).
- [ ] Aucune nouvelle dépendance sans justification (S6).

**Concurrence**
- [ ] `go test -race` passe ; aucun mutex copié ni embarqué (K1, K4).
- [ ] Chaque `select` bloquant a une sortie (K7) ; chaque goroutine a une fin.

**Tests**
- [ ] Le test a été vu en échec, avec un message clair (P2).
- [ ] Il teste l'API publique, dans un package `_test` (Q2, T1).
- [ ] Pas plus de 3 doublures ; fakes avec contrat pour les dépendances importantes (Q16, Q18).
- [ ] Pas de `time.Sleep` arbitraire ; aucun test ne peut bloquer (T16).
- [ ] Pas de comparaison de chaînes sur du JSON/XML ou des messages d'erreur (T9, E3).

---

### Sources

- Sairyss, *Domain-Driven Hexagon*, README : <https://github.com/Sairyss/domain-driven-hexagon>
- Chris James et contributeurs, *Learn Go with Tests*, tous les chapitres (fondamentaux Go,
  fondamentaux des tests, construire une application, questions-réponses, méta) :
  <https://quii.gitbook.io/learn-go-with-tests> (source : <https://github.com/quii/learn-go-with-tests>)
- Ouvrages cités par ces guides : *Secure by Design* (Johnsson, Deogun, Sawano), *Growing
  Object-Oriented Software, Guided by Tests* (Freeman, Pryce), *Refactoring* (Fowler), *Modern
  Software Engineering* (Farley).
