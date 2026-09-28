/*
 * Rendu Markdown des notes, entièrement côté navigateur.
 *
 * Le contenu est enregistré tel quel (texte brut) par l'API : ce script ne
 * fait qu'améliorer l'affichage. Sans JavaScript, la note reste lisible en
 * texte brut et tous les formulaires fonctionnent.
 *
 * Sécurité : marked transforme le Markdown en HTML sans rien filtrer, donc un
 * contenu malveillant (<script>, onerror=…, liens javascript:) passerait tel
 * quel. Tout le HTML produit passe par DOMPurify avant d'être inséré dans la
 * page. La CSP du client (script-src 'self') bloque en plus tout script en
 * ligne qui passerait malgré tout.
 */
(function () {
    "use strict";

    if (!window.marked || !window.DOMPurify) {
        return; // bibliothèques absentes : on garde le texte brut
    }

    marked.setOptions({
        gfm: true,     // tableaux, listes de tâches, texte barré…
        breaks: true   // un retour à la ligne simple reste un retour à la ligne
    });

    var PURIFY_OPTIONS = {
        USE_PROFILES: { html: true },
        FORBID_TAGS: ["style", "form", "button", "textarea", "select", "option", "iframe", "object", "embed"],
        FORBID_ATTR: ["style", "id", "name"]
    };

    /* Markdown -> fragment HTML nettoyé et adapté à la page. */
    function render(source, firstHeadingLevel) {
        var clean = DOMPurify.sanitize(marked.parse(source), PURIFY_OPTIONS);
        var template = document.createElement("template");
        template.innerHTML = clean;
        var root = template.content;

        shiftHeadings(root, firstHeadingLevel);
        secureLinks(root);
        replaceImages(root);
        describeCheckboxes(root);
        return root;
    }

    /*
     * Le titre de la note est déjà le <h1> de la page : les titres de la note
     * sont décalés pour que le plus haut devienne un <h2>, sans sauter de
     * niveau, afin de garder une hiérarchie cohérente pour les lecteurs d'écran.
     */
    function shiftHeadings(root, firstLevel) {
        var headings = root.querySelectorAll("h1, h2, h3, h4, h5, h6");
        if (headings.length === 0) {
            return;
        }
        var highest = 6;
        headings.forEach(function (h) {
            highest = Math.min(highest, parseInt(h.tagName.charAt(1), 10));
        });
        var offset = firstLevel - highest;
        if (offset === 0) {
            return;
        }
        headings.forEach(function (h) {
            var level = Math.max(1, Math.min(6, parseInt(h.tagName.charAt(1), 10) + offset));
            var replacement = document.createElement("h" + level);
            while (h.firstChild) {
                replacement.appendChild(h.firstChild);
            }
            h.replaceWith(replacement);
        });
    }

    /* Les liens externes s'ouvrent dans le même onglet, sans transmettre la page d'origine. */
    function secureLinks(root) {
        root.querySelectorAll("a[href]").forEach(function (a) {
            var url;
            try {
                url = new URL(a.getAttribute("href"), window.location.href);
            } catch (e) {
                a.removeAttribute("href");
                return;
            }
            if (url.origin !== window.location.origin) {
                a.setAttribute("rel", "noopener noreferrer nofollow");
            }
        });
    }

    /*
     * Images : la CSP n'autorise que les images du client lui-même, et une
     * image externe pourrait servir à suivre la lecture de la note. Elles
     * sont remplacées par un lien vers l'image.
     */
    function replaceImages(root) {
        root.querySelectorAll("img").forEach(function (img) {
            var src = img.getAttribute("src") || "";
            var label = "Image" + (img.alt ? " : " + img.alt : "");
            var replacement;
            if (/^https?:\/\//i.test(src)) {
                replacement = document.createElement("a");
                replacement.href = src;
                replacement.rel = "noopener noreferrer nofollow";
            } else {
                replacement = document.createElement("span");
            }
            replacement.textContent = "[" + label + "]";
            img.replaceWith(replacement);
        });
    }

    /* Listes de tâches « - [x] » : cases en lecture seule, avec un nom accessible. */
    function describeCheckboxes(root) {
        root.querySelectorAll("input").forEach(function (input) {
            if (input.type !== "checkbox") {
                input.remove();
                return;
            }
            input.disabled = true;
            input.setAttribute("aria-label", input.checked ? "Tâche faite" : "Tâche à faire");
        });
    }

    /* Consultation : remplace le texte brut par le rendu. */
    function renderNoteContents() {
        document.querySelectorAll("[data-markdown]").forEach(function (el) {
            var level = parseInt(el.getAttribute("data-first-heading") || "2", 10);
            var fragment = render(el.textContent, level);
            el.textContent = "";
            el.appendChild(fragment);
            el.classList.add("markdown");
            if (el.hasAttribute("data-tasks-url")) {
                enableTasks(el);
            }
        });
    }

    /*
     * Listes de tâches cliquables : chaque case cochée ou décochée est
     * enregistrée aussitôt (POST /notes/{id}/tasks, avec le jeton CSRF).
     * Le serveur modifie la ligne correspondante du texte Markdown.
     * En cas d'échec, la case reprend son état et un message est annoncé.
     */
    function enableTasks(el) {
        var url = el.getAttribute("data-tasks-url");
        var csrf = el.getAttribute("data-csrf");
        var status = document.getElementById("note-tasks-status");

        el.querySelectorAll('input[type="checkbox"]').forEach(function (box, index) {
            var item = box.closest("li");
            var label = item ? item.textContent.replace(/\s+/g, " ").trim() : "";
            box.disabled = false;
            box.setAttribute("aria-label", label || "Tâche");
            if (item) {
                item.classList.add("task");
            }

            box.addEventListener("change", function () {
                var done = box.checked;
                box.disabled = true;
                announce(status, "");
                fetch(url, {
                    method: "POST",
                    headers: { "Content-Type": "application/x-www-form-urlencoded" },
                    body: new URLSearchParams({ csrf_token: csrf, index: String(index), done: String(done) }),
                    credentials: "same-origin"
                }).then(function (response) {
                    if (!response.ok) {
                        throw response.status;
                    }
                    announce(status, (done ? "Tâche cochée : " : "Tâche décochée : ") + label);
                }).catch(function (code) {
                    box.checked = !done;
                    announce(status, code === 401
                        ? "Votre session a expiré : rechargez la page pour vous reconnecter."
                        : "La tâche n'a pas pu être enregistrée. Rechargez la page et réessayez.");
                }).finally(function () {
                    box.disabled = false;
                });
            });
        });
    }

    function announce(region, message) {
        if (region) {
            region.textContent = message;
        }
    }

    /* Extraits dans les listes : on n'en garde que le texte, sans la syntaxe (**, ##, …). */
    function renderExcerpts() {
        document.querySelectorAll("[data-markdown-excerpt]").forEach(function (el) {
            var text = render(el.textContent, 2).textContent.replace(/\s+/g, " ").trim();
            if (text) {
                el.textContent = text;
            }
        });
    }

    /*
     * Édition : onglets « Écrire » / « Aperçu » au-dessus de la zone de texte,
     * selon le motif d'onglets de l'ARIA Authoring Practices Guide
     * (flèches gauche/droite pour passer d'un onglet à l'autre).
     */
    function setupEditors() {
        document.querySelectorAll("textarea[data-markdown-editor]").forEach(function (textarea) {
            var id = textarea.id;
            var field = textarea.closest(".field");

            var tablist = document.createElement("div");
            tablist.className = "editor-tabs";
            tablist.setAttribute("role", "tablist");
            tablist.setAttribute("aria-label", "Mode d'édition du contenu");

            var writeTab = makeTab(id + "-tab-write", "Écrire", id + "-panel-write", true);
            var previewTab = makeTab(id + "-tab-preview", "Aperçu", id + "-panel-preview", false);
            tablist.append(writeTab, previewTab);

            // La zone de texte est enveloppée dans le premier panneau
            var writePanel = document.createElement("div");
            writePanel.id = id + "-panel-write";
            writePanel.setAttribute("role", "tabpanel");
            writePanel.setAttribute("aria-labelledby", writeTab.id);
            textarea.parentNode.insertBefore(writePanel, textarea);
            writePanel.appendChild(textarea);

            var previewPanel = document.createElement("div");
            previewPanel.id = id + "-panel-preview";
            previewPanel.className = "editor-preview markdown";
            previewPanel.setAttribute("role", "tabpanel");
            previewPanel.setAttribute("aria-labelledby", previewTab.id);
            previewPanel.tabIndex = 0;
            previewPanel.hidden = true;

            writePanel.parentNode.insertBefore(tablist, writePanel);
            writePanel.parentNode.insertBefore(previewPanel, writePanel.nextSibling);
            if (field) {
                field.classList.add("field--editor");
            }

            function select(tab) {
                var preview = tab === previewTab;
                if (preview) {
                    previewPanel.textContent = "";
                    if (textarea.value.trim() === "") {
                        var empty = document.createElement("p");
                        empty.className = "editor-preview__empty";
                        empty.textContent = "Rien à prévisualiser pour l'instant.";
                        previewPanel.appendChild(empty);
                    } else {
                        previewPanel.appendChild(render(textarea.value, 2));
                    }
                }
                writeTab.setAttribute("aria-selected", String(!preview));
                previewTab.setAttribute("aria-selected", String(preview));
                writeTab.tabIndex = preview ? -1 : 0;
                previewTab.tabIndex = preview ? 0 : -1;
                writePanel.hidden = preview;
                previewPanel.hidden = !preview;
            }

            [writeTab, previewTab].forEach(function (tab) {
                tab.addEventListener("click", function () { select(tab); });
                tab.addEventListener("keydown", function (event) {
                    if (event.key === "ArrowRight" || event.key === "ArrowLeft" || event.key === "Home" || event.key === "End") {
                        event.preventDefault();
                        var other = tab === writeTab ? previewTab : writeTab;
                        var target = event.key === "Home" ? writeTab : event.key === "End" ? previewTab : other;
                        select(target);
                        target.focus();
                    }
                });
            });
        });
    }

    function makeTab(id, label, panelId, selected) {
        var tab = document.createElement("button");
        tab.type = "button";
        tab.id = id;
        tab.className = "editor-tab";
        tab.textContent = label;
        tab.setAttribute("role", "tab");
        tab.setAttribute("aria-controls", panelId);
        tab.setAttribute("aria-selected", String(selected));
        tab.tabIndex = selected ? 0 : -1;
        return tab;
    }

    renderNoteContents();
    renderExcerpts();
    setupEditors();
})();
