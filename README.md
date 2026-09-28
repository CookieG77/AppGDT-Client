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

**Comptes de démonstration** : depuis le dépôt [AppGDT-Server](https://github.com/CookieG77/AppGDT-Server), `go run ./cmd/seed` crée `demo@example.com` et `camille@example.com` (mot de passe `Demo1234!`), avec des espaces et des notes d'exemple.

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

## Authentification et sécurité des formulaires

- **Session** : après la connexion, le JWT renvoyé par l'API est gardé dans un cookie `HttpOnly` (illisible par JavaScript), `SameSite=Lax`, qui expire en même temps que le token. En HTTPS, il est aussi `Secure` et préfixé `__Host-`.
- **Vérification** : à chaque page, le client demande à l'API qui est connecté (`GET /users/me`). Un token refusé (expiré, compte supprimé) est retiré du navigateur.
- **Pages protégées** : un visiteur est redirigé vers `/login`, puis ramené à la page demandée une fois connecté (seules les adresses internes sont acceptées, pour éviter les redirections ouvertes). Ces pages ne sont pas mises en cache (`Cache-Control: no-store`).
- **CSRF** : chaque formulaire contient un jeton aléatoire, comparé à celui du cookie `gdt_csrf`. Les envois marqués par le navigateur comme venant d'un autre site (`Sec-Fetch-Site`) sont refusés.
- **Déconnexion** : le cookie est supprimé. Le JWT reste techniquement valide jusqu'à son expiration, l'API ne permettant pas de le révoquer.

## Pages

| Adresse | Page |
|---|---|
| `/` | Accueil |
| `/login`, `/register` | Connexion, inscription (visiteurs uniquement) |
| `/privacy` | Confidentialité : données conservées, cookies, droits RGPD |
| `/spaces` | Mes espaces |
| `/spaces/new` | Nouvel espace |
| `/spaces/{id}` | Un espace et ses notes |
| `/spaces/{id}/edit`, `/spaces/{id}/delete` | Modification, suppression (avec confirmation) |
| `/spaces/{id}/notes/new` | Nouvelle note dans l'espace |
| `/notes/{id}` | Une note, en mode consultation |
| `/notes/{id}/edit` | Une note, en mode édition |
| `/notes/{id}/delete` | Suppression d'une note (avec confirmation) |
| `/account` | Mon compte : informations, export et suppression des données |
| `/account/export` | Téléchargement de toutes les données du compte (JSON) |
| `/account/delete` | Suppression du compte (mot de passe + confirmation) |

Les formulaires HTML ne connaissant que `GET` et `POST`, les modifications sont envoyées en `POST` au client, qui appelle l'API en `PUT` ou `DELETE`. Toutes les pages fonctionnent sans JavaScript.

## Droits sur les données (RGPD)

La page **Mon compte** (lien sur le pseudo, dans l'en-tête) donne accès aux deux droits proposés par l'API :

- **Portabilité / accès** : téléchargement d'un fichier JSON contenant le profil, les espaces et toutes les notes (`GET /users/me/export`). Le mot de passe n'est jamais exporté.
- **Effacement** : suppression définitive du compte et de toutes ses données (`DELETE /users/me`). Le mot de passe est redemandé et une case de confirmation doit être cochée ; ensuite la session est fermée.

La page publique **Confidentialité** (`/privacy`, liée dans le pied de page et sous le formulaire d'inscription) détaille les données conservées, leurs finalités et bases légales, leurs durées de conservation, les cookies (tous strictement nécessaires, donc sans bandeau de consentement) et la façon d'exercer ses droits. En cas de déploiement réel, elle doit être complétée avec l'identité et le contact du responsable du traitement.

## Markdown dans les notes

Le contenu des notes peut être mis en forme en Markdown. **L'API n'est pas concernée** : le texte est enregistré tel quel, et c'est le navigateur qui l'interprète à l'affichage (`web/static/js/markdown.js`).

- **Consultation** : le texte brut est remplacé par son rendu HTML. Les titres sont décalés pour commencer au niveau `<h2>` (le titre de la note est le `<h1>`), et les cases des listes de tâches reçoivent un nom accessible.
- **Édition** : onglets « Écrire » / « Aperçu » (motif d'onglets ARIA, navigation aux flèches) et aide repliable listant la syntaxe.
- **Listes** : les extraits de notes sont affichés sans la syntaxe (`**`, `##`…).
- **Sécurité** : [marked](https://github.com/markedjs/marked) ne filtre pas le HTML, donc tout le rendu passe par [DOMPurify](https://github.com/cure53/DOMPurify) (suppression des `<script>`, attributs `on…`, liens `javascript:`…). Les images externes sont remplacées par un lien (elles seraient bloquées par la CSP et pourraient servir à suivre la lecture). Les deux bibliothèques sont servies par le client (`web/static/vendor/`, licences jointes) : aucune ressource externe n'est chargée.
- **Sans JavaScript** : la note s'affiche en texte brut, avec ses retours à la ligne. Tout reste utilisable.

## Structure du projet

```
cmd/client/          Point d'entrée : configuration, templates, routes, arrêt propre
internal/
  apiclient/         Client HTTP typé vers l'API (modèles, appels, erreurs de l'API)
  config/            Chargement de la configuration (.env + variables d'environnement)
  handler/           Handlers HTTP : lisent la requête, appellent l'API, rendent une page
  middleware/        Logs, en-têtes de sécurité, panics, session, pages protégées, CSRF
  server/            Déclaration des routes et création du serveur HTTP (+ tests de bout en bout)
  session/           Cookies (JWT, message flash, jeton CSRF) et données de la requête
  view/              Chargement des templates et rendu des pages
web/
  templates/
    layouts/         Squelette HTML commun (base.html)
    partials/        Morceaux réutilisables (en-tête, champs de formulaire, messages…)
    pages/           Une page par fichier, qui définit le bloc "content"
  static/            CSS, JavaScript (rendu Markdown), bibliothèques (vendor/), polices et images
```
