# Rapport — Spike USB de l'ATEM Micro Camera Panel

Date : 2026-10-11 · Statut : **terminé, GO** · Répond à la question ouverte n°1 de [SPEC.md](../../SPEC.md)

## 1. Résumé

**Le panneau est entièrement pilotable depuis un PC Windows, en USB, sans mixeur ATEM ni logiciel Blackmagic.**

- C'est un périphérique **HID standard** (interface MI_02), sans pilote propriétaire. Le seul verrou est une **authentification défi/réponse** sur le rapport feature 0x05. On l'a cassée : c'est le même algorithme que le DaVinci Speed Editor.
- **Toutes les entrées sont décodées et vérifiées sur le matériel** :
  - 31 boutons ;
  - 9 molettes en position absolue 0–1024 ;
  - le levier iris et la bague master black, de −4096 à +4096.
- **Toutes les sorties sont décodées et vérifiées sur le matériel** :
  - les 14 afficheurs 7 segments ;
  - la matrice caméra 7×10 tricolore ;
  - les 75 LEDs : boutons caméra RGB, rétroéclairages, voyants, barre iris.
- Le pilote est écrit en **Go pur**, sans cgo et sans dépendance externe. Il colle donc à l'hypothèse « un seul `.exe` » du SPEC.
- Le programme de test [`paneltest`](../paneltest.md) permet d'éprouver chaque commande physique et affiche des données caméra simulées sur le panneau : ND, CC, GAIN, SHUTTER, IRIS, MASTER BLACK, numéro de caméra et tally.

Conséquence pour le projet : le module `surface-micro-panel` n'est plus un risque. Le panneau n'a pas besoin de se faire passer pour un ATEM.

## 2. Démarche (multi-agents)

| Rôle | Qui / modèle | Pourquoi ce modèle | Résultat |
|---|---|---|---|
| Orchestration, sondes USB en direct, pilote Go, intégration, docs | Agent principal (Opus) | Raisonnement protocolaire et décisions d'architecture | Descripteur HID, authentification, captures, pilote, carte des LEDs |
| Rétro-ingénierie statique d'**ATEM Software Control.exe** (Windows) | Sous-agent Opus | Désassemblage x64 long et difficile | Codage 7 segments, police et format de la matrice, format du rapport LEDs, LEDs caméra, rôle des rapports feature |
| Analyse du DMG **Middle Control 4.0.4** (macOS) | Sous-agent Opus | Extraction DMG et analyse de binaire Mach-O | **Piste négative** : Middle Control ne parle pas au panneau en USB (voir §6) |
| Interface web du programme de test | Sous-agent Sonnet | Travail de front-end bien spécifié, rapide et peu coûteux | `cmd/paneltest/web/index.html` (panneau virtuel, checklist, journal) |

Les tests sur le matériel ont été faits avec le propriétaire, qui manipulait le panneau et envoyait des photos :
1. une capture authentifiée de toutes les commandes, dans un ordre imposé ;
2. trois photos de validation des sorties ;
3. sept photos à motifs binaires plus une photo de contrôle pour cartographier les 75 LEDs.

## 3. Ce qui a été trouvé

Détails octet par octet : [atem-micro-camera-panel-protocol.md](../atem-micro-camera-panel-protocol.md).

| Sujet | Résultat | Source |
|---|---|---|
| Identité USB | VID 0x1EDB / PID 0xBF01. HID sur MI_02 (page 0xFF00). MI_00 est une interface « Media » qui ne sert pas aux entrées/sorties du panneau | `Get-PnpDevice`, descripteur HID |
| Authentification | Rapport feature 0x05 : `00` → défi, `01`, `03` + réponse → `04` + délai de 600 s. Algorithme du Speed Editor | Essai en direct (réussi du premier coup) et code d'ATEM SC |
| Boutons | Rapport 0x03 : jusqu'à 6 codes u16 maintenus. Caméras 0x40–0x47, scènes 0x48–0x4C, STORE 0x4D, flèches 0x4E–0x55, ON/WB/BARS 0x56–0x58, bas du panneau 0x59–0x5E | Capture guidée |
| Molettes | Rapport 0x0E : 9 valeurs u16 absolues 0–1024, lisibles aussi en feature (lues à la connexion) | Capture guidée |
| Levier et bague | Rapport 0x0F : int32 `[1, levier, bague]`, ±4096 | Capture guidée |
| Afficheurs | Rapport 0x10 : 14 octets, bit 7 = a … bit 1 = g, bit 0 = point. Ordre : ND, CC, GAIN×2, SHUTTER×4, IRIS×3, MB×3 | ATEM SC (« Auto » = EE 38 1E 3A) et photo |
| Matrice | Rapport 0x11 : 7×10 pixels, 2 bits par pixel (1 rouge, 2 vert, 3 ambre), polices 1 et 2 chiffres | ATEM SC et photo |
| LEDs | Rapport 0x02 : bitmap de 30 octets suivi d'un numéro de couche. 75 LEDs sur la couche 0 (carte complète dans la doc protocole) | ATEM SC et 8 photos |
| Autres rapports feature | Batterie (0x06), affectation des entrées (0x09/0x0A), scene files stockés **dans le panneau** (0x12/0x13), nom (0x15) | ATEM SC (non testé) |

