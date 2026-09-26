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

# Für den CI-Status in release. Ausdrücklich gesetzt statt aus origin
# abgeleitet: so läuft die Prüfung auch in einer Kopie, deren origin kein
# GitHub ist.
GH_REPO := kephalaion/kephalaion

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
	@echo "Branches:"
	@echo "  dev                    Arbeitsstand; sichern nur hier, ohne Prüfung"
	@echo "  main                   nur veröffentlichte Stände; rückt allein über"
	@echo "                         release per Fast-Forward auf dev vor"
	@echo ""
	@echo "CI (ci.yml):"
	@echo "  quick                  Push auf dev: make check-quick"
	@echo "  full                   Push auf main, Pull Requests, von Hand; release"
	@echo "                         verlangt einen grünen Lauf auf dev für HEAD"
	@echo ""
	@echo "Repo:  $(REPO)"
	@echo ""

# Gesichert wird nur auf dev, und zwar ohne Prüfung: dev ist die Sicherung
# des Arbeitsstands. Gepusht wird ausdrücklich nach origin dev, auch ohne
# Upstream, und nie mit Force — das Ruleset auf GitHub sperrt ihn ohnehin.
sichern: ## Committet alles auf dev und pusht nach origin dev, ohne Prüfung (MSG optional)
	@set -eu; \
	  branch="$$(git -C "$(REPO)" symbolic-ref --quiet --short HEAD || true)"; \
	  test "$$branch" = dev || { \
	    printf 'Gesichert wird nur auf dev, aktuell: %s\n' "$${branch:-(kein Branch)}" >&2; \
	    printf 'Erst wechseln: git switch dev\n' >&2; \
	    exit 1; \
	  }; \
	  git -C "$(REPO)" add -A; \
	  if git -C "$(REPO)" diff --cached --quiet; then \
	    printf 'nichts zu committen\n'; \
	  else \
	    printf '%s\n' "$${MSG:-Zwischenstand}" | git -C "$(REPO)" commit -q -F -; \
	    printf 'committet\n'; \
	  fi; \
	  git -C "$(REPO)" push origin dev || { \
	    printf '\nDer Push nach origin dev ist gescheitert. Ist dev auf origin weiter, erst\n' >&2; \
	    printf 'holen und dann erneut sichern:\n' >&2; \
	    printf '  git pull --no-rebase origin dev\n' >&2; \
	    printf '  make -C k-playbook-local sichern\n' >&2; \
	    exit 1; \
	  }

