# Commandes courantes du projet. Utilisation : make <cible> (make help pour la liste).
# Fonctionne sous Linux, macOS et Windows (make installé via Chocolatey, Scoop ou winget).

ifeq ($(OS),Windows_NT)
	EXT := .exe
	RM_BIN := if exist bin rmdir /s /q bin
else
	EXT :=
	RM_BIN := rm -rf bin
endif

BIN := bin/gdt-client$(EXT)

.PHONY: help run build test vet fmt clean

help:
	@echo Cibles disponibles :
	@echo   make run    - lance le client web
	@echo   make build  - compile le client dans bin/
	@echo   make test   - lance les tests
	@echo   make vet    - analyse statique du code
	@echo   make fmt    - formate le code
	@echo   make clean  - supprime bin/

run:
	go run ./cmd/client

build:
	go build -o $(BIN) ./cmd/client

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	$(RM_BIN)