Piège technique : les rapports feature doivent être lus et écrits **à leur taille exacte**. Avec un tampon complété à la taille maximale, le panneau répond `ERROR_GEN_FAILURE`. Le pilote en tient compte.

## 4. Ce qui a été livré

| Chemin | Contenu |
|---|---|
| [`internal/surfaces/micropanel/`](../../internal/surfaces/micropanel) | Pilote : accès HID Windows en Go pur (`setupapi`/`hid.dll`), authentification, décodage des entrées, encodeurs des afficheurs, de la matrice et des LEDs, ré-authentification périodique |
| [`cmd/paneltest/`](../../cmd/paneltest) | Banc de test : serveur local, caméras simulées, interface web embarquée, checklist des 42 commandes, explorateur de LEDs |
| [`cmd/panelprobe/`](../../cmd/panelprobe/main.go) | Outil bas niveau : dump hexadécimal, écritures brutes, démo |
| [`docs/atem-micro-camera-panel-protocol.md`](../atem-micro-camera-panel-protocol.md) | Référence du protocole |
| [`docs/paneltest.md`](../paneltest.md) | Mode d'emploi du banc de test |

Lancer le banc :

```bash
go run ./cmd/paneltest
```

## 5. Vérification

| Vérification | Résultat |
|---|---|
| `go vet ./...` | OK |
| `go test ./...` | OK : réponse d'authentification testée contre 3 défis réels acceptés par le panneau, police 7 segments comparée aux octets d'ATEM SC, encodage de texte |
| Connexion en Go pur sur le vrai panneau | OK : authentification, session de 10 min, lecture des positions initiales |
| Afficheurs, matrice, LEDs caméra | OK, vérifiés sur photo (`2`, `A`, `12`, `50`, `F2.8`, `-05`, matrice « 1 », boutons rouge/vert/cyan/ambre) |
| Carte des 75 LEDs | OK : 7 motifs binaires plus 1 motif de contrôle conforme à la prédiction |
| Boutons, molettes, levier, bague | Codes vérifiés par capture. **La checklist complète dans `paneltest` reste à faire par le propriétaire** (voir §7) |

Ce qui n'a pas été vérifié :
- l'écriture des positions des molettes (rapport 0x0E en feature) ;
- les couches LEDs 1 à 5 ;
- les rapports 0x06, 0x09–0x0B, 0x12–0x15.

## 6. Pistes écartées

- **Middle Control (macOS)** pilote le panneau *à travers un vrai mixeur ATEM*, via son option « ATEM Link » et le SDK `BMDSwitcherAPI` ; le panneau y est branché au mixeur. Le binaire ne contient aucun code HID pour ce panneau. Il est inutile pour notre cas sans mixeur.
- **Se faire passer pour un mixeur ATEM** (hypothèse 3 du SPEC) : inutile, le panneau se pilote directement en HID.

## 7. Points ouverts

1. **Noms des codes 0x59–0x5E.** Ils sont déduits de l'ordre d'appui : 0x59 Black/Flare (jamais vu en capture), 0x5A IRIS/MB ACTIVE, 0x5B AUTO IRIS, 0x5C CALL, 0x5D PANEL ACTIVE, 0x5E PREVIEW. À confirmer avec le journal de `paneltest`.
2. **Bouton carré Black/Flare** : aucun code observé et aucune LED trouvée. Est‑ce vraiment un bouton ?
3. **Molettes** : peut‑on écrire leur position (feature 0x0E) pour les resynchroniser sur la valeur réelle d'une caméra ? C'est important pour l'UX (pas de saut de valeur au changement de caméra).
4. **Couches LEDs 1–5** (clignotement ? luminosité ?) et **bits au‑delà de 74**, qui perturbent la matrice.
5. Pas encore de **build tag ni de version non‑Windows** du pilote. Linux et macOS restent « nice to have » dans le SPEC.
6. ATEM Software Control et `paneltest` ne doivent pas tourner en même temps.

## 8. Impact sur le SPEC et prochaines étapes

- La **question ouverte n°1 est résolue**. Le panneau est un périphérique HID authentifié, utilisable sans mixeur.
- **L'hypothèse 2 est confirmée** : Go convient, sans cgo.
- L'interface `Surface` du `core` peut s'appuyer directement sur `micropanel.State` (entrées) et sur les encodeurs d'afficheurs, de matrice et de LEDs (retour d'état).
- Prochaines étapes proposées :
  1. Faire la checklist complète dans `paneltest` et confirmer les points 1 et 2 du §7.
  2. Tester l'écriture de la feature 0x0E.
  3. Rédiger `SPEC-surface-micro-panel.md` à partir de ce rapport.
  4. Démarrer `core`, puis `camera-blackmagic` (REST de la Studio 6K Pro).
