.DEFAULT_GOAL := help

# Ohne Vorgabewert: sichern fällt auf einen Standardtext zurück.
MSG ?=
# Ohne Vorgabewert: release verlangt die Version ausdrücklich.
VERSION ?=
# Über die Umgebung statt in der Kommandozeile: eine mehrzeilige Message würde
# die Rezeptzeile sonst zerreißen, weil make sie zeilenweise an die Shell gibt.
export MSG

# Beide Wurzeln aus dem Ort dieser Datei ableiten, nicht aus dem
# Arbeitsverzeichnis: so funktioniert auch make -C k-playbook-local sichern.
LOCAL := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO := $(abspath $(LOCAL)/..)

.PHONY: help sichern release

help: ## Zeigt diese Hilfe an
	@echo "Targets:"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Parameter:"
	@echo "  MSG=...                Commit-Message, optional für sichern"
	@echo "  VERSION=vX.Y.Z         Version für release, Pflicht; mit Suffix"
	@echo "                         (vX.Y.Z-rc1) eine Vorabversion"
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

# Ein Release ist nur der Tag: Bauen, Prüfen und Veröffentlichen übernimmt
# .github/workflows/release.yml. Getaggt wird genau der Stand, der auf
# origin/main liegt — sauber, auf main, gepusht —, damit CI dasselbe baut, was
# hier geprüft wurde.
release: ## Legt den Tag VERSION an und pusht ihn; CI baut das Release
	@set -eu; \
	  version="$(VERSION)"; \
	  test -n "$$version" || { \
	    printf 'Aufruf: make -C k-playbook-local release VERSION=v0.1.0\n' >&2; exit 1; \
	  }; \
	  printf '%s\n' "$$version" | \
	    grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$$' || { \
	    printf 'Keine gültige Version: %s (erwartet vX.Y.Z oder vX.Y.Z-suffix)\n' "$$version" >&2; \
	    exit 1; \
	  }; \
	  branch="$$(git -C "$(REPO)" symbolic-ref --quiet --short HEAD || true)"; \
	  test "$$branch" = main || { \
	    printf 'Release nur von main, aktuell: %s\n' "$${branch:-(kein Branch)}" >&2; exit 1; \
	  }; \
	  dirty="$$(git -C "$(REPO)" status --porcelain)"; \
	  test -z "$$dirty" || { \
	    printf 'Der Arbeitsbaum ist nicht sauber; getaggt wird nur ein committeter Stand:\n%s\n' "$$dirty" >&2; \
	    exit 1; \
	  }; \
	  git -C "$(REPO)" fetch --quiet origin main; \
	  head="$$(git -C "$(REPO)" rev-parse HEAD)"; \
	  remote="$$(git -C "$(REPO)" rev-parse origin/main)"; \
	  test "$$head" = "$$remote" || { \
	    printf 'main ist nicht gepusht oder nicht aktuell (HEAD %s, origin/main %s).\n' "$$head" "$$remote" >&2; \
	    exit 1; \
	  }; \
	  if git -C "$(REPO)" rev-parse --verify --quiet "refs/tags/$$version" >/dev/null; then \
	    printf 'Den Tag %s gibt es lokal bereits.\n' "$$version" >&2; exit 1; \
	  fi; \
	  if git -C "$(REPO)" ls-remote --exit-code --tags origin "refs/tags/$$version" >/dev/null 2>&1; then \
	    printf 'Den Tag %s gibt es auf origin bereits.\n' "$$version" >&2; exit 1; \
	  fi; \
	  git -C "$(REPO)" tag -a "$$version" -m "Release $$version"; \
	  git -C "$(REPO)" push origin "refs/tags/$$version"; \
	  printf '\nTag %s gepusht. Der Workflow „Release“ baut und veröffentlicht jetzt:\n' "$$version"; \
	  printf '  gh run list --workflow release.yml --limit 1\n'
