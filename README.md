# GDT — Client

Client web de l'application **GDT**, une application web de gestion de notes organisées par espaces, réalisée dans le cadre d'un test technique.

Développé en Go, il génère les pages HTML avec le moteur `html/template` et communique avec le serveur ([AppGDT-Server](https://github.com/CookieG77/AppGDT-Server)) via son API HTTP. Le navigateur ne dialogue qu'avec ce client : c'est lui qui appelle l'API.

```
Navigateur ──HTML / formulaires──▶ Client Go ──JSON + JWT──▶ API (AppGDT-Server) ──▶ PostgreSQL
```

> 🚧 Projet en cours de développement.

## Stack technique

- **Go**
- **`net/http`** : serveur HTTP et routage (motifs `GET /chemin/{id}` de Go 1.22+)
- **`html/template`** : rendu des pages, avec échappement automatique contre les failles XSS
- **`embed`** : templates et fichiers statiques intégrés au binaire
- **`log/slog`** : logs structurés en JSON
- **[godotenv](https://github.com/joho/godotenv)** : chargement du fichier `.env` (même bibliothèque que le serveur)

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
| `API_TIMEOUT`  | `5s`                    | Durée maximale d'un appel à l'API |
| `TLS_CERT_FILE` | *(vide)*               | Certificat TLS (PEM) : active HTTPS avec `TLS_KEY_FILE` |
| `TLS_KEY_FILE`  | *(vide)*               | Clé privée du certificat (PEM) |
| `TLS_HSTS`      | `false`                | Envoie l'en-tête HSTS (HTTPS uniquement, à réserver à la production) |

## Lancement

```bash
go run ./cmd/client
```

Le client est ensuite accessible sur <http://localhost:3000>.

Pour lancer les tests :

```bash
go test ./...
```

Pour produire un exécutable :

```bash
go build -o gdt-client ./cmd/client
```

## HTTPS (optionnel)

Le JWT de l'utilisateur est stocké dans un cookie : sans HTTPS, il circule en clair entre le navigateur et le client. Pour chiffrer ces échanges, fournir un certificat et sa clé privée :

```env
TLS_CERT_FILE=certs/localhost.pem
TLS_KEY_FILE=certs/localhost-key.pem
```

Le client est alors accessible sur <https://localhost:3000>. Seul TLS 1.2 ou plus récent est accepté.

**Générer un certificat de développement** avec [mkcert](https://github.com/FiloSottile/mkcert), qui crée une autorité locale reconnue par le navigateur (pas d'avertissement) :

```bash
mkcert -install
mkdir certs
mkcert -cert-file certs/localhost.pem -key-file certs/localhost-key.pem localhost 127.0.0.1 ::1
```

Sans mkcert, un certificat auto-signé fonctionne aussi, mais le navigateur affichera un avertissement :

```bash
go run "$(go env GOROOT)/src/crypto/tls/generate_cert.go" --host localhost
# produit cert.pem et key.pem dans le dossier courant
```

Le dossier `certs/` et les fichiers `*.pem` / `*.key` sont ignorés par Git : **une clé privée ne doit jamais être versionnée.**

`TLS_HSTS=true` demande au navigateur de n'utiliser que HTTPS pendant un an. À ne pas activer sur `localhost`, où la règle s'appliquerait à tous les ports.

## Structure du projet

```
cmd/client/          Point d'entrée : configuration, templates, routes, arrêt propre
internal/
  apiclient/         Client HTTP typé vers l'API (modèles, appels, erreurs de l'API)
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
  static/            CSS, polices embarquées (licence OFL) et images
```
