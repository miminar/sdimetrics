#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

tmpd="$(mktemp -d)"
function cleanup() {
    rm -rf "$tmpd"
}
trap cleanup EXIT

# maps source file path to the result file path
declare -A results=()

function setImageDigestOnDs() {
    local dsPath="$1"
    csplit -f 'split-def' "$dsPath" '%^---$%1' '/^---$/' '{*}'
    local pullSpec=
    local dsFile=
    local digest
    ls -l 
    for f in split-def*; do
        jq . <(yaml2json "$f") > "$f.json"
        if [[ -n "${pullSpec:-}" ]]; then
            continue
        fi
        pullSpec="$(jq -r 'if .kind == "DaemonSet" then
            .spec.template.spec.containers[].image | sub("[@].*"; "")
        else
            ""
        end' "$f.json" | head -n 1)"
        if [[ -n "${pullSpec:-}" ]]; then
            dsFile="$f.json"
        fi
    done

    digest="$(skopeo inspect "docker://$pullSpec" | jq -r .Digest)"
    if [[ -z "${digest:-}" ]]; then
        printf 'Could not determine digest for image "%s"!\n' >&2 "$pullSpec"
        exit 1
    fi

    jq --arg pullSpec "${pullSpec%%:*}@${digest}" \
        '.spec.template.spec.containers |= [.[] | .image |= $pullSpec]' \
        "$dsFile" | sponge "$dsFile"

    local dst
    dst="result-$(basename "$dsPath").yml"
    for f in *.json; do
        printf -- '---\n'   >>"$dst"
        json2yaml "$f"      >>"$dst"
    done
    results["$dsPath"]="$dst"
    rm ./split-def* ||:
}

pushd "$tmpd"
    for fn in "$root/k8s"/ds-*.yaml "$root/systemd-reloader"/ds-*.yaml; do
        setImageDigestOnDs "$fn"
    done
popd

set -x
git stash
    git checkout digest
        git fetch
        git rebase origin/master
        for src in "${!results[@]}"; do
            result="${results["$src"]}"
            cp "$tmpd/$result" "$src"
        done
        git commit -vasm 'updated digests for the daemonset images'
        git push -f
    git checkout -
git stash pop
