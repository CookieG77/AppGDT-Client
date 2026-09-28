# GDT — Client

Client web de l'application **GDT**, une application web de gestion de notes organisées par espaces, réalisée dans le cadre d'un test technique.

Développé en Go, il génère les pages HTML avec le moteur `html/template` et communique avec le serveur ([AppGDT-Server](https://github.com/CookieG77/AppGDT-Server)) via son API HTTP. Le navigateur ne dialogue qu'avec ce client : c'est lui qui appelle l'API.

```
Navigateur ──HTML / formulaires──▶ Client Go ──JSON + JWT──▶ API (AppGDT-Server) ──▶ PostgreSQL
```

> 🚧 Projet en cours de développement.

## Stack technique

- **Go** (bibliothèque standard uniquement)
- **`net/http`** : serveur HTTP et routage (motifs `GET /chemin/{id}` de Go 1.22+)
- **`html/template`** : rendu des pages, avec échappement automatique contre les failles XSS
- **`embed`** : templates et fichiers statiques intégrés au binaire
- **`log/slog`** : logs structurés en JSON

## Prérequis

- Go 1.27 ou plus récent
- Le serveur [AppGDT-Server](https://github.com/CookieG77/AppGDT-Server) lancé et accessible

## Configuration

Copier `.env.example` en `.env` puis adapter les valeurs. Les variables déjà définies dans l'environnement sont prioritaires sur le fichier.

| Variable       | Défaut                  | Description                   |
|----------------|-------------------------|-------------------------------|
| `PORT`         | `3000`                  | Port d'écoute du client       |
| `ADDRESS`      | `localhost`             | Adresse d'écoute du client    |
| `API_BASE_URL` | `http://localhost:8080` | URL de base de l'API serveur  |

## Lancement

```bash
go run ./cmd/client
```

Le client est ensuite accessible sur <http://localhost:3000>.

Pour produire un exécutable :

```bash
go build -o gdt-client ./cmd/client
```

## Structure du projet

```
cmd/client/          Point d'entrée : configuration, templates, routes, arrêt propre
internal/
  config/            Chargement de la configuration (.env + variables d'environnement)
  handler/           Handlers HTTP : lisent la requête, appellent l'API, rendent une page
  middleware/        Logs des requêtes, en-têtes de sécurité, récupération des panics
  server/            Déclaration des routes et création du serveur HTTP
  view/              Chargement des templates et rendu des pages
web/
  templates/
    layouts/         Squelette HTML commun (base.html)
    partials/        Morceaux réutilisables (navigation…)
    pages/           Une page par fichier, qui définit le bloc "content"
  static/            CSS et autres fichiers statiques
```
