.DEFAULT_GOAL := help

# Ohne Vorgabewert: sichern fällt auf einen Standardtext zurück.
MSG ?=
# Über die Umgebung statt in der Kommandozeile: eine mehrzeilige Message würde
# die Rezeptzeile sonst zerreißen, weil make sie zeilenweise an die Shell gibt.
export MSG

# Beide Wurzeln aus dem Ort dieser Datei ableiten, nicht aus dem
# Arbeitsverzeichnis: so funktioniert auch make -C k-playbook-local sichern.
LOCAL := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO := $(abspath $(LOCAL)/..)

.PHONY: help sichern

help: ## Zeigt diese Hilfe an
	@echo "Targets:"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Parameter:"
	@echo "  MSG=...                Commit-Message, optional für sichern"
	@echo ""
	@echo "Repo:  $(REPO)"
	@echo ""

sichern: ## Committet alles und pusht ohne Prüfung (MSG optional)
	@git -C "$(REPO)" add -A; \
	if git -C "$(REPO)" diff --cached --quiet; then \
	  printf 'nichts zu committen\n'; \
	else \
	  printf '%s\n' "$${MSG:-Zwischenstand}" | git -C "$(REPO)" commit -q -F - && \
	  printf 'committet\n'; \
	fi
	@git -C "$(REPO)" push
