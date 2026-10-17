# Audit du code face aux règles d'ingénierie Go (2026-10-17)

Référence : [docs/go-engineering-rules.md](../go-engineering-rules.md). Pour chaque écart : la règle,
où il est, et ce qui a été décidé pour la 0.3.0-ALPHA. « Corrigé » veut dire que c'est fait et
testé ; « Gardé » veut dire que l'écart est assumé, avec sa raison (règle zéro : une règle se
dépasse en connaissance de cause).

## Écarts corrigés

| # | Règle | Écart constaté | Correction |
|---|---|---|---|
| 1 | M5, X3 | Le module `demo` importait le driver `blackmagic` pour lui emprunter ses réglages : un module dépendait d'un autre. | `demo` déclare ses propres réglages ; il ne dépend que de `core` et des kits partagés. |
| 2 | A2, C4 | Un « driver » regroupait plusieurs modèles, et le choix du modèle se faisait par `switch` sur l'identifiant (`specsFor`, `if m.ID != ...`). | Un module = un modèle (`core.Module`). Le code commun d'une marque vit dans une base (`cameras/bm`) que les modules composent, avec des hooks. |
| 3 | A1, A2 | `app` (couche application) importait `config` (infrastructure) : `config.Rig`, `config.Preset`, `config.Secrets`. | Les ports `Store` et `Secrets` et le modèle sauvegardé (`app.SavedRig`) sont déclarés dans `app` ; `config` les implémente. |
| 4 | H4, I7, D19 | Les types du domaine (`core.ControlSpec`, `Value`, `CameraConfig`, `ModelInfo`…) portaient des balises `json`, et `rig.json` était écrit directement depuis eux. | Plus aucune balise `json` dans `core`. `config` a ses propres DTO de persistance et un mapper ; le format de `rig.json` ne change pas. |
| 5 | C6, D30 | Logger global (`slog.Default()` dans le driver Blackmagic). | Le logger est injecté dans les modules par `cameras.All(log)`. |
| 6 | C6 | Variables globales modifiables : `app.commandTimeout`, `gui.Version`, `gui.MinWindow`, `widgets.LeverEvery`. | Passées en options ou en constantes (voir le détail dans les commits). |
| 7 | C14, G6 | Le même code (copie des specs, abonnés, recherche de spec) était dupliqué dans `demo`, `blackmagic` et `camerakit.Placeholder`. | Extrait dans `camerakit.Specs` et `camerakit.Subscribers`. |
| 8 | U4 | `Service.AssignSlot` (une commande) renvoyait des données (`Slot`). | Elle ne renvoie plus que l'erreur ; l'appelant relit `Rig()`. |
| 9 | A2 | La GUI (adaptateur entrant) appelait directement `sysinfo` (adaptateur sortant). | La GUI reçoit sa source de statistiques par injection. |
| 10 | X1, X2 | Aucune vérification automatique du sens des dépendances. | Test d'architecture : `core` n'importe aucun autre package interne, ni `net/http`, ni Fyne ; `app` n'importe ni `config`, ni la GUI, ni un module caméra ; un module caméra n'importe pas un autre module. |
| 11 | E3 | Un test comparait le texte d'une erreur (`strings.Contains(err.Error(), "not implemented")`). | Remplacé par `errors.Is`. |
| 12 | K1, O1, S6 | La CI ne lançait ni `-race`, ni `govulncheck`. | Ajoutés à la CI. |
| 13 | Q18 | Le contrat ne vérifiait que le comportement hors ligne ; le fake (`demo`) et la vraie caméra n'étaient pas soumis aux mêmes vérifications une fois connectés. | Nouveau contrat `cameratest.Online`, exécuté sur `demo` et sur la base `bm` (contre son faux serveur). |

## Écarts gardés (et pourquoi)

| # | Règle | Écart | Raison |
|---|---|---|---|
| A | T1 | Les tests sont dans le package testé (`package app`, pas `package app_test`). | Les passer tous en package externe obligerait à exporter des internes uniquement pour les tests, ce qu'interdit Q3. Les nouveaux tests des modules caméra et du cœur sont écrits en package externe ; les autres le deviendront au fil des modifications. |
| B | T16, K11 | Une trentaine de `time.Sleep` dans les tests du contrôleur de panneau, de la GUI et du driver Blackmagic. | Le contrôleur et la GUI ont leurs propres horloges (tickers Fyne, clignotements) : les retirer demande d'injecter une horloge partout, un chantier à part. Les attentes sont bornées (aucun test ne bloque indéfiniment). À reprendre avec `testing/synctest`. |
| C | C3 | `core.Camera` a 7 méthodes. | C'est le port que chaque module implémente, avec plusieurs implémentations réelles (démo, Blackmagic, placeholders) : la règle C4 l'autorise. Le découper obligerait chaque appelant à recomposer les morceaux. |
| D | D20 | `core.Value` est une struct (`Kind` + un champ par type) : une valeur `Kind: range` avec un `Choice` rempli est représentable. | Une union fermée (interface à méthode privée) toucherait tout le code de la GUI et du panneau pour un gain faible : `Value.Check` et `ControlSpec.Validate` gardent l'invariant à chaque frontière. |
| E | M9 | Noms de fichiers génériques (`service.go`, `camera.go`). | Renommer en masse brouille l'historique git pour peu de gain ; les nouveaux fichiers suivent la règle. |
| F | « héritage » | La demande parle d'un module qui « hérite » de la base `bm`. | Go n'a pas d'héritage et la règle C11 préfère la composition : le module construit la base avec ses réglages et ses hooks (`bm.New(cfg, bm.Model{...})`). L'effet attendu est le même : le code commun n'est écrit qu'une fois, et le module ne surcharge que ce qui lui est propre. |
