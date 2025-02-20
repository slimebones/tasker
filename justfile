set shell := ["nu", "-c"]
set dotenv-load
# HOME := env HOME
dbmate := if os_family() == "windows" { "dbmate.cmd" } else { "dbmate" }

run *args="": compile
	@ ./bin/main {{args}}

compile:
	@ rm -rf bin
	@ mkdir bin
	@ go build -o bin/main

compile_release:
	@ rm -rf bin
	@ mkdir bin
	@ go build -ldflags "-s -w" -o bin/main

test t="":
	@ if "{{t}}" == "" { go test ./... } else { go test ./{{t}} }

create_db:
	dbmate --url $"sqlite:($env.HOME)/appdata/roaming/slimebones/tasker/main.db" create

drop_db:
	dbmate --url $"sqlite:($env.HOME)/appdata/roaming/slimebones/tasker/main.db" drop

recreate_db: drop_db create_db