# Ein Release ist ein Fast-Forward von main auf dev und ein Tag darauf: Bauen,
# Prüfen und Veröffentlichen übernimmt .github/workflows/release.yml.
# Released wird genau der Stand, der auf origin/dev liegt und dort einen
# grünen vollständigen CI-Lauf hat — sauber, auf dev, gepusht —, damit main und
# Tag dasselbe tragen, was CI vollständig geprüft hat. Alle Prüfungen laufen
# vor dem ersten Push, die CI als letzte:
# - Es zählt der neueste vollständige Lauf von ci.yml auf dev für HEAD
#   (run-name "CI full, …"); ein schneller (Push auf dev) zählt nicht.
# - Gibt es keinen, stößt release ihn an (suite=full, mit einer Kennung) und
#   findet ihn über die URL, die gh workflow run zurückgibt, sonst über die
#   Kennung im run-name — nicht einfach den neuesten.
# - Der Lauf zählt nur, wenn sein headSha HEAD ist: --ref dev nimmt die Spitze
#   von dev beim Anstoß, und dev kann inzwischen weiter sein.
# - Läuft er noch, wartet release darauf (gh run watch), statt einen zweiten
#   anzustoßen. Ist er rot, bricht es ab; neu versucht wird nur ausdrücklich
#   (gh run rerun, danach release erneut), release wiederholt nie selbst.
# Nach der CI holt release origin erneut und wiederholt die Prüfungen dort:
# Während des Wartens kann dev, main oder der Tag sich bewegt haben.
# Ein Abbruch nach dem Push von main lässt sich wiederholen: main steht dann
# schon auf HEAD, ein lokal schon angelegter Tag auf HEAD wird nur noch gepusht.
release: ## Nach grüner vollständiger CI für HEAD: main per Fast-Forward auf dev, Tag VERSION pushen; CI baut das Release
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
	  test "$$branch" = dev || { \
	    printf 'Release nur von dev, aktuell: %s\n' "$${branch:-(kein Branch)}" >&2; exit 1; \
	  }; \
	  dirty="$$(git -C "$(REPO)" status --porcelain)"; \
	  test -z "$$dirty" || { \
	    printf 'Der Arbeitsbaum ist nicht sauber; released wird nur ein committeter Stand:\n%s\n' "$$dirty" >&2; \
	    exit 1; \
	  }; \
	  head="$$(git -C "$(REPO)" rev-parse HEAD)"; \
	  check_remote() { \
	    git -C "$(REPO)" fetch --quiet origin dev main; \
	    remote="$$(git -C "$(REPO)" rev-parse origin/dev)"; \
	    if test "$$head" != "$$remote"; then \
	      printf 'dev ist nicht gepusht oder nicht aktuell (HEAD %s, origin/dev %s).\n' "$$head" "$$remote" >&2; \
	      if git -C "$(REPO)" merge-base --is-ancestor "$$head" "$$remote"; then \
	        printf 'origin/dev ist weiter; erst holen: git pull --no-rebase origin dev\n' >&2; \
	      elif git -C "$(REPO)" merge-base --is-ancestor "$$remote" "$$head"; then \
	        printf 'Lokale Commits fehlen auf origin; erst sichern: make -C k-playbook-local sichern\n' >&2; \
	      else \
	        printf 'dev und origin/dev sind auseinandergelaufen; erst holen und sichern:\n' >&2; \
	        printf '  git pull --no-rebase origin dev\n  make -C k-playbook-local sichern\n' >&2; \
	      fi; \
	      exit 1; \
	    fi; \
	    if ! git -C "$(REPO)" merge-base --is-ancestor origin/main HEAD; then \
	      printf 'main hat Commits, die dev fehlen (etwa ein auf GitHub gemergter PR):\n' >&2; \
	      git -C "$(REPO)" log --oneline HEAD..origin/main >&2; \
	      printf '\nErst nach dev holen, sichern und dann erneut release:\n' >&2; \
	      printf '  git merge origin/main\n' >&2; \
	      printf '  make -C k-playbook-local sichern\n' >&2; \
	      printf '  make -C k-playbook-local release VERSION=%s\n' "$$version" >&2; \
	      exit 1; \
	    fi; \
	    rc=0; \
	    git -C "$(REPO)" ls-remote --exit-code --tags origin "refs/tags/$$version" >/dev/null 2>&1 || rc=$$?; \
	    case "$$rc" in \
	      0) printf 'Den Tag %s gibt es auf origin bereits.\n' "$$version" >&2; exit 1 ;; \
	      2) ;; \
	      *) printf 'Die Tags auf origin lassen sich nicht abfragen (git ls-remote, Code %s).\n' "$$rc" >&2; exit 1 ;; \
	    esac; \
	  }; \
	  check_remote; \
	  tagged=; \
	  if git -C "$(REPO)" rev-parse --verify --quiet "refs/tags/$$version" >/dev/null; then \
	    tagcommit="$$(git -C "$(REPO)" rev-parse "refs/tags/$$version^{commit}")"; \
	    test "$$tagcommit" = "$$head" || { \
	      printf 'Den Tag %s gibt es lokal bereits, auf einem anderen Commit (%s, HEAD %s).\n' "$$version" "$$tagcommit" "$$head" >&2; \
	      printf 'Eine andere Version wählen oder den lokalen Tag löschen: git tag -d %s\n' "$$version" >&2; \
	      exit 1; \
	    }; \
	    tagged=1; \
	  fi; \
	  command -v gh >/dev/null 2>&1 || { \
	    printf 'gh fehlt; ohne gh lässt sich der CI-Status von %s nicht prüfen.\n' "$$head" >&2; \
	    printf 'GitHub CLI installieren und anmelden: gh auth login\n' >&2; \
	    exit 1; \
	  }; \
	  id="$$(gh run list -R "$(GH_REPO)" --workflow ci.yml --branch dev --commit "$$head" --limit 100 \
	    --json databaseId,displayTitle,headSha,createdAt \
	    --jq "[.[] | select(.headSha == \"$$head\" and (.displayTitle | startswith(\"CI full,\")))] | sort_by(.createdAt) | last | .databaseId // \"\"")" || { \
	    printf 'Die CI-Läufe von %s lassen sich nicht abfragen (gh run list).\n' "$$head" >&2; exit 1; \
	  }; \
	  if test -z "$$id"; then \
	    kennung="release-$$version-$$(date -u +%Y%m%dT%H%M%SZ)-$$$$"; \
	    printf 'Für %s gibt es auf dev keinen vollständigen CI-Lauf; stoße ihn an (Kennung %s).\n' "$$head" "$$kennung"; \
	    out="$$(gh workflow run ci.yml -R "$(GH_REPO)" --ref dev -f suite=full -f kennung="$$kennung")" || { \
	      printf 'Der vollständige CI-Lauf lässt sich nicht anstoßen (gh workflow run).\n' >&2; exit 1; \
	    }; \
	    url="$$(printf '%s\n' "$$out" | grep -Eo 'https://[^[:space:]]+/actions/runs/[0-9]+' | tail -n 1 || true)"; \
	    if test -n "$$url"; then \
	      id="$${url##*/}"; \
	    else \
	      printf 'gh workflow run nennt keine URL; suche den Lauf über die Kennung.\n'; \
	      tries=0; \
	      while test -z "$$id"; do \
	        tries=$$((tries + 1)); \
	        test "$$tries" -le 20 || { \
	          printf 'Den angestoßenen Lauf mit der Kennung %s finde ich nicht.\n' "$$kennung" >&2; \
	          printf 'Nachsehen: gh run list -R %s --workflow ci.yml --branch dev\n' "$(GH_REPO)" >&2; \
	          printf 'Danach release erneut aufrufen; es findet einen vollständigen Lauf für HEAD selbst.\n' >&2; \
	          exit 1; \
	        }; \
	        sleep 3; \
	        id="$$(gh run list -R "$(GH_REPO)" --workflow ci.yml --branch dev --event workflow_dispatch --limit 50 \
	          --json databaseId,displayTitle \
	          --jq "[.[] | select(.displayTitle | endswith(\", kennung $$kennung\"))] | first | .databaseId // \"\"")" || id=; \
	      done; \
	    fi; \
	  fi; \
	  run="$$(gh run view "$$id" -R "$(GH_REPO)" --json headSha,status,conclusion,url,displayTitle \
	    --jq '"\(.headSha)|\(.status)|\(.conclusion)|\(.url)|\(.displayTitle)"')" || { \
	    printf 'Der CI-Lauf %s lässt sich nicht abfragen (gh run view).\n' "$$id" >&2; exit 1; \
	  }; \
	  sha="$${run%%|*}"; rest="$${run#*|}"; \
	  status="$${rest%%|*}"; rest="$${rest#*|}"; \
	  conclusion="$${rest%%|*}"; rest="$${rest#*|}"; \
	  url="$${rest%%|*}"; title="$${rest#*|}"; \
	  test "$$sha" = "$$head" || { \
	    printf 'Der CI-Lauf %s prüft %s, nicht HEAD %s: dev ist inzwischen weiter.\n' "$$url" "$$sha" "$$head" >&2; \
	    printf 'Nichts gepusht. Erst holen, dann erneut release: git pull --no-rebase origin dev\n' >&2; \
	    exit 1; \
	  }; \
	  case "$$title" in \
	    "CI full,"*) ;; \
	    *) printf 'Der CI-Lauf %s ist kein vollständiger (%s).\n' "$$url" "$$title" >&2; exit 1 ;; \
	  esac; \
	  if test "$$status" != completed; then \
	    printf 'Der vollständige CI-Lauf auf %s läuft (%s); warte darauf: %s\n' "$$head" "$$status" "$$url"; \
	    gh run watch "$$id" -R "$(GH_REPO)" --exit-status --compact --interval 10 || true; \
	    run="$$(gh run view "$$id" -R "$(GH_REPO)" --json status,conclusion --jq '"\(.status)|\(.conclusion)"')" || { \
	      printf 'Der CI-Lauf %s lässt sich nicht abfragen (gh run view).\n' "$$url" >&2; exit 1; \
	    }; \
	    status="$${run%%|*}"; conclusion="$${run#*|}"; \
	    test "$$status" = completed || { \
	      printf 'Das Warten auf den CI-Lauf %s brach ab (%s). Nichts gepusht.\n' "$$url" "$$status" >&2; \
	      printf 'Erneut release aufrufen; es wartet dann weiter auf diesen Lauf.\n' >&2; \
	      exit 1; \
	    }; \
	  fi; \
	  test "$$conclusion" = success || { \
	    printf 'Der vollständige CI-Lauf auf %s ist nicht grün (%s): %s\n' "$$head" "$$conclusion" "$$url" >&2; \
	    printf 'Nichts gepusht. Einen neuen Versuch gibt es nur ausdrücklich:\n' >&2; \
	    printf '  gh run rerun %s -R %s\n' "$$id" "$(GH_REPO)" >&2; \
	    printf '  make -C k-playbook-local release VERSION=%s\n' "$$version" >&2; \
	    exit 1; \
	  }; \
	  printf 'Vollständiger CI-Lauf auf %s grün: %s\n' "$$head" "$$url"; \
	  check_remote; \
	  git -C "$(REPO)" push origin HEAD:refs/heads/main || { \
	    printf 'Der Push nach main ist gescheitert; es ist kein Tag angelegt oder gepusht.\n' >&2; exit 1; \
	  }; \
	  if test -z "$$tagged"; then \
	    git -C "$(REPO)" tag -a "$$version" -m "Release $$version"; \
	  fi; \
	  git -C "$(REPO)" push origin "refs/tags/$$version" || { \
	    printf 'main steht auf %s, aber der Tag %s ist nicht gepusht.\n' "$$head" "$$version" >&2; \
	    printf 'Erneut aufrufen, er wird dann nur gepusht:\n' >&2; \
	    printf '  make -C k-playbook-local release VERSION=%s\n' "$$version" >&2; \
	    exit 1; \
	  }; \
	  if git -C "$(REPO)" show-ref --verify --quiet refs/heads/main; then \
	    git -C "$(REPO)" fetch --quiet origin main:main || \
	      printf 'Hinweis: der lokale Branch main ließ sich nicht nachziehen (git fetch origin main:main).\n' >&2; \
	  fi; \
	  printf '\nmain steht jetzt auf %s.\n' "$$head"; \
	  printf 'Tag %s gepusht. Der Workflow „Release“ baut und veröffentlicht jetzt:\n' "$$version"; \
	  printf '  gh run list --workflow release.yml --limit 1\n'
