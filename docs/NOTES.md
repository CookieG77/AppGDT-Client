Fichier de notes servant a conservé les choix réalisés au fur et à mesure du development des deux applications,
ainsi que des justifications associées.
⚠️ Ceci n'est pas un des éléments du rendu, juste des notes personnelles que j'ai utilisé pour ne pas oublier des éléments dans mon rapport.

### 28/09 :

- J'ai mis en place le serveur backend for frontend rapidement étant donné qu'une bonne partie de la structure et des fichiers sont les mêmes que pour l'API.

- J'ai mis en place la possibilité d'utilisé un certificat pour la sécurité. Bien sûr, pour tester, j'ai utilisé un certificat auto-signé.
  Mais pour un vrai déploiement, il faudrait utiliser une automatisation pour avoir des certificats venant de certificateurs reconnue et qui sont recréé dès qu'ils ne sont plus valides.
- J'ai aussi mis en place mon package pour envoyer les requêtes à l'API du serveur en utilisant des structs pour forcer une structure de requête sûr